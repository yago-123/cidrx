package cidrx

import (
	"fmt"
	"math/bits"
	"net"
)

// Snapshot captures the current state of a Pool for export/import (no serialization).
// The Blocks map holds raw bitmap words for each active block.
type Snapshot struct {
	BlockMask      net.IPMask // the mask for each block
	NetworkAddr    Uint128    // base network address
	HostBits       uint       // host bits per block
	BlockSize      uint64     // addresses per block
	NextBlockIndex uint64     // next unused block index
	MaxBlocks      uint64     // total possible blocks

	FreeList []uint64            // block indices with free addresses
	Blocks   map[uint64][]uint64 // blockIndex -> bitmap words
}

// NewPoolFromSnapshot constructs a Pool from a previously taken Snapshot. It discards any existing state and recreates
// blocks, freeList, and indexes.
func NewPoolFromSnapshot(s *Snapshot) (*Pool, error) {
	// Validate snapshot consistency
	if s.BlockMask == nil || s.BlockSize == 0 {
		return nil, fmt.Errorf("invalid snapshot: incomplete configuration")
	}

	// Initialize pool structure
	p := &Pool{
		blockMask:      append(net.IPMask{}, s.BlockMask...),
		networkAddr:    s.NetworkAddr,
		hostBits:       s.HostBits,
		blockSize:      s.BlockSize,
		blocks:         make(map[uint64]*block, len(s.Blocks)),
		freeList:       make([]uint64, len(s.FreeList)),
		nextBlockIndex: s.NextBlockIndex,
		maxBlocks:      s.MaxBlocks,
	}
	copy(p.freeList, s.FreeList)

	// Rebuild each block
	for idx, words := range s.Blocks {
		// Compute block prefix IP
		startBI := p.networkAddr.add(Uint128{Lo: idx}.lsh(p.hostBits))
		prefix := net.IPNet{IP: startBI.toIP(), Mask: p.blockMask}

		// Recreate block
		blk := newBlock(prefix, p.blockSize)
		copy(blk.used, words)

		// Recalc freeCount
		var usedCount uint64
		for _, w := range words {
			usedCount += uint64(bits.OnesCount64(w))
		}
		blk.freeCount = p.blockSize - usedCount
		p.blocks[idx] = blk
	}

	return p, nil
}

// Snapshot creates a deep copy of the Pool's current state.
func (p *Pool) Snapshot() *Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Copy freeList
	fl := make([]uint64, len(p.freeList))
	copy(fl, p.freeList)

	// Copy each block's bitmap words
	bm := make(map[uint64][]uint64, len(p.blocks))
	for idx, blk := range p.blocks {
		words := make([]uint64, len(blk.used))
		copy(words, blk.used)
		bm[idx] = words
	}

	return &Snapshot{
		BlockMask:      append(net.IPMask{}, p.blockMask...),
		NetworkAddr:    p.networkAddr,
		HostBits:       p.hostBits,
		BlockSize:      p.blockSize,
		NextBlockIndex: p.nextBlockIndex,
		MaxBlocks:      p.maxBlocks,
		FreeList:       fl,
		Blocks:         bm,
	}
}
