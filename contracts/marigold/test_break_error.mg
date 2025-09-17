// Test invalid break usage
define main() : int {
    emit("This should fail")
    break  // Error: not in a loop
    return 0
}