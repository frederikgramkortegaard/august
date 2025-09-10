package blockchain

import (
	"time"
)

const (
	RecalculationFrequency = 2016
)

type DifficultyAdjustment struct {
	BlockInterval    time.Duration
	AdjustmentPeriod int
}

func BlockHashMeetsDifficulty(hash Hash32, difficulty uint64) bool {
	// Convert hash to big integer for comparison
	// For simplicity, check if hash starts with enough zero bytes
	leadingZeros := 0
	for _, b := range hash {
		if b == 0 {
			leadingZeros += 8
		} else {
			for i := 7; i >= 0; i-- {
				if (b >> i) == 0 {
					leadingZeros++
				} else {
					break
				}
			}
			break
		}
	}
	return uint64(leadingZeros) >= difficulty
}

func GetTargetDifficulty(height int, blocks []*Block) uint64 {

	// Base Case, Genesis Difficulty
	if height < RecalculationFrequency {
		return 1
	}

	lastAdjustmentHeight := (height / RecalculationFrequency) * RecalculationFrequency
	if height == lastAdjustmentHeight {
		prevAdjustmentHeight := lastAdjustmentHeight - RecalculationFrequency
		prevDifficulty := GetTargetDifficulty(prevAdjustmentHeight, blocks)

		firstBlock := blocks[height-2016]
		lastBlock := blocks[height-1]

		actualTime := lastBlock.Header.Timestamp - firstBlock.Header.Timestamp
		expectedTime := uint64(RecalculationFrequency * 10 * 60)

		// Adjust difficulty based in time ratio
		newDifficulty := prevDifficulty * expectedTime / actualTime

		if newDifficulty > prevDifficulty*4 {
			newDifficulty = prevDifficulty * 4
		}

		if newDifficulty < prevDifficulty/4 {
			newDifficulty = prevDifficulty / 4
		}
		if newDifficulty == 0 {
			newDifficulty = 1
		}

		return newDifficulty
	} else {

		return GetTargetDifficulty(lastAdjustmentHeight, blocks)
	}

}
