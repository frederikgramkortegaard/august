define init() : int {
	return 1
}

define call() : int {
	if len(@tsxdata) >= 3 {
		prefix: string = @tsxdata[:3]
		if prefix == "buy" {
			persistent["action"] = "buy_detected"
			emit(1)
			return 1
		}
	}

	persistent["action"] = "other"
	emit(0)
	return 0
}