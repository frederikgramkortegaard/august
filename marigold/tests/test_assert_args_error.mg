// Test assert() with wrong number of arguments (should fail compilation)
define main() : int {
    emit("This should fail to compile")
    assert()  // Error: assert expects 1 argument
    return 0
}