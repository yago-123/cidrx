package cidrx

import "errors"

var (
	// ErrBlockFull indicates no free addresses remain in the block
	ErrBlockFull = errors.New("block is full")
	// ErrOutOfRange indicates the index or IP is outside the block range
	ErrOutOfRange = errors.New("IP offset out of range")
	// ErrNotAllocated indicates an attempt to release an IP that wasn't allocated
	ErrNotAllocated = errors.New("IP not allocated")
)
