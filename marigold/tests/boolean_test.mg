define test_literals() : bool {
	t: bool = true
	f: bool = false
	return t && f
}

define test_logical_and() : bool {
	return true && true
}

define test_logical_or() : bool {
	return false || true
}

define test_comparisons() : bool {
	a: int = 5
	b: int = 10

	less: bool = a < b
	greater: bool = b > a
	equal: bool = a == a
	not_equal: bool = a != b

	return less && greater && equal && not_equal
}

define test_nested_logic() : bool {
	x: int = 5
	y: int = 10
	z: int = 15

	condition1: bool = x < y && y < z
	condition2: bool = x == 5 || y == 5

	return condition1 && condition2
}