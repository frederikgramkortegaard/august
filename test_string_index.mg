define init() : int {
	data: string = "hello"
	firstChar: string = data[0]
	persistent["first"] = firstChar
	return 1
}

define call() : int {
	data: string = @tsxdata
	if len(data) > 3 {
		prefix: string = data[0:3]
		persistent["prefix"] = prefix
	}
	return 1
}