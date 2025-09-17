// Test invalid continue usage
define main() : int {
    if true {
        continue  // Error: not in a loop
    }
    return 0
}