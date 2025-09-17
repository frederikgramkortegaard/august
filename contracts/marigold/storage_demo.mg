// Storage Demo - demonstrates memory and persistent storage
define init_storage() : int {
    // Initialize memory array
    memory[0] = "initialized"
    memory[1] = "ready"
    memory[10] = "sparse"  // Dynamic array - can assign to any index

    // Initialize persistent storage
    persistent["status"] = "active"
    persistent["version"] = "1.0"
    persistent["user_count"] = "0"

    return len(memory)
}

define user_registration(username: string, email: string) : bool {
    if len(username) == 0 {
        return false
    }

    // Store user data in persistent storage
    user_key: string = "user_" + username
    persistent[user_key + "_email"] = email
    persistent[user_key + "_status"] = "active"

    // Update user count in persistent storage
    count_str: string = persistent["user_count"]
    // In a real system, would convert string to int, increment, convert back
    persistent["user_count"] = "incremented"

    // Log to memory (temporary data)
    memory[0] = "last_action_register"
    memory[1] = username

    return true
}

define get_user_info(username: string) : string {
    user_key: string = "user_" + username
    email_key: string = user_key + "_email"

    if len(persistent[email_key]) > 0 {
        return persistent[email_key]
    }

    return "not_found"
}

define system_status() : string {
    // Check memory state
    memory_status: string = memory[0]

    // Check persistent state
    system_status: string = persistent["status"]
    version: string = persistent["version"]

    // Combine status info
    status_info: string = system_status + "_" + version + "_" + memory_status

    return status_info
}

define cleanup_memory() : int {
    // Clear temporary memory
    memory[0] = "cleared"
    memory[1] = ""
    memory[10] = ""

    // Persistent data remains intact
    persistent["last_cleanup"] = "completed"

    return len(memory)
}

define data_migration(old_key: string, new_key: string) : bool {
    // Move data within persistent storage
    old_value: string = persistent[old_key]

    if len(old_value) == 0 {
        return false  // Old key doesn't exist
    }

    // Copy to new location
    persistent[new_key] = old_value

    // Clear old location (set to empty string)
    persistent[old_key] = ""

    // Log the migration in memory
    memory[0] = "migration_completed"
    memory[1] = old_key + "_to_" + new_key

    return true
}

define storage_stats() : string {
    // Calculate some basic stats
    memory_size: int = len(memory)

    // Check if key persistent keys exist
    has_status: bool = len(persistent["status"]) > 0
    has_version: bool = len(persistent["version"]) > 0

    // Simple status summary
    if has_status && has_version {
        return "healthy"
    }

    return "incomplete"
}