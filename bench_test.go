package cidrx //nolint:testpackage // it's OK to be just cidrx

import (
	"net"
	"sync"
	"testing"
)

func BenchmarkAllocateSequential(b *testing.B) {
	b.ReportAllocs()
	pool, _ := NewPool("2001:db8::", 64, 120, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pool.Allocate()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReleaseSequential(b *testing.B) {
	b.ReportAllocs()
	pool, _ := NewPool("2001:db8::", 64, 120, 1024)
	ips := make([]net.IP, b.N)
	for i := 0; i < b.N; i++ {
		ip, err := pool.Allocate()
		if err != nil {
			b.Fatal(err)
		}
		ips[i] = ip
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := pool.Release(ips[i]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAllocateReleaseMixed(b *testing.B) {
	b.ReportAllocs()
	pool, _ := NewPool("2001:db8::", 64, 120, 1024)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ip, err := pool.Allocate()
			if err != nil {
				b.Fatal(err)
			}
			if errRelease := pool.Release(ip); errRelease != nil {
				b.Fatal(errRelease)
			}
		}
	})
}

func BenchmarkConcurrentAlloc(b *testing.B) {
	b.ReportAllocs()
	pool, _ := NewPool("2001:db8::", 64, 120, 1024)
	var wg sync.WaitGroup
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := pool.Allocate()
			if err != nil {
				b.Fatal(err)
			}
		}()
	}
	wg.Wait()
}

func BenchmarkBlockIndexForIP(b *testing.B) {
	b.ReportAllocs()
	pool, _ := NewPool("2001:db8::", 64, 120, 1024)
	ip, _ := pool.Allocate()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// inline block index compute via Uint128
		ipBI := fromIP(ip)
		delta := ipBI.sub(pool.networkAddr)
		_ = delta.rsh(pool.hostBits).Lo
	}
}

// BenchmarkNewPool measures the cost of creating a new Pool.
func BenchmarkNewPool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := NewPool("2001:db8::", 64, 120, 1024); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAllocateBlockCreation forces block creation on every Allocate.
func BenchmarkAllocateBlockCreation(b *testing.B) {
	b.ReportAllocs()
	pool, _ := NewPool("2001:db8::", 64, 120, 1024)
	// clear freeList so each Allocate creates a new block
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.freeList = pool.freeList[:0]
		pool.nextBlockIndex = uint64(i % 1024)
		if _, err := pool.Allocate(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAllocateBlockSize120(b *testing.B) {
	b.ReportAllocs()
	pool, _ := NewPool("2001:db8::", 64, 120, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := pool.Allocate(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAllocateBlockSize112(b *testing.B) {
	b.ReportAllocs()
	pool, _ := NewPool("2001:db8::", 64, 112, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := pool.Allocate(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAllocVariousCIDRs(b *testing.B) {
	cases := []struct {
		name           string
		netPrefix      int
		blockPrefix    int
		expectedBlocks int
	}{
		{"Net64_Block120", 64, 120, 1024},
		{"Net56_Block120", 56, 120, 1024},
		{"Net48_Block120", 48, 120, 1024},
		{"Net64_Block112", 64, 112, 1024},
		{"Net64_Block100", 64, 100, 1024},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			pool, err := NewPool("2001:db8::", c.netPrefix, c.blockPrefix, c.expectedBlocks)
			if err != nil {
				b.Skipf("Skipping invalid config %s: %v", c.name, err)
				return
			}
			b.ReportAllocs()
			// Prime for hot-path
			ip, _ := pool.Allocate()
			_ = pool.Release(ip)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := pool.Allocate(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkReleaseVariousCIDRs(b *testing.B) { //nolint:gocognit // test funcs are long
	cases := []struct {
		name           string
		netPrefix      int
		blockPrefix    int
		expectedBlocks int
		nAlloc         int
	}{
		{"Net64_Block120", 64, 120, 1024, 1024},
		{"Net56_Block120", 56, 120, 1024, 1024},
		{"Net48_Block120", 48, 120, 1024, 1024},
		{"Net64_Block112", 64, 112, 1024, 1024},
		{"Net64_Block100", 64, 100, 1024, 1024},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			pool, err := NewPool("2001:db8::", c.netPrefix, c.blockPrefix, c.expectedBlocks)
			if err != nil {
				b.Skipf("Skipping invalid config %s: %v", c.name, err)
				return
			}
			b.ReportAllocs()

			// Prepare a batch of allocations
			ips := make([]net.IP, c.nAlloc)
			for i := 0; i < c.nAlloc; i++ {
				ip, errAlloc := pool.Allocate()
				if errAlloc != nil {
					b.Fatal(errAlloc)
				}
				ips[i] = ip
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Release a subset of 10
				for j := 0; j < 10; j++ {
					if errRelease := pool.Release(ips[j]); errRelease != nil {
						b.Fatal(errRelease)
					}
				}

				b.StopTimer()

				// Re-allocate them under StopTimer so the next iteration can release them again
				for j := 0; j < 10; j++ {
					ips[j], err = pool.Allocate()
					if err != nil {
						b.Fatal(err)
					}
				}
				b.StartTimer()
			}
		})
	}
}
