package cidrx //nolint:testpackage // it's OK to be just cidrx

import (
	"net"
	"sync"
	"testing"
)

// TestNewPoolTooManyBlocks ensures NewPool rejects diffs > 63
func TestNewPoolTooManyBlocks(t *testing.T) {
	// diff = blockPrefix - netPrefixLen = 64 -> too many
	_, err := NewPool("2001:db8::", 0, 64, 1)
	if err == nil {
		t.Fatal("expected error for diff >63, got nil")
	}
}

// TestAllocateRelease ensures that an allocated IP can be released and reallocated
func TestAllocateRelease(t *testing.T) {
	pool, err := NewPool("2001:db8:0:1::", 64, 120, 16)
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	ip1, err := pool.Allocate()
	if err != nil {
		t.Fatalf("Allocate error: %v", err)
	}
	if errRelease := pool.Release(ip1); errRelease != nil {
		t.Fatalf("Release error: %v", errRelease)
	}
	ip2, err := pool.Allocate()
	if err != nil {
		t.Fatalf("Allocate second error: %v", err)
	}
	if !ip1.Equal(ip2) {
		t.Errorf("Expected reallocated IP %v, got %v", ip1, ip2)
	}
}

// TestExhaustion verifies that exhaustion returns an error
func TestExhaustion(t *testing.T) {
	// /127 baseCIDR => 2 addresses, blockPrefix 128 => 1 address per block, 2 blocks
	pool, err := NewPool("::", 127, 128, 2)
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}
	// Should be able to allocate twice, then fail on third
	for i := 0; i < 2; i++ {
		if _, errAllocate := pool.Allocate(); errAllocate != nil {
			t.Fatalf("Allocate %d error: %v", i, errAllocate)
		}
	}
	if _, errAllocate := pool.Allocate(); errAllocate == nil {
		t.Error("Expected exhaustion error, got nil")
	}
}

// TestReleaseAfterFullBlock measures that after exhausting a single small block, a Release correctly frees a
// slot for reuse
func TestReleaseAfterFullBlock(t *testing.T) {
	// /126 block → 4 addresses per block
	pool, _ := NewPool("2001:db8::", 64, 126, 1)

	// Allocate exactly 4 IPs to fill the first block
	ips := make([]net.IP, 4)
	for i := 0; i < 4; i++ {
		ip, err := pool.Allocate()
		if err != nil {
			t.Fatalf("Allocate #%d error: %v", i, err)
		}
		ips[i] = ip
	}

	// Next allocation should succeed by creating a new block
	if ipNew, err := pool.Allocate(); err != nil {
		t.Fatalf("Expected new block allocation, got error: %v", err)
	} else if ipNew.Equal(ips[0]) {
		t.Errorf("Expected a different-block IP, got %v", ipNew)
	}

	// Now release one from the first block
	if err := pool.Release(ips[2]); err != nil {
		t.Fatalf("Release error: %v", err)
	}

	// Next allocation should return the released IP
	ipAgain, err := pool.Allocate()
	if err != nil {
		t.Fatalf("Allocate after release error: %v", err)
	}
	if !ipAgain.Equal(ips[2]) {
		t.Errorf("Expected reallocated IP %v, got %v", ips[2], ipAgain)
	}
}

// TestMultiPrefixPools ensures that pools with different prefixes do not overlap
func TestMultiPrefixPools(t *testing.T) {
	// Two disjoint /64 networks
	poolA, _ := NewPool("2001:db8:0:a::", 64, 120, 1)
	poolB, _ := NewPool("2001:db8:0:b::", 64, 120, 1)

	seen := make(map[string]struct{})
	// Allocate 10 from each
	for _, pool := range []*Pool{poolA, poolB} {
		for i := 0; i < 10; i++ {
			ip, err := pool.Allocate()
			if err != nil {
				t.Fatalf("Allocate error: %v", err)
			}
			s := ip.String()
			if _, dup := seen[s]; dup {
				t.Errorf("Duplicate across pools: %s", s)
			}
			seen[s] = struct{}{}
		}
	}
}

// TestNewPoolInvalidParams verifies that NewPool rejects bad inputs
func TestNewPoolInvalidParams(t *testing.T) {
	cases := []struct {
		addr            string
		netPrefix, blk  int
		wantErrContains string
	}{
		{"not-an-ip", 64, 120, "invalid IPv6"},
		{"2001:db8::", -1, 120, "between 0 and 128"},
		{"2001:db8::", 64, 64, "block prefix must be"},
		{"2001:db8::", 64, 129, "block prefix must be"},
		{"2001:db8::", 0, 64, "too many blocks"},
	}

	for _, c := range cases {
		if _, err := NewPool(c.addr, c.netPrefix, c.blk, 1); err == nil || !contains(err.Error(), c.wantErrContains) {
			t.Errorf("NewPool(%q, %d, %d): err %v, want contains %q",
				c.addr, c.netPrefix, c.blk, err, c.wantErrContains)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (sub == s[:len(sub)] || sub == s[len(s)-len(sub):] || len(sub) == 0 || (len(s) > len(sub) && (contains(s[1:], sub) || contains(s[:len(s)-1], sub))))
}

// TestFreeListCapacity ensures we pre-reserved the freeList capacity
func TestFreeListCapacity(t *testing.T) {
	const exp = 42
	p, _ := NewPool("2001:db8::", 64, 120, exp)
	if cap(p.freeList) != exp {
		t.Errorf("freeList capacity = %d; want %d", cap(p.freeList), exp)
	}
}

// TestNetworkContainment ensures every allocated IP lives within the baseCIDR
func TestNetworkContainment(t *testing.T) {
	base := "2001:db8:abcd::"
	p, _ := NewPool(base, 64, 120, 10)
	for i := 0; i < 100; i++ {
		ip, _ := p.Allocate()
		if !net.ParseIP(base).Equal(ip.Mask(net.CIDRMask(64, 128))) {
			t.Errorf("Allocated %v outside /64 %s", ip, base)
		}
	}
}

// TestReleaseInvalid ensures releasing an IP outside the pool yields an error.
func TestReleaseInvalid(t *testing.T) {
	pool, _ := NewPool("2001:db8::", 64, 120, 16)
	invalid := net.ParseIP("2001:db8::1:1:1:1")
	err := pool.Release(invalid)
	if err == nil {
		t.Error("Expected error releasing invalid IP, got nil")
	}
}

// TestDoubleRelease ensures that releasing the same IP twice yields an error
// and does not corrupt the freeList.
func TestDoubleRelease(t *testing.T) {
	pool, _ := NewPool("2001:db8::", 64, 120, 1)
	ip, _ := pool.Allocate()
	if err := pool.Release(ip); err != nil {
		t.Fatalf("First release error: %v", err)
	}
	if err := pool.Release(ip); err == nil {
		t.Error("Expected error on double release, got nil")
	}
	ip2, err := pool.Allocate()
	if err != nil {
		t.Fatalf("Allocate after double-release error: %v", err)
	}
	if !ip2.Equal(ip) {
		t.Errorf("Expected to re-allocate %v, got %v", ip, ip2)
	}
}

// TestFullExhaustion ensures that once maxBlocks are used up, Allocate errors.
func TestFullExhaustion(t *testing.T) {
	// Use a tiny network so maxBlocks=2
	pool, _ := NewPool("2001:db8::", 126, 127, 2)
	// Each block yields 2^(127-126)=2 addresses -> maxBlocks = 2^(127-126)=2 blocks
	// Total addresses = 2 blocks * 2 addresses = 4
	for i := 0; i < 4; i++ {
		if _, err := pool.Allocate(); err != nil {
			t.Fatalf("Unexpected Allocate[%d] error: %v", i, err)
		}
	}
	// Fifth Allocate should exhaust
	if _, err := pool.Allocate(); err == nil {
		t.Error("Expected pool exhausted error, got nil")
	}
}

// TestReleaseUnknownIP ensures Release rejects IPs within the base CIDR but not on a block boundary.
func TestReleaseUnknownIP(t *testing.T) {
	pool, _ := NewPool("2001:db8::", 64, 120, 1)

	// Manually craft an IP in the base /64 but not one we ever Allocate()
	unallocatedIP := net.ParseIP("2001:db8::ff")
	if err := pool.Release(unallocatedIP); err == nil {
		t.Errorf("Expected error releasing unallocated IP %v, got nil", unallocatedIP)
	}
}

// TestUniqueAllocations ensures no duplicate IPs in sequential allocs.
func TestUniqueAllocations(t *testing.T) {
	n := 1000
	pool, _ := NewPool("2001:db8::", 64, 120, n)
	held := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		ip, err := pool.Allocate()
		if err != nil {
			t.Fatalf("Allocate error at %d: %v", i, err)
		}
		s := ip.String()
		if _, exists := held[s]; exists {
			t.Errorf("Duplicate IP allocated: %s", s)
		}
		held[s] = struct{}{}
	}
}

// TestConcurrentUnique ensures unique IPs under concurrent alloc.
func TestConcurrentUnique(t *testing.T) {
	n := 1000
	pool, _ := NewPool("2001:db8::", 64, 120, n)
	held := make(map[string]struct{}, n)
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ip, err := pool.Allocate()
			if err != nil {
				t.Errorf("Allocate error: %v", err)
				return
			}
			mu.Lock()
			s := ip.String()
			if _, exists := held[s]; exists {
				t.Errorf("Duplicate IP in concurrent alloc: %s", s)
			}
			held[s] = struct{}{}
			mu.Unlock()
		}()
	}
	wg.Wait()
}
