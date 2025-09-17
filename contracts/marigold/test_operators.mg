// Test new arithmetic operators
define main() : int {
    // Test modulo operator
    remainder1: int = 10 % 3    // Should be 1
    remainder2: float = 10.5 % 3.0  // Should be 1.5

    emit("Modulo tests:")
    emit(remainder1)
    emit(remainder2)

    // Test exponentiation operator
    power1: float = 2.0 ^ 3.0   // Should be 8.0
    power2: float = 5 ^ 2       // Should be 25.0
    power3: float = 2 ^ -1      // Should be 0.5

    emit("Exponentiation tests:")
    emit(power1)
    emit(power2)
    emit(power3)

    // Test operator precedence
    // 2 + 3 * 4 % 5 ^ 2 should be: 2 + ((3 * 4) % (5 ^ 2)) = 2 + (12 % 25) = 2 + 12 = 14
    complex: float = 2.0 + 3.0 * 4.0 % 5.0 ^ 2.0

    emit("Complex expression:")
    emit(complex)

    return 0
}