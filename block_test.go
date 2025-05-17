package cidrx //nolint:testpackage // it's OK to be just cidrx

import (
	"errors"
	"net"
	"testing"
)

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

func TestBlockIpToBitOutOfRange(t *testing.T) {
	prefix := net.IPNet{IP: net.ParseIP("2001:db8::"), Mask: net.CIDRMask(120, 128)}
	b := newBlock(prefix, 2)
	// IP outside block
	external := net.ParseIP("2001:db8::100")
	if _, err := b.ipToBit(external); err == nil {
		t.Error("Expected ErrOutOfRange, got nil")
	}
}
