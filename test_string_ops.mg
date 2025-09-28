define init() : int {
	data: string = "hello world"

	// Test string indexing
	firstChar: string = data[0]
	lastChar: string = data[10]
	persistent["first"] = firstChar
	persistent["last"] = lastChar

	// Test string slicing
	prefix: string = data[0:5]
	suffix: string = data[6:]
	middle: string = data[2:8]

	persistent["prefix"] = prefix
	persistent["suffix"] = suffix
	persistent["middle"] = middle

	emit(len(prefix))
	emit(len(suffix))

	return 1
}

define call() : int {
	data: string = @tsxdata
	if len(data) >= 3 {
		// Test indexing with @tsxdata
		char0: string = data[0]
		char1: string = data[1]
		char2: string = data[2]

		// Test slicing with @tsxdata
		first3: string = data[0:3]

		persistent["tsx_char0"] = char0
		persistent["tsx_char1"] = char1
		persistent["tsx_char2"] = char2
		persistent["tsx_first3"] = first3

		emit(len(first3))
	}
	return 1
}