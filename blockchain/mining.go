package blockchain

type NonceType = uint64

const (
	// Currency units: 1 AUG = 10^18 leaf
	LeafPerAUG      = 1_000_000_000_000_000_000 // 10^18
	BlockReward     = 50 * LeafPerAUG           // 50 AUG in leaf units
	HalvingInterval = 210000                    // Blocks between reward halvings
)
