package cidrx

import (
	"encoding/binary"
	"math/bits"
	"net"
)

// Uint128 represents a 128-bit unsigned integer as two 64-bit words
type Uint128 struct {
	Hi, Lo uint64
}

// add returns the sum x + y
func (x Uint128) add(y Uint128) Uint128 {
	lo, carry := bits.Add64(x.Lo, y.Lo, 0)
	hi, _ := bits.Add64(x.Hi, y.Hi, carry)
	return Uint128{Hi: hi, Lo: lo}
}

// sub returns the difference x - y
// Assumes x >= y
func (x Uint128) sub(y Uint128) Uint128 {
	lo, borrow := bits.Sub64(x.Lo, y.Lo, 0)
	hi, _ := bits.Sub64(x.Hi, y.Hi, borrow)
	return Uint128{Hi: hi, Lo: lo}
}

// lsh shifts x left by k bits (0<=k<128)
func (x Uint128) lsh(k uint) Uint128 {
	if k >= 64 {
		return Uint128{Hi: x.Lo << (k - 64), Lo: 0}
	}
	hi := x.Hi<<k | x.Lo>>(64-k)
	lo := x.Lo << k
	return Uint128{Hi: hi, Lo: lo}
}

// rsh shifts x right by k bits (0<=k<128)
func (x Uint128) rsh(k uint) Uint128 {
	if k >= 64 {
		return Uint128{Hi: 0, Lo: x.Hi >> (k - 64)}
	}
	lo := x.Lo>>k | x.Hi<<(64-k)
	hi := x.Hi >> k
	return Uint128{Hi: hi, Lo: lo}
}

// toIP converts a Uint128 to a 16-byte net.IP
func (x Uint128) toIP() net.IP {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[:8], x.Hi)
	binary.BigEndian.PutUint64(buf[8:], x.Lo)
	return net.IP(buf)
}

// fromIP parses a 16-byte net.IP into a Uint128
func fromIP(ip net.IP) Uint128 {
	b := ip.To16()
	hi := binary.BigEndian.Uint64(b[:8])
	lo := binary.BigEndian.Uint64(b[8:])
	return Uint128{Hi: hi, Lo: lo}
}
