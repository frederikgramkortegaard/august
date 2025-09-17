define test_errors(x: int) : int {
	// This should cause a type error - condition should be Bool
	if x {
		return 1
	}

	// This should cause an undefined variable error
	return unknown_var
}