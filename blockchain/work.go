package blockchain

import (
	"math/big"
)

// Maximum target (2^256 - 1)
var maxTarget = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

// CalculateBlockWork calculates the amount of work represented by a given difficulty
// Work = 2^256 / difficulty_target
func CalculateBlockWork(difficulty uint64) *big.Int {
	if difficulty == 0 {
		return big.NewInt(0)
	}

	// Calculate target from difficulty
	// target = max_target / difficulty
	target := new(big.Int).Div(maxTarget, big.NewInt(int64(difficulty)))
	
	// Work = 2^256 / target
	// But since target = max_target / difficulty, we have:
	// Work = 2^256 / (max_target / difficulty) = (2^256 * difficulty) / max_target
	// Since max_target ≈ 2^256, this simplifies to approximately: difficulty
	// But let's do it properly:
	
	work := new(big.Int).Div(maxTarget, target)
	return work
}

// AddWork adds two work values (both as big.Int strings)
func AddWork(work1Str, work2Str string) string {
	work1 := new(big.Int)
	work2 := new(big.Int)
	
	// Parse existing work values
	if work1Str != "" {
		work1.SetString(work1Str, 10)
	}
	if work2Str != "" {
		work2.SetString(work2Str, 10)
	}
	
	// Add them
	total := new(big.Int).Add(work1, work2)
	return total.String()
}

// CompareWork compares two work values, returns:
// -1 if work1 < work2
//  0 if work1 == work2
//  1 if work1 > work2
func CompareWork(work1Str, work2Str string) int {
	work1 := new(big.Int)
	work2 := new(big.Int)
	
	work1.SetString(work1Str, 10)
	work2.SetString(work2Str, 10)
	
	return work1.Cmp(work2)
}