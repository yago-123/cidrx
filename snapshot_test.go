package cidrx //nolint:testpackage // it's OK to be just cidrx

import (
	"net"
	"testing"
)

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
