package blockchain

import (
	. "august/types"
	"math/big"
)

const (
	RecalculationFrequency = 2016
	TargetTimespan         = 1209600 // 2 weeks in seconds
	TargetSpacing          = 600     // 10 minutes in seconds
	MaxAdjustmentFactor    = 4       // Maximum 4x adjustment in either direction
)

// Bitcoin's max target (difficulty 1) - easiest possible difficulty
const MaxTargetCompact uint32 = 0x1d00ffff

// Super easy target for testing - requires only 8 leading zero bits
const TestTargetCompact uint32 = 0x1f00ffff

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

// CalculateDifficulty calculates difficulty from target bits
func CalculateDifficulty(targetBits uint32) *big.Float {
	target := CompactToBig(targetBits)
	maxTarget := CompactToBig(TestTargetCompact) // Use test difficulty for development
	d := new(big.Float).SetInt(maxTarget)
	t := new(big.Float).SetInt(target)
	d.Quo(d, t)
	return d
}

// GetTargetBits calculates the next target in compact format
func GetTargetBits(height int, blocks []*Block) uint32 {
	if height == 0 || len(blocks) == 0 {
		return TestTargetCompact // Use easy difficulty for testing
	}

	if height%RecalculationFrequency != 0 {
		return blocks[len(blocks)-1].Header.Bits
	}

	firstBlock := blocks[len(blocks)-RecalculationFrequency]
	lastBlock := blocks[len(blocks)-1]

	actualTimespan := int64(lastBlock.Header.Timestamp - firstBlock.Header.Timestamp)

	if actualTimespan < TargetTimespan/MaxAdjustmentFactor {
		actualTimespan = TargetTimespan / MaxAdjustmentFactor
	}
	if actualTimespan > TargetTimespan*MaxAdjustmentFactor {
		actualTimespan = TargetTimespan * MaxAdjustmentFactor
	}

	currentTarget := CompactToBig(lastBlock.Header.Bits)
	newTarget := new(big.Int).Mul(currentTarget, big.NewInt(actualTimespan))
	newTarget.Div(newTarget, big.NewInt(TargetTimespan))

	maxTarget := CompactToBig(TestTargetCompact) // Use test difficulty for development
	if newTarget.Cmp(maxTarget) > 0 {
		newTarget = maxTarget
	}

	return BigToCompact(newTarget)
}

// GetBlockWork calculates the work contribution of a single block
func GetBlockWork(targetBits uint32) *big.Int {
	target := CompactToBig(targetBits)
	two256 := new(big.Int).Lsh(big.NewInt(1), 256)
	work := new(big.Int).Div(two256, new(big.Int).Add(target, big.NewInt(1)))
	return work
}

// BlockHashMeetsDifficulty checks if a block hash meets the target requirement
func BlockHashMeetsDifficulty(hash Hash32, targetBits uint32) bool {
	target := CompactToBig(targetBits)
	hashInt := new(big.Int).SetBytes(hash[:])
	return hashInt.Cmp(target) <= 0
}
