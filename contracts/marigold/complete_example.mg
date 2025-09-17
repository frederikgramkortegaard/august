// Complete example showcasing all features: global storage, built-ins, and main function
define process_data(count: int) : int {
    // Store data in memory (temporary)
    memory[0] = "processing"
    memory[1] = "started"

    // Store results in persistent storage
    persistent["status"] = "active"
    persistent["processed_count"] = "calculating"

    // Use emit to output progress
    emit("Processing started")
    emit(count)

    // Process the data
    result: int = count * 2

    // Update memory with result info
    memory[2] = "completed"

    // Update persistent storage
    persistent["last_result"] = "finished"
    persistent["status"] = "completed"

    emit("Processing completed")
    emit(result)

    return result
}

define check_system() : bool {
    // Check if system is properly initialized
    status: string = persistent["status"]
    memory_state: string = memory[0]

    memory_size: int = len(memory)
    emit("System check")
    emit(memory_size)

    // Simple validation
    if len(status) > 0 {
        return true
    }

    return false
}

define main() : int {
    emit("Smart Contract Starting")

    // Initialize system
    persistent["version"] = "1.0"
    persistent["status"] = "initializing"
    memory[0] = "system_start"

    // Process some data
    result1: int = process_data(10)
    result2: int = process_data(25)

    // Check system health
    system_ok: bool = check_system()

    if system_ok {
        emit("System is healthy")
        total: int = result1 + result2
        emit(total)

        // Store final results
        persistent["final_total"] = "calculated"
        memory[10] = "final_state"

        emit("Contract execution completed successfully")
        return total
    } else {
        emit("System error detected")
        stop()  // Exit early
        return -1  // Never reached
    }
}