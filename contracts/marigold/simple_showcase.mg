// Simple Type System Showcase - demonstrates all major features working together
define demo_basic_types() : int {
    // Basic types
    count: int = 42
    rate: float = 3.14
    message: string = "hello"
    flag: bool = true

    return count
}

define demo_arrays() : int {
    // Array operations
    numbers: [5]int = [10, 20, 30, 40, 50]
    names: []string = ["alice", "bob", "charlie"]

    // Array indexing
    first_num: int = numbers[0]
    second_name: string = names[1]

    // Array modification
    numbers[2] = 35

    // Array length
    count: int = len(numbers)
    name_count: int = len(names)

    return first_num + count + name_count
}

define demo_maps() : int {
    // Map creation and usage
    scores: map[string]int = {}
    grades: map[string]float = {}

    // Map assignment
    scores["alice"] = 95
    scores["bob"] = 87
    scores["charlie"] = 92

    grades["alice"] = 3.8
    grades["bob"] = 3.2
    grades["charlie"] = 3.6

    // Map lookup
    alice_score: int = scores["alice"]
    bob_grade: float = grades["bob"]

    // Convert float to int for return
    bob_grade_int: int = bob_grade

    return alice_score + bob_grade_int
}

define demo_string_operations() : int {
    // String operations
    text: string = "programming"
    first_char: string = text[0]
    length: int = len(text)

    // String concatenation
    greeting: string = "Hello"
    name: string = "World"
    message: string = greeting + " " + name

    message_length: int = len(message)

    return length + message_length
}

define demo_arithmetic() : int {
    // Mixed arithmetic operations
    base: int = 10
    multiplier: float = 2.5
    bonus: int = 5

    // Mixed operations (int/float arithmetic)
    result1: float = base * multiplier
    result2: float = result1 + bonus
    final_result: int = result2  // Float to int conversion

    // Division always returns float
    division_result: float = base / 3
    division_int: int = division_result

    return final_result + division_int
}

define demo_comprehensive_example() : int {
    // Create a map of scores
    student_scores: map[string]int = {}
    student_scores["alice"] = 95
    student_scores["bob"] = 87
    student_scores["charlie"] = 92

    // Create array of student names
    students: [3]string = ["alice", "bob", "charlie"]

    // Calculate total score
    total: int = 0
    i: int = 0
    while i < len(students) {
        student: string = students[i]
        score: int = student_scores[student]
        total = total + score
        i = i + 1
    }

    // Calculate average (using float division)
    count: int = len(students)
    average_float: float = total / count
    average: int = average_float

    // Find highest score
    highest: int = 0
    i = 0
    while i < len(students) {
        student: string = students[i]
        score: int = student_scores[student]
        if score > highest {
            highest = score
        }
        i = i + 1
    }

    // String operations for demo
    class_name: string = "CS101"
    report_title: string = "Grade Report: " + class_name
    title_length: int = len(report_title)

    return total + average + highest + title_length
}