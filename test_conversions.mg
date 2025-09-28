define init() : int {
	// Test string to int conversion
	strNum: string = "123"
	intVal: int = int(strNum)
	persistent["int_result"] = string(intVal)

	// Test negative numbers
	strNeg: string = "-456"
	intNeg: int = int(strNeg)
	persistent["neg_result"] = string(intNeg)

	// Test int to string conversion
	bigNum: int = 789
	strResult: string = string(bigNum)
	persistent["str_result"] = strResult

	return 1
}

define call() : int {
	return 1
}