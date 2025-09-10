package blockchain

import (
	"math/big"
)

const (
	RecalculationFrequency = 2016
	TargetTimespan        = 1209600 // 2 weeks in seconds
	TargetSpacing         = 600     // 10 minutes in seconds
	MaxAdjustmentFactor   = 4       // Maximum 4x adjustment in either direction
)

// Bitcoin's max target (difficulty 1) - this is the easiest possible difficulty
// In hex: 0x00000000FFFF0000000000000000000000000000000000000000000000000000
// This is stored in compact format as 0x1d00ffff
const MaxTargetCompact uint32 = 0x1d00ffff

var MaxTarget = CompactToBig(MaxTargetCompact)

// CompactToBig converts a compact representation (bits) to a big.Int target
// Bitcoin's "bits" format: 0xAABBCCDD where AA is the exponent and BBCCDD is the mantissa
func CompactToBig(compact uint32) *big.Int {
	// Extract exponent and mantissa
	exponent := compact >> 24
	mantissa := compact & 0x00ffffff
	
	// Handle negative bit (0x00800000) - Bitcoin uses this for negative numbers
	negative := mantissa&0x00800000 != 0
	mantissa &= 0x007fffff
	
	// Calculate the target value
	target := big.NewInt(int64(mantissa))
	
	// Shift by the exponent
	// If exponent > 3, left shift by (exponent-3)*8 bits
	// If exponent <= 3, right shift
	if exponent > 3 {
		target.Lsh(target, 8*(uint(exponent)-3))
	} else {
		target.Rsh(target, 8*(3-uint(exponent)))
	}
	
	if negative {
		target.Neg(target)
	}
	
	return target
}

// BigToCompact converts a big.Int target to compact representation (bits)
func BigToCompact(target *big.Int) uint32 {
	if target.Sign() == 0 {
		return 0
	}
	
	// Make a copy to avoid modifying the original
	n := new(big.Int).Set(target)
	negative := n.Sign() < 0
	if negative {
		n.Neg(n)
	}
	
	// Convert to bytes
	bytes := n.Bytes()
	
	// Determine the exponent (number of bytes needed)
	exponent := uint32(len(bytes))
	
	// Extract mantissa (first 3 bytes)
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
	
	// Normalize: if the high byte is >= 0x80, we need to add a zero byte
	// This prevents the number from being interpreted as negative
	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}
	
	// Add negative bit if needed
	if negative {
		mantissa |= 0x00800000
	}
	
	// Combine exponent and mantissa
	return (exponent << 24) | mantissa
}

// CalculateDifficulty calculates the difficulty from a target
// Difficulty = max_target / current_target
func CalculateDifficulty(targetBits uint32) *big.Float {
	target := CompactToBig(targetBits)
	// Difficulty = MaxTarget / Target
	difficulty := new(big.Float).SetInt(MaxTarget)
	targetFloat := new(big.Float).SetInt(target)
	difficulty.Quo(difficulty, targetFloat)
	return difficulty
}

// GetTargetBits calculates the next target in compact format
// This is Bitcoin's actual difficulty adjustment algorithm
func GetTargetBits(height int, blocks []*Block) uint32 {
	// Genesis and early blocks use max target (easiest difficulty)
	if height == 0 || len(blocks) == 0 {
		return MaxTargetCompact
	}
	
	// No adjustment needed except at specific intervals
	if height%RecalculationFrequency != 0 {
		// Return the same target as the previous block
		return blocks[len(blocks)-1].Header.Bits
	}
	
	// Time to adjust difficulty
	// Get the first and last block of the adjustment period
	firstBlock := blocks[len(blocks)-RecalculationFrequency]
	lastBlock := blocks[len(blocks)-1]
	
	// Calculate actual time span
	actualTimespan := int64(lastBlock.Header.Timestamp - firstBlock.Header.Timestamp)
	
	// Limit adjustment to prevent huge swings
	adjustedTimespan := actualTimespan
	if adjustedTimespan < TargetTimespan/MaxAdjustmentFactor {
		adjustedTimespan = TargetTimespan / MaxAdjustmentFactor
	}
	if adjustedTimespan > TargetTimespan*MaxAdjustmentFactor {
		adjustedTimespan = TargetTimespan * MaxAdjustmentFactor
	}
	
	// Calculate new target
	// new_target = old_target * actual_timespan / target_timespan
	currentTarget := CompactToBig(lastBlock.Header.Bits)
	newTarget := new(big.Int).Mul(currentTarget, big.NewInt(adjustedTimespan))
	newTarget.Div(newTarget, big.NewInt(TargetTimespan))
	
	// Ensure new target doesn't exceed max target (minimum difficulty)
	if newTarget.Cmp(MaxTarget) > 0 {
		newTarget = MaxTarget
	}
	
	// Convert back to compact format
	return BigToCompact(newTarget)
}

// GetBlockWork calculates the work contribution of a single block
// Work = 2^256 / (target + 1)
func GetBlockWork(targetBits uint32) *big.Int {
	target := CompactToBig(targetBits)
	
	// Work = 2^256 / (target + 1)
	// We calculate this as (2^256 - 1) / target + 1 to avoid overflow
	work := new(big.Int).Lsh(big.NewInt(1), 256)
	work.Sub(work, big.NewInt(1))
	work.Div(work, target)
	work.Add(work, big.NewInt(1))
	
	return work
}

// BlockHashMeetsDifficulty checks if a block hash meets the target requirement
func BlockHashMeetsDifficulty(hash Hash32, targetBits uint32) bool {
	// Convert compact bits to target
	target := CompactToBig(targetBits)
	
	// Convert hash to big.Int
	hashInt := new(big.Int).SetBytes(hash[:])
	
	// Hash must be less than or equal to target
	return hashInt.Cmp(target) <= 0
}