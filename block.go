package cidrx

import (
	"encoding/binary"
	"math/bits"
	"net"
)

// block represents a fixed-size bitmap for one IPv6 CIDR segment.
type block struct {
	prefix    net.IPNet
	used      []uint64 // bitmap words: 1 means allocated
	freeCount uint64   // how many bits are still free
	size      uint64   // total bits
}

// newBlock creates a block for the given prefix and size.
func newBlock(prefix net.IPNet, size uint64) *block {
	words := int((size + 63) / 64)
	return &block{
		prefix:    prefix,
		used:      make([]uint64, words),
		freeCount: size,
		size:      size,
	}
}

// allocBit finds and sets the first zero bit, returning its index
func (b *block) allocBit() (uint64, error) {
	// Iterate over the bitmap words to find a free bit
	for wi, word := range b.used {
		// Check if the word is not full by checking if the bitwise NOT is not zero
		if ^word != 0 {
			// Find the first zero bit in the word
			bit := bits.TrailingZeros64(^word)
			// Retrieve the index of the zero bit
			idx := uint64(wi)*64 + uint64(bit)
			if idx >= b.size {
				continue
			}

			// Set the bit to 1 (allocated)
			b.used[wi] |= 1 << bit
			b.freeCount--

			return idx, nil
		}
	}
	return 0, ErrBlockFull
}

// releaseBit clears the bit at idx
func (b *block) releaseBit(idx uint64) error {
	if idx >= b.size {
		return ErrOutOfRange
	}

	// Calculate the word index and bit position
	wi := idx / 64
	bit := idx % 64

	// Check if the bit is already free
	if b.used[wi]&(1<<bit) == 0 {
		return ErrNotAllocated
	}

	// Clear the bit (release it)
	b.used[wi] &^= 1 << bit
	b.freeCount++
	return nil
}

// bitToIP converts a bit index into an IPv6 address within this block
func (b *block) bitToIP(idx uint64) net.IP {
	// Split block base address into high and low parts
	h := binary.BigEndian.Uint64(b.prefix.IP[:8])
	l := binary.BigEndian.Uint64(b.prefix.IP[8:])

	// Add offset to low part, propagating carry to high part
	newLow, carry := bits.Add64(l, idx, 0)
	newHigh, _ := bits.Add64(h, 0, carry)

	// Construct the new IP address
	ip := make(net.IP, net.IPv6len)
	binary.BigEndian.PutUint64(ip[:8], newHigh)
	binary.BigEndian.PutUint64(ip[8:], newLow)

	return ip
}

// ipToBit returns the bit index for the given IP in this block
func (b *block) ipToBit(ip net.IP) (uint64, error) {
	// Load block base address
	h0 := binary.BigEndian.Uint64(b.prefix.IP[:8])
	l0 := binary.BigEndian.Uint64(b.prefix.IP[8:])

	// Load target IP
	h1 := binary.BigEndian.Uint64(ip[:8])
	l1 := binary.BigEndian.Uint64(ip[8:])

	// Check if the IP is within the block range
	dL, borrow := bits.Sub64(l1, l0, 0)
	dH, under := bits.Sub64(h1, h0, borrow)

	// Bound-check against block size
	if under != 0 || dH != 0 {
		return 0, ErrOutOfRange
	}
	if dL >= b.size {
		return 0, ErrOutOfRange
	}

	return dL, nil
}
