// Test assert() function
define main() : int {
    emit("Testing assert() function")

    // Test basic assertions that should pass
    assert(true)
    assert(1 == 1)
    assert(5 > 3)
    assert("hello" == "hello")

    // Test boolean variables
    isValid: bool = true
    assert(isValid)

    // Test complex boolean expressions
    x: int = 10
    y: int = 5
    assert(x > y && y > 0)
    assert(x % 2 == 0 || y % 2 == 1)

    // Test with arithmetic
    assert(x + y == 15)
    assert(x - y == 5)
    assert(x * y == 50)
    assert(x / y == 2)

    emit("All assertions passed!")
    return 0
}