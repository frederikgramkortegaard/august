package config

import "august/types"

// Difficuly / Targets / Recalculations
const (
	RecalculationFrequency = 2016
	TargetTimespan         = 1209600 // 2 weeks in seconds
	TargetSpacing          = 600     // 10 minutes in seconds
	MaxAdjustmentFactor    = 4       // Maximum 4x adjustment in either direction
	Difficulty             = 1

	// Bitcoin's max target (difficulty 1) - easiest possible difficulty
	MaxTargetCompact uint32 = 0x1d00ffff
	// Super easy target for testing - requires only 8 leading zero bits
	TestTargetCompact uint32 = 0x1f00ffff
)

// Currency multipliers (like time package)
const (
	Leaf = uint64(1)                // Base unit
	AUG  = uint64(1_000_000) * Leaf // 1 AUG = 1M leaf units
)

// Mining / Gas
const (
	BlockReward           = 50 * AUG   // 50 AUG block reward
	HalvingInterval       = 210000     // Blocks between reward halvings
	MaxAmount             = ^uint64(0) // Maximum uint64 value for overflow checks
	MainnetChainID        = uint64(1)  // Main network chain ID
	TestnetChainID        = uint64(2)  // Test network chain ID
	ContractDeploymentFee = 1 * AUG    // 1 AUG deployment fee

	// Gas constants
	GasTransfer          = uint64(21000)   // Base gas for regular transfer
	GasContractDeploy    = uint64(21000)   // Base gas for contract deployment (before init execution)
	GasContractInitLimit = uint64(1000000) // Max gas for contract init execution
)

// Test user for genesis block
var FirstUser = types.PublicKey{
	0x04, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde,
	0xf0, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
	0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
	0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
}
