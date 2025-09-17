// Test break and continue statements
define main() : int {
    emit("Testing break and continue")

    // Test continue - skip even numbers, emit odd numbers
    i: int = 0
    while i < 10 {
        i = i + 1

        // Skip even numbers
        if i % 2 == 0 {
            continue
        }

        emit(i)  // Should emit 1, 3, 5, 7, 9
    }

    emit("Testing break")

    // Test break - stop at first number > 5
    j: int = 0
    while j < 20 {
        j = j + 1

        if j > 5 {
            emit("Breaking at:")
            emit(j)
            break
        }

        emit(j)  // Should emit 1, 2, 3, 4, 5
    }

    emit("Loop tests completed")
    return 0
}