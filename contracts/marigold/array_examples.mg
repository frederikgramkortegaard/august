define array_demo() : int {
	// Explicit size
	explicit: [3]int = [10, 20, 30]

	// Inferred size
	inferred: []string = ["hello", "world", "test"]

	// Just declaration (uninitialized)
	uninitialized: [5]bool

	// Use the arrays
	first: int = explicit[0]
	second: string = inferred[1]
	size1: int = len(explicit)
	size2: int = len(inferred)

	return first + size1 + size2
}