package cidrx //nolint:testpackage // it's OK to be just cidrx

import (
	"net"
	"testing"
)

// TestSnapshotRoundTrip ensures that a pool can be created, modified, and then restored from a snapshot
func TestSnapshotRoundTrip(t *testing.T) {
	// Create a pool and allocate some IPs
	pool, _ := NewPool("2001:db8::", 64, 120, 10)
	ips := make([]net.IP, 5)
	for i := range ips {
		ip, err := pool.Allocate()
		if err != nil {
			t.Fatalf("Allocate error: %v", err)
		}
		ips[i] = ip
	}

	// Release one to mix state
	if err := pool.Release(ips[2]); err != nil {
		t.Fatalf("Release error: %v", err)
	}

	// Snapshot
	snap := pool.Snapshot()

	// Make further modifications
	_, _ = pool.Allocate()

	// Restore fresh pool from snapshot
	restored, err := NewPoolFromSnapshot(snap)
	if err != nil {
		t.Fatalf("Restore error: %v", err)
	}

	// Allocate again, should match sequence from snapshot state
	for i := range ips {
		ip, errAllocate := restored.Allocate()
		if errAllocate != nil {
			t.Fatalf("Restored Allocate error: %v", errAllocate)
		}
		// First should be the one we released
		if i == 0 && !ip.Equal(ips[2]) {
			t.Errorf("Expected first restored IP %v, got %v", ips[2], ip)
		}
	}
}

// TestSnapshotExhaustion ensures that if the original pool is exhausted, the restored pool from Snapshot is also exhausted
func TestSnapshotExhaustion(t *testing.T) {
	// /127 -> 2 addresses, blockPrefix=128 → 1 address per block → 2 blocks total
	pool, _ := NewPool("2001:db8::", 127, 128, 2)

	// Allocate both addresses
	if _, err := pool.Allocate(); err != nil {
		t.Fatalf("first allocate error: %v", err)
	}
	if _, err := pool.Allocate(); err != nil {
		t.Fatalf("second allocate error: %v", err)
	}
	// Now pool should be exhausted
	if _, err := pool.Allocate(); err == nil {
		t.Fatal("expected exhaustion on original pool, got nil")
	}

	// Taake a snapshot of the exhausted state
	snap := pool.Snapshot()

	// Restore into a new pool
	restored, err := NewPoolFromSnapshot(snap)
	if err != nil {
		t.Fatalf("restore error: %v", err)
	}

	// The restored pool should also be exhausted
	if _, errAllocate := restored.Allocate(); errAllocate == nil {
		t.Error("expected exhaustion on restored pool, got nil")
	}
}
