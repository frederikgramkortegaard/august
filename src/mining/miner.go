package mining

import (
	"gocuria/src/config"
	"gocuria/src/hashing"
	"gocuria/src/models"
)

type NonceType = uint64

func BlockHashMeetsDifficulty(hash [32]byte) bool {

	if config.Difficulty > len(hash) {
		return false
	}

	for i := 0; i < config.Difficulty; i++ {
		if hash[i] != 0 {
			return false
		}
	}

	return true

}

func MineCorrectNonce(header *models.BlockHeader) (NonceType, error) {

	for hash := hashing.HashBlockHeader(header); !BlockHashMeetsDifficulty(hash); hash = hashing.HashBlockHeader(header) {
		header.Nonce += 1
	}

	return header.Nonce, nil

}
