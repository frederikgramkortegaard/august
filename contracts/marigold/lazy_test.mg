// Test that has() function is removed
define init() : int {
  return 0
}

define call() : int {
  test : bool = has(persistent, "key")  // This should error
  return 0
}