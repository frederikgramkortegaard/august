define countdown(start: int) : int {
	i: int = start
	while i > 0 {
		emit i
		i = i - 1
	}
	return 0
}

define sum_range(from: int, to: int) : int {
	total: int = 0
	i: int = from
	while i <= to {
		total = total + i
		i = i + 1
	}
	return total
}