// Test built-in functions: emit() and stop()
define test_emit() : int {
    // Test emit with different simple types
    emit(42)
    emit(3.14)
    emit("hello world")
    emit(true)

    return 0
}

define test_stop() : int {
    // Test stop function (no parameters)
    value: int = 5
    if value > 0 {
        stop()  // Should exit here
    }

    // This should never be reached
    return 1
}

define test_emit_with_variables() : int {
    count: int = 100
    rate: float = 2.5
    message: string = "processing"
    done: bool = false

    emit(count)
    emit(rate)
    emit(message)
    emit(done)

    return count
}

define demo_workflow() : string {
    // Simulate a workflow with emit and conditional stop
    memory[0] = "workflow_started"
    persistent["status"] = "running"

    emit("Starting workflow...")

    step1: int = 10
    emit(step1)

    if step1 > 5 {
        emit("Step 1 completed")
    }

    // Check for early termination
    should_stop: bool = true
    if should_stop {
        emit("Terminating early")
        stop()  // Exit here
    }

    return "workflow_completed"  // Never reached
}