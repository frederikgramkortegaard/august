define init() : int {
  // Initialize counter to 42
  persistent["counter"] = "42"
  return 0
}

define call() : int {
  // Get current counter value
  currentStr: string = persistent["counter"]
  current: int = 0
  if len(currentStr) > 0 {
    current = int(currentStr)
  }

  // Increment counter
  newValue: int = current + 1
  persistent["counter"] = string(newValue)

  // Emit new value
  emit(newValue)
  return newValue
}