# cidrx

`cidrx` is a simple Go library for efficient IPv6 address pool management. It uses fixed-size bitmap blocks and 128-bit
integer arithmetic to allocate and release IPs in `O(1)` time with minimal heap allocations.

## Features
* **Large-scale pools**: Supports up to 2⁶³ blocks per pool, each block covering `2^(128-blockPrefix)` addresses
* **Lazy block creation**: Blocks are allocated on-demand, minimizing memory usage
* **Bitmap-backed**: Each block uses a `uint64` bitmap for ultra-fast allocation and release
* **Low allocations**: Pre-reserved free-list and bitwise arithmetic mean zero or minimal heap allocations on the hot path
* **Snapshot/Restore**: Export pool state and recreate it later via `Snapshot` and `NewPoolFromSnapshot`
* **Concurrency-safe**: Thread-safe via a simple `sync.Mutex`; optional sharding strategies can further improve throughput

## Installation
```bash
go get github.com/yago-123/cidrx
```

## Usage
```go
package main

import (
    "fmt"
    "net"
    "log"
	
    "github.com/yago-123/cidrx"
)

func main() {
    // Create a pool for 2001:db8::/64 split into /120 blocks
    pool, err := cidrx.NewPool("2001:db8::", 64, 120, 1024)
    if err != nil {
        log.Fatal(err)
    }

    // Allocate 10 IPs
    for i := 0; i < 10; i++ {
        ip, errAlloc := pool.Allocate()
        if errAlloc != nil {
            log.Printf("Allocate error: %v", errAlloc)
            continue
        }
        fmt.Println("Got IP:", ip)
    }

    // Release an IP
    err = pool.Release(net.ParseIP("2001:db8::3"))
    if err != nil {
        log.Println("Release error:", err)
    }
}
```

## API
### `NewPool(netAddress string, netPrefixLen, blockPrefix, expectedBlocks int) (*Pool, error)`
Constructs a new IPv6 pool.

* `netAddress`: base IPv6 (e.g. `"2001:db8::"`).
* `netPrefixLen`: prefix length of the network (0–128).
* `blockPrefix`: prefix length of each block; must satisfy `netPrefix < blockPrefix ≤ 128` and `blockPrefix−netPrefix ≤ 63`.
* `expectedBlocks`: estimate for number of blocks to pre-allocate free-list capacity.

### `(*Pool) Allocate() (net.IP, error)`
Allocates and returns the next available IP in the pool.

### `(*Pool) Release(ip net.IP) error`
Releases a previously allocated IP back to the pool.

### `(*Pool) Snapshot() *Snapshot`
Returns an in-memory snapshot of the pool state (configuration + bitmaps).

### `NewPoolFromSnapshot(s *Snapshot) (*Pool, error)`
Rebuilds a `Pool` from a prior snapshot.

## Testing & Benchmarking
Run the test suite:
```bash
go test ./... -v
```

Run benchmarks:
```bash
go test ./... -bench=. -benchmem
```

## Benchmarks
All tests run on Linux amd64 (AMD Ryzen 5 5600H). Timings are per-op:

| Benchmark                                             | ns/op | B/op  | allocs/op | Notes                                               |
|-------------------------------------------------------|-------|-------|-----------|-----------------------------------------------------|
| **AllocateSequential**                                | 40,7  | 16    | 1         | Hot-path allocate: bitmap math + one 16-byte slice alloc |
| **ReleaseSequential**                                 | 48,6  | 43    | 0         | Zero-alloc release: bit clear + slice append under capacity |
| **AllocateReleaseMixed**                              | 112,7 | 64    | 1         | Allocate (40 ns) + Release (48 ns) + minimal coordination |
| **ConcurrentAlloc** (need improvement!)               | 536,7 | 103   | 2         | Global mutex contention adds ~500 ns under heavy concurrency |
| **BlockIndexForIP**                                   | 2,3   | 0     | 0         | Pure Uint128 shift/subtract—blazingly fast          |
| **NewPool**                                           | 1.998 | 8.384 | 5         | One-time cost: parse CIDR, init map & free-list     |
| **AllocateBlockCreation** (`/120` first-hit)          | 121,7 | 160   | 4         | Zeroing a new `/120` bitmap (32 B) + map insertion  |
| **AllocateBlockSize120** (256-addr reuse)             | 41,6  | 16    | 1         | Reuse hot path for `/120` blocks—same perf as sequential alloc |
| **AllocateBlockSize112** (65.536-addr first-hit)      | 442,0 | 16    | 1         | First-time creation of a 1 024-word bitmap (~8 KB). |
| **AllocateBlockSize100** (268.435.456-addr first-hit) | 5.934 | 16    | 1         | First-time creation of a 4 194 304-word bitmap (~32 MB) |
| **AllocVariousCIDRs** (`/64`→`/120` hot path)         | 41,6  | 16    | 1         | Primed free-list shows identical per-op cost across CIDRs |
| **AllocVariousCIDRs** (`/112` first-hit)              | 429,6 | 16    | 1         | First block creation overhead for `/112`            |
| **AllocVariousCIDRs** (`/100` first-hit)              | 5.934 | 16    | 1         | First block creation overhead for `/100`            |
| **ReleaseVariousCIDRs** (10 releases/iter)            | 415,3 | 407   | 0         | ~10 × ReleaseSequential (10 × 48 ns = 480 ns) minus timer costs |

### Summary
- **Hot-path allocate**: ~40 ns/op with 1 alloc (16 B) (most likely coming from `bitToIP` alloc)
- **Release**: ~48 ns/op with 0 allocs
- **Mixed allocate+release**: ~112 ns/op
- **Concurrent allocate**: ~500 ns/op under a single mutex
- **Block creation**: cost scales with bitmap size (32 B → 32 MB)
- **Block-index math**: ~2 ns/op with 0 allocs

### Next-step optimizations
1. Shard the pool or use per-block locks to reduce mutex blocks 
2. Pool or reuse large bitmaps to avoid zeroing overhead on first use
  
