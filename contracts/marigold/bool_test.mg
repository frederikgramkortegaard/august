define test_booleans(x: int, y: int) : bool {
	x_positive: bool = x > 0
	y_positive: bool = y > 0

	if x_positive && y_positive {
		return true
	} else {
		return false
	}
}

define test_logic() : bool {
	a: bool = true
	b: bool = false

	result1: bool = a && b
	result2: bool = a || b

	return result2
}