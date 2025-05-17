package cidrx

import (
	"fmt"
	"net"
	"sync"
)

const (
	ipv6BitLen = net.IPv6len * 8
)

// Pool manages IPv6 allocations using Uint128 arithmetic and a pre-reserved free list.
type Pool struct {
	// extracts a block’s network base by masking off host-offset bits
	blockMask net.IPMask
	// CIDR base address Uint128
	networkAddr Uint128
	// number of bits for host offsets per block.
	hostBits uint
	// number of addresses per block = 1<<hostBits.
	blockSize uint64

	// map blockIndex → bitmap block instance.
	blocks map[uint64]*block // maps blockIndex → bitmap block instance.
	// list of blocks initialized with free space
	freeList []uint64
	// next block index to allocate
	nextBlockIndex uint64
	// maxBlocks represent the maximum number of blocks that can be allocated. This creates a hard limit of 2^63 blocks
	// being allowed
	maxBlocks uint64
	// protects freeList and blocks
	mu sync.Mutex
}

// NewPool constructs a Pool that transforms the given IPv6 network into fixed-size blocks (bitmaps).
//
//	networkAddress   – the base IPv6 network (e.g. "2001:db8::")
//	netPrefixLen     – the prefix length of the network (0 ≤ netPrefixLen ≤ 128)
//	blockPrefix      – the prefix length of each allocation block; must satisfy
//	                   netPrefixLen < blockPrefix ≤ 128 and (blockPrefix – netPrefixLen) ≤ 63
//	expectedBlocks   – an estimate of how many blocks you’ll use, to pre-reserve
//	                   freeList capacity (avoids slice reallocations on Allocate)
//
// Returns a *Pool ready to Allocate() and Release() IPs, or an error if any arguments
// are invalid (bad IP, out-of-range prefixes, or too many blocks)
func NewPool(netAddress string, netPrefixLen, blockPrefix, expectedBlocks int) (*Pool, error) {
	ip := net.ParseIP(netAddress)
	if ip == nil || ip.To16() == nil || ip.To4() != nil {
		return nil, fmt.Errorf("invalid IPv6 address %q", netAddress)
	}

	if netPrefixLen < 0 || netPrefixLen > (ipv6BitLen) {
		return nil, fmt.Errorf("baseCIDR prefix must be between 0 and 128")
	}

	// Create the base CIDR
	mask := net.CIDRMask(netPrefixLen, ipv6BitLen)
	netIP := ip.Mask(mask)

	// Ensure block prefix length is larger than network prefix
	if blockPrefix <= netPrefixLen || blockPrefix > ipv6BitLen {
		return nil, fmt.Errorf("block prefix must be > /%d and ≤128", netPrefixLen)
	}

	// Make sure number of blocks will be able to fitted in a uint64 (hard limit of 2^63 blocks)
	diff := blockPrefix - netPrefixLen
	if diff > 63 {
		return nil, fmt.Errorf("too many blocks (diff %d >63)", diff)
	}

	maxBlocks := uint64(1) << uint(diff) // number of blocks available for this prefix
	hBits := uint(128 - blockPrefix)     // how many bits for host offsets
	bSize := uint64(1) << hBits          // how many ips per block

	pool := &Pool{
		blockMask:   mask,
		networkAddr: fromIP(netIP),
		hostBits:    hBits,
		blockSize:   bSize,
		blocks:      make(map[uint64]*block),
		freeList:    make([]uint64, 0, expectedBlocks),
		maxBlocks:   maxBlocks,
	}
	return pool, nil
}

// Allocate returns a free IPv6 from the pool.
func (p *Pool) Allocate() (net.IP, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if we have any free blocks with ready-to-use IPs
	for len(p.freeList) > 0 {
		// Pop the last block index from the freeList
		bi := p.freeList[len(p.freeList)-1]
		p.freeList = p.freeList[:len(p.freeList)-1]

		// Try to allocate an IP (as a bit) from the block
		blk := p.blocks[bi]
		idx, err := blk.allocBit()
		if err == nil {
			// If allocation was successful, check if the block still has free space and push it back to the freeList
			if blk.freeCount > 0 {
				p.freeList = append(p.freeList, bi)
			}
			return blk.bitToIP(idx), nil
		}
	}

	// Otherwise and if remains within limits (no IP exhaustion yet) allocate a new block
	if p.nextBlockIndex >= p.maxBlocks {
		return nil, fmt.Errorf("pool exhausted")
	}

	blkIncoming := p.nextBlockIndex
	// Compute first base IP of the new block
	startBI := p.networkAddr.lsh(0).add(Uint128{Lo: blkIncoming}.lsh(p.hostBits))
	startIP := startBI.toIP()

	// Create the block prefix
	prefix := net.IPNet{IP: startIP, Mask: p.blockMask}

	// Allocate the first IP in the new block
	blk := newBlock(prefix, p.blockSize)
	idx, _ := blk.allocBit()
	ip := blk.bitToIP(idx)

	// Add the new block to the free blocks pool
	p.blocks[blkIncoming] = blk
	if blk.freeCount > 0 { // rare case, but possible
		p.freeList = append(p.freeList, blkIncoming)
	}
	p.nextBlockIndex++
	return ip, nil
}

// Release frees an IPv6 back to the pool.
func (p *Pool) Release(ip net.IP) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Compute block base index: (ipBI - base) >> hostBits
	ipBI := fromIP(ip)
	delta := ipBI.sub(p.networkAddr)
	bi := delta.rsh(p.hostBits).Lo

	blk, ok := p.blocks[bi]
	if !ok {
		return fmt.Errorf("IP %s not from pool", ip)
	}

	// Retrieve the index of the IP and release it
	idx, err := blk.ipToBit(ip)
	if err != nil {
		return err
	}
	if errRelease := blk.releaseBit(idx); errRelease != nil {
		return errRelease
	}

	// If block has any free space, ensure it's in freeList
	if blk.freeCount > 0 {
		p.freeList = append(p.freeList, bi)
	}
	return nil
}
