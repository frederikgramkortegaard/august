define add(x: int, y: int) : int {
	return x + y
}

define subtract(a: int, b: int) : int {
	return a - b
}

define multiply(x: int, y: int) : int {
	return x * y
}

define factorial(n: int) : int {
	if n <= 1 {
		return 1
	} else {
		return n * factorial(n - 1)
	}
}