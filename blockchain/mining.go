package blockchain

type NonceType = uint64

const (
	// Currency units: 1 AUG = 10^18 leaf
	LeafPerAUG      = uint64(1_000_000_000_000_000_000) // 10^18
	BlockReward     = uint64(5_000_000_000_000_000_000) // 5 AUG in leaf units (reduced to fit uint64)
	HalvingInterval = 210000                            // Blocks between reward halvings
)
