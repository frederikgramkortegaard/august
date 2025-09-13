package blockchain

import (
	"math/big"
)

// Bitcoin mainnet difficulty-1 target
// 0x00000000FFFF0000000000000000000000000000000000000000000000000000
var maxTarget, _ = new(big.Int).SetString("00000000FFFF0000000000000000000000000000000000000000000000000000", 16)

func getMaxTarget() *big.Int {
	return new(big.Int).Set(maxTarget) // return a copy for safety
}

// CalculateBlockWork computes the expected work for a given difficulty.
// work = floor(2^256 / (target + 1))
func CalculateBlockWork(difficulty uint64) *big.Int {
	if difficulty == 0 {
		return big.NewInt(0)
	}

	target := new(big.Int).Div(getMaxTarget(), big.NewInt(int64(difficulty)))

	two256 := new(big.Int).Lsh(big.NewInt(1), 256)
	denom := new(big.Int).Add(target, big.NewInt(1))

	return new(big.Int).Div(two256, denom)
}

// AddWork adds two chainwork values given as decimal strings.
func AddWork(work1Str, work2Str string) string {
	work1 := new(big.Int)
	work2 := new(big.Int)

	if work1Str != "" {
		work1.SetString(work1Str, 10)
	}
	if work2Str != "" {
		work2.SetString(work2Str, 10)
	}

	return new(big.Int).Add(work1, work2).String()
}

// CompareWork compares two chainwork values given as decimal strings.
// Returns -1 if work1 < work2, 0 if equal, 1 if work1 > work2.
func CompareWork(work1Str, work2Str string) int {
	work1 := new(big.Int)
	work2 := new(big.Int)

	work1.SetString(work1Str, 10)
	work2.SetString(work2Str, 10)

	return work1.Cmp(work2)
}
