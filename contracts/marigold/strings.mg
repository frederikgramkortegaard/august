define greet(name: string) : string {
	return "Hello, " + name
}

define repeat(text: string, count: int) : string {
	result: string = ""
	i: int = 0
	while i < count {
		result = result + text
		i = i + 1
	}
	return result
}
