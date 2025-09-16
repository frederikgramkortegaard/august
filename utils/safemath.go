package utils

import "fmt"

// SafeAdd performs addition with overflow check
func SafeAdd(a, b uint64) (uint64, error) {
	maxAmount := ^uint64(0) // Maximum uint64 value
	if a > maxAmount-b {
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
	maxAmount := ^uint64(0) // Maximum uint64 value
	if amount > maxAmount {
		return fmt.Errorf("amount %d exceeds maximum allowed value %d", amount, maxAmount)
	}
	return nil
}