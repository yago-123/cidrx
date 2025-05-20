package cidrx //nolint:testpackage // it's OK to be just cidrx

import (
	"errors"
	"math"
	"net"
	"testing"
)

// TestBlockAllocRelease ensures that a block can allocate and release IPs correctly
func TestBlockAllocRelease(t *testing.T) {
	prefix := net.IPNet{IP: net.ParseIP("2001:db8:0:1::"), Mask: net.CIDRMask(120, 128)}
	b := newBlock(prefix, 4)
	// Allocate all 4
	ips := make([]net.IP, 4)
	for i := 0; i < 4; i++ {
		idx, err := b.allocBit()
		if err != nil {
			t.Fatalf("allocBit #%d error: %v", i, err)
		}
		ips[i] = b.bitToIP(idx)
	}
	// Next alloc should ErrBlockFull
	if _, err := b.allocBit(); !errors.Is(err, ErrBlockFull) {
		t.Errorf("Expected ErrBlockFull, got %v", err)
	}
	// Release and reallocate
	for i, ip := range ips {
		delta, err := b.ipToBit(ip)
		if err != nil || delta != uint64(i) {
			t.Errorf("ipToBit: got %d, want %d", delta, i)
		}
		if errRelease := b.releaseBit(delta); errRelease != nil {
			t.Errorf("releaseBit #%d error: %v", i, errRelease)
		}
		// Realloc should succeed
		if _, errAlloc := b.allocBit(); errAlloc != nil {
			t.Errorf("re-alloc #%d error: %v", i, errAlloc)
		}
	}
}

// TestFreeCount ensures freeCount is tracked correctly
func TestFreeCount(t *testing.T) {
	prefix := net.IPNet{IP: net.ParseIP("2001:db8::"), Mask: net.CIDRMask(124, 128)}
	size := uint64(16)
	b := newBlock(prefix, size)

	if b.freeCount != size {
		t.Fatalf("initial freeCount = %d; want %d", b.freeCount, size)
	}

	// Allocate half
	for i := uint64(0); i < size/2; i++ {
		_, err := b.allocBit()
		if err != nil {
			t.Fatalf("allocBit #%d: %v", i, err)
		}
	}
	if b.freeCount != size/2 {
		t.Errorf("after half allocs freeCount = %d; want %d", b.freeCount, size/2)
	}

	// Release one
	if err := b.releaseBit(3); err != nil {
		t.Fatalf("releaseBit: %v", err)
	}
	if b.freeCount != size/2+1 {
		t.Errorf("after release freeCount = %d; want %d", b.freeCount, size/2+1)
	}
}

// TestAllocAcrossWordBoundary ensures allocBit scans across multiple uint64 words
func TestAllocAcrossWordBoundary(t *testing.T) {
	prefix := net.IPNet{IP: net.ParseIP("2001:db8:1::"), Mask: net.CIDRMask(112, 128)}
	size := uint64(80) // spans two 64-bit words (64+16)
	b := newBlock(prefix, size)

	// Allocate first 64 bits
	for i := uint64(0); i < 64; i++ {
		idx, err := b.allocBit()
		if err != nil {
			t.Fatalf("allocBit #%d: %v", i, err)
		}
		if idx != i {
			t.Errorf("allocBit #%d returned idx %d", i, idx)
		}
	}

	// Next 16 should be in word 1
	for i := uint64(64); i < size; i++ {
		idx, err := b.allocBit()
		if err != nil {
			t.Fatalf("allocBit #%d: %v", i, err)
		}
		if idx != i {
			t.Errorf("allocBit #%d returned idx %d", i, idx)
		}
	}

	// Then full
	if _, err := b.allocBit(); !errors.Is(err, ErrBlockFull) {
		t.Errorf("expected ErrBlockFull, got %v", err)
	}
}

// TestReleaseBitInvalid ensures invalid indexes are rejected
func TestReleaseBitInvalid(t *testing.T) {
	prefix := net.IPNet{IP: net.ParseIP("2001:db8::"), Mask: net.CIDRMask(120, 128)}
	b := newBlock(prefix, 4)

	if err := b.releaseBit(5); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("releaseBit(5) err = %v; want ErrOutOfRange", err)
	}

	if err := b.releaseBit(math.MaxUint64); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("releaseBit(MaxUint64) err = %v; want ErrOutOfRange", err)
	}
}

// TestBitToIPBoundary tests bitToIP at idx=0 and idx=size-1
func TestBitToIPBoundary(t *testing.T) {
	prefix := net.IPNet{IP: net.ParseIP("2001:db8::"), Mask: net.CIDRMask(125, 128)}
	size := uint64(8)
	b := newBlock(prefix, size)

	ip0 := b.bitToIP(0)
	want0 := prefix.IP
	if !ip0.Equal(want0) {
		t.Errorf("bitToIP(0) = %v; want %v", ip0, want0)
	}

	ipLast := b.bitToIP(size - 1)

	// Manually compute expected last
	exp := make(net.IP, len(want0))
	copy(exp, want0)
	exp[15] += byte(size - 1)
	if !ipLast.Equal(exp) {
		t.Errorf("bitToIP(%d) = %v; want %v", size-1, ipLast, exp)
	}
}

// TestRoundTripLargeBlock performs a small sampling of random indices in a bigger block
func TestRoundTripLargeBlock(t *testing.T) {
	prefix := net.IPNet{IP: net.ParseIP("2001:db8:ffff::"), Mask: net.CIDRMask(120, 128)}
	size := uint64(256)
	b := newBlock(prefix, size)

	// Test a few key indices
	for _, idx := range []uint64{0, 1, 63, 64, 127, 128, 255} {
		ip := b.bitToIP(idx)
		got, err := b.ipToBit(ip)
		if err != nil {
			t.Errorf("ipToBit(%v) error: %v", ip, err)
		}
		if got != idx {
			t.Errorf("ipToBit(%v) = %d; want %d", ip, got, idx)
		}
	}
}

func TestBlockIpToBitOutOfRange(t *testing.T) {
	prefix := net.IPNet{IP: net.ParseIP("2001:db8::"), Mask: net.CIDRMask(120, 128)}
	b := newBlock(prefix, 2)
	// IP outside block
	external := net.ParseIP("2001:db8::100")
	if _, err := b.ipToBit(external); err == nil {
		t.Error("Expected ErrOutOfRange, got nil")
	}
}
