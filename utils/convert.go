package utils

import (
	"encoding/binary"
	"math/big"
)

// Uint64ToBytes converts uint64 to big-endian byte slice
func Uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

// Uint32ToBytes converts uint32 to big-endian byte slice
func Uint32ToBytes(n uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	return b
}

// CompactToBig converts a compact representation (bits) to a big.Int target
func CompactToBig(compact uint32) *big.Int {
	exponent := compact >> 24
	mantissa := compact & 0x007fffff // ignore sign bit

	target := big.NewInt(int64(mantissa))

	if exponent > 3 {
		target.Lsh(target, 8*(uint(exponent)-3))
	} else {
		target.Rsh(target, 8*(3-uint(exponent)))
	}

	return target
}

// BigToCompact converts a big.Int target to compact representation (bits)
func BigToCompact(target *big.Int) uint32 {
	if target.Sign() <= 0 {
		return 0
	}

	n := new(big.Int).Set(target)
	bytes := n.Bytes()
	exponent := uint32(len(bytes))

	var mantissa uint32
	if exponent > 0 {
		mantissa = uint32(bytes[0]) << 16
	}
	if exponent > 1 {
		mantissa |= uint32(bytes[1]) << 8
	}
	if exponent > 2 {
		mantissa |= uint32(bytes[2])
	}

	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}

	return (exponent << 24) | mantissa
}
