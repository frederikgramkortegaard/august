// Math Library Contract - showcases arithmetic operations and type conversions
define calculate_statistics(numbers: []int) : map[string]float {
    stats: map[string]float = {}

    if len(numbers) == 0 {
        stats["sum"] = 0.0
        stats["average"] = 0.0
        stats["min"] = 0.0
        stats["max"] = 0.0
        return stats
    }

    // Calculate sum
    sum: int = 0
    i: int = 0
    while i < len(numbers) {
        sum = sum + numbers[i]
        i = i + 1
    }

    // Convert to float for average calculation
    sum_float: float = sum  // int to float conversion
    count_float: float = len(numbers)  // int to float conversion
    average: float = sum_float / count_float  // Division always returns float

    stats["sum"] = sum_float
    stats["average"] = average

    // Find min and max
    min_val: int = numbers[0]
    max_val: int = numbers[0]

    i = 1
    while i < len(numbers) {
        current: int = numbers[i]
        if current < min_val {
            min_val = current
        }
        if current > max_val {
            max_val = current
        }
        i = i + 1
    }

    stats["min"] = min_val  // int to float conversion
    stats["max"] = max_val  // int to float conversion

    return stats
}

define matrix_multiply_2x2(a: [4]float, b: [4]float) : [4]float {
    // Treat arrays as 2x2 matrices: [a00, a01, a10, a11]
    result: [4]float

    // result[0] = a[0]*b[0] + a[1]*b[2]  (top-left)
    result[0] = a[0] * b[0] + a[1] * b[2]

    // result[1] = a[0]*b[1] + a[1]*b[3]  (top-right)
    result[1] = a[0] * b[1] + a[1] * b[3]

    // result[2] = a[2]*b[0] + a[3]*b[2]  (bottom-left)
    result[2] = a[2] * b[0] + a[3] * b[2]

    // result[3] = a[2]*b[1] + a[3]*b[3]  (bottom-right)
    result[3] = a[2] * b[1] + a[3] * b[3]

    return result
}

define calculate_compound_interest(principal: float, rate: float, years: int) : float {
    // Compound interest: A = P(1 + r)^n
    // Simplified without exponentiation - just iterate
    amount: float = principal
    annual_rate: float = 1.0 + rate

    i: int = 0
    while i < years {
        amount = amount * annual_rate
        i = i + 1
    }

    return amount
}

define generate_fibonacci(count: int) : []int {
    if count <= 0 {
        return []
    }

    if count == 1 {
        return [0]
    }

    if count == 2 {
        return [0, 1]
    }

    // For larger counts, we'd need dynamic arrays
    // For demo, return first few fibonacci numbers
    fib: [10]int

    fib[0] = 0
    fib[1] = 1

    i: int = 2
    while i < count && i < 10 {  // Limit to array size
        fib[i] = fib[i-1] + fib[i-2]
        i = i + 1
    }

    // Return fixed array (would return slice in full implementation)
    return fib
}

define calculate_distance(x1: float, y1: float, x2: float, y2: float) : float {
    // Distance formula: sqrt((x2-x1)² + (y2-y1)²)
    // Without sqrt, just return squared distance
    dx: float = x2 - x1
    dy: float = y2 - y1
    squared_distance: float = dx * dx + dy * dy

    return squared_distance
}

define analyze_number_array(numbers: []int) : map[string]float {
    stats: map[string]float = calculate_statistics(numbers)

    // Add additional analysis
    sum: float = stats["sum"]
    count: int = len(numbers)
    count_float: float = count

    // Calculate variance (simplified)
    average: float = stats["average"]
    variance_sum: float = 0.0

    i: int = 0
    while i < count {
        value: float = numbers[i]  // int to float conversion
        diff: float = value - average
        squared_diff: float = diff * diff
        variance_sum = variance_sum + squared_diff
        i = i + 1
    }

    variance: float = variance_sum / count_float
    stats["variance"] = variance

    // Sum of squares
    sum_of_squares: float = 0.0
    i = 0
    while i < count {
        value: float = numbers[i]  // int to float conversion
        squared: float = value * value
        sum_of_squares = sum_of_squares + squared
        i = i + 1
    }

    stats["sum_of_squares"] = sum_of_squares

    return stats
}