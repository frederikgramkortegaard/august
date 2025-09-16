package blockchain

import (
	"fmt"
)

// Currency multipliers (like time package)
const (
	Leaf = uint64(1)                        // Base unit
	AUG  = uint64(1_000_000) * Leaf        // 1 AUG = 1M leaf units
)

const (
	BlockReward           = 50 * AUG              // 50 AUG block reward
	HalvingInterval       = 210000                // Blocks between reward halvings
	MaxAmount             = ^uint64(0)            // Maximum uint64 value for overflow checks
	MainnetChainID        = uint64(1)             // Main network chain ID
	TestnetChainID        = uint64(2)             // Test network chain ID
	ContractDeploymentFee = 1 * AUG               // 1 AUG deployment fee

	// Gas constants
	GasTransfer          = uint64(21000)          // Base gas for regular transfer
	GasContractDeploy    = uint64(21000)          // Base gas for contract deployment (before init execution)
	GasContractInitLimit = uint64(1000000)        // Max gas for contract init execution
)

// SafeAdd performs addition with overflow check
func SafeAdd(a, b uint64) (uint64, error) {
	if a > MaxAmount-b {
		return 0, fmt.Errorf("addition overflow: %d + %d exceeds maximum uint64", a, b)
	}
	return a + b, nil
}

// SafeSubtract performs subtraction with underflow check
func SafeSubtract(a, b uint64) (uint64, error) {
	if a < b {
		return 0, fmt.Errorf("subtraction underflow: %d - %d would be negative", a, b)
	}
	return a - b, nil
}

// ValidateAmount checks if an amount is within valid range
func ValidateAmount(amount uint64) error {
	if amount > MaxAmount {
		return fmt.Errorf("amount %d exceeds maximum allowed value %d", amount, MaxAmount)
	}
	return nil
}
