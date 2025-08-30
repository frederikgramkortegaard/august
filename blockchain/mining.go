package blockchain


type NonceType = uint64

func BlockHashMeetsDifficulty(hash [32]byte) bool {

	if Difficulty > len(hash) {
		return false
	}

	for i := 0; i < Difficulty; i++ {
		if hash[i] != 0 {
			return false
		}
	}

	return true

}

func MineCorrectNonce(header *BlockHeader) (NonceType, error) {

	for hash := HashBlockHeader(header); !BlockHashMeetsDifficulty(hash); hash = HashBlockHeader(header) {
		header.Nonce += 1
	}

	return header.Nonce, nil

}
