define init() : int {
	// Test @tsxdata access
	dataLength: int = len(@tsxdata)
	persistent["data_length"] = string(dataLength)

	// Store the transaction data
	persistent["tsx_data"] = @tsxdata

	emit(dataLength)
	return 1
}

define call() : int {
	// Test @tsxdata at runtime
	dataLength: int = len(@tsxdata)
	persistent["runtime_data_length"] = string(dataLength)
	persistent["runtime_tsx_data"] = @tsxdata

	emit(dataLength)
	return dataLength
}