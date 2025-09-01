package blockchain

type NonceType = uint64

const MaxTarget = 0x00000000FFFF0000000000000000000000000000000000000000000000000000

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

func MineCorrectNonce(header *BlockHeader, difficulty uint64) (NonceType, error) {

	for hash := HashBlockHeader(header); !BlockHashMeetsDifficulty(hash, difficulty); hash = HashBlockHeader(header) {
		header.Nonce += 1
	}

	return header.Nonce, nil

}
