package blockchain

import (
	"august/config"
	"august/utils"
	"math/big"
)

// CalculateDifficulty calculates difficulty from target bits
func CalculateDifficulty(targetBits uint32) *big.Float {
	target := utils.CompactToBig(targetBits)
	maxTarget := utils.CompactToBig(config.TestTargetCompact) // Use test difficulty for development
	d := new(big.Float).SetInt(maxTarget)
	t := new(big.Float).SetInt(target)
	d.Quo(d, t)
	return d
}

// GetTargetBits calculates the next target in compact format
func GetTargetBits(height int, blocks []*Block) uint32 {
	if height == 0 || len(blocks) == 0 {
		return config.TestTargetCompact // Use easy difficulty for testing
	}

	if height%config.RecalculationFrequency != 0 {
		return blocks[len(blocks)-1].Header.Bits
	}

	firstBlock := blocks[len(blocks)-config.RecalculationFrequency]
	lastBlock := blocks[len(blocks)-1]

	actualTimespan := int64(lastBlock.Header.Timestamp - firstBlock.Header.Timestamp)

	if actualTimespan < config.TargetTimespan/config.MaxAdjustmentFactor {
		actualTimespan = config.TargetTimespan / config.MaxAdjustmentFactor
	}
	if actualTimespan > config.TargetTimespan*config.MaxAdjustmentFactor {
		actualTimespan = config.TargetTimespan * config.MaxAdjustmentFactor
	}

	currentTarget := utils.CompactToBig(lastBlock.Header.Bits)
	newTarget := new(big.Int).Mul(currentTarget, big.NewInt(actualTimespan))
	newTarget.Div(newTarget, big.NewInt(config.TargetTimespan))

	maxTarget := utils.CompactToBig(config.TestTargetCompact) // Use test difficulty for development
	if newTarget.Cmp(maxTarget) > 0 {
		newTarget = maxTarget
	}

	return utils.BigToCompact(newTarget)
}

// GetBlockWork calculates the work contribution of a single block
func GetBlockWork(targetBits uint32) *big.Int {
	target := utils.CompactToBig(targetBits)
	two256 := new(big.Int).Lsh(big.NewInt(1), 256)
	work := new(big.Int).Div(two256, new(big.Int).Add(target, big.NewInt(1)))
	return work
}

// BlockHashMeetsDifficulty checks if a block hash meets the target requirement
func BlockHashMeetsDifficulty(hash Hash32, targetBits uint32) bool {
	target := utils.CompactToBig(targetBits)
	hashInt := new(big.Int).SetBytes(hash[:])
	return hashInt.Cmp(target) <= 0
}
