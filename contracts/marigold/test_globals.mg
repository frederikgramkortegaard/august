// Test global storage variables
define test_memory() : string {
    // Use global memory array
    memory[0] = "first"
    memory[1] = "second"
    memory[5] = "fifth"  // Should work - dynamic array

    size: int = len(memory)
    first: string = memory[0]

    return first
}

define test_persistent() : string {
    // Use global persistent map
    persistent["user_count"] = "100"
    persistent["balance"] = "5000"

    count: string = persistent["user_count"]
    balance: string = persistent["balance"]

    return count
}

define test_both() : int {
    // Use both storage types together
    memory[0] = "memory_data"
    persistent["key"] = "persistent_data"

    mem_len: int = len(memory)

    // Store length in persistent
    persistent["memory_size"] = "calculated"

    return mem_len
}