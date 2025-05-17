// Package cidrx provides a simple IPv6 address pool allocator.
// It partitions a base IPv6 network (for example 2001:db8::/64) into fixed-size bitmap blocks
// (for example /120) and offers O(1) Allocate and Release methods with minimal heap allocations.
// The pool supports in-memory snapshot and restore operations.
//
// Example:
//
//	import "github.com/yago-123/cidrx"
//
//	// Create a pool for 2001:db8::/64 split into /120 blocks
//	pool, err := cidrx.NewPool("2001:db8::", 64, 120, 1024)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Allocate an IP
//	ip, err := pool.Allocate()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Take a snapshot of the pool state
//	snap := pool.Snapshot()
//
//	// Release the IP
//	if err := pool.Release(ip); err != nil {
//	    log.Fatal(err)
//	}
//
// // Restore the pool from snapshot
// restoredPool, err := cidrx.NewPoolFromSnapshot(snap)
package cidrx
