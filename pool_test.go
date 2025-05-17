package cidrx //nolint:testpackage // it's OK to be just cidrx

import (
	"net"
	"sync"
	"testing"
)

// TestAllocateRelease ensures that an allocated IP can be released and reallocated.
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

// TestExhaustion verifies that exhaustion returns an error.
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

// TestReleaseInvalid ensures releasing an IP outside the pool yields an error.
func TestReleaseInvalid(t *testing.T) {
	pool, _ := NewPool("2001:db8::", 64, 120, 16)
	invalid := net.ParseIP("2001:db8::1:1:1:1")
	err := pool.Release(invalid)
	if err == nil {
		t.Error("Expected error releasing invalid IP, got nil")
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
