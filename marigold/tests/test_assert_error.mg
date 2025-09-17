// Test assert() with wrong argument type (should fail compilation)
define main() : int {
    emit("This should fail to compile")
    assert("not a boolean")  // Error: assert expects boolean
    return 0
}