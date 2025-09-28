define init() : int {
	// Test simple string indexing first
	test: string = "hello"
	char: string = test[0]  // Should be "h"
	persistent["char"] = char

	// Test version string specifically
	version: string = "v2.0.0"
	char2: string = version[2]  // Should be "0"
	persistent["char2"] = char2

	return 1
}

define call() : int {
	return 1
}