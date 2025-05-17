package cidrx //nolint:testpackage // it's OK to be just cidrx

import (
	"net"
	"testing"
)

func TestUint128AddSub(t *testing.T) {
	a := Uint128{Hi: 1, Lo: 0}
	b := Uint128{Hi: 0, Lo: 1}
	sum := a.add(b)
	expectedSum := Uint128{Hi: 1, Lo: 1}
	if sum != expectedSum {
		t.Errorf("add: got %v, want %v", sum, expectedSum)
	}
	delta := sum.sub(b)
	if delta != a {
		t.Errorf("sub: got %v, want %v", delta, a)
	}
}

func TestUint128Shifts(t *testing.T) {
	// Test left shift
	x := Uint128{Hi: 0, Lo: 1}
	y := x.lsh(64)
	if y.Hi != 1 || y.Lo != 0 {
		t.Errorf("lsh64: got %v, want {1,0}", y)
	}
	// Test right shift
	z := y.rsh(64)
	if z != x {
		t.Errorf("rsh64: got %v, want %v", z, x)
	}
}

func TestUint128IPConversion(t *testing.T) {
	ips := []string{
		"::1",
		"2001:db8::",
		"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
	}
	for _, s := range ips {
		raw := net.ParseIP(s)
		u := fromIP(raw)
		back := u.toIP()
		if !back.Equal(raw) {
			t.Errorf("IP conversion: got %v, want %v", back, raw)
		}
	}
}
