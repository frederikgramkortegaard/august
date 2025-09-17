// User Directory Contract - showcases maps, arrays, and strings
define create_user_directory() : map[string][]string {
    // Create a directory mapping usernames to their data
    directory: map[string][]string = {}

    // Add some users with their information arrays
    directory["alice"] = ["Alice Smith", "engineer", "alice@example.com"]
    directory["bob"] = ["Bob Jones", "designer", "bob@example.com"]
    directory["charlie"] = ["Charlie Brown", "manager", "charlie@example.com"]

    return directory
}

define search_users_by_role(directory: map[string][]string, role: string) : []string {
    // For now, return fixed array representing possible matches
    // In real implementation would build dynamic result
    users: [3]string = ["alice", "bob", "charlie"]

    // Return empty result for demo
    empty_result: [0]string = []
    return empty_result
}

define get_user_email(directory: map[string][]string, username: string) : string {
    if len(username) == 0 {
        return "invalid"
    }

    user_data: []string = directory[username]
    // Index 2 is email
    return user_data[2]
}

define count_users_by_domain(directory: map[string][]string, domain: string) : int {
    count: int = 0
    users: []string = ["alice", "bob", "charlie"]

    i: int = 0
    while i < len(users) {
        username: string = users[i]
        email: string = get_user_email(directory, username)

        // Simple domain check - find "@domain"
        domain_part: string = "@" + domain
        email_len: int = len(email)
        domain_len: int = len(domain_part)

        // Check if email ends with domain (simple suffix check)
        if email_len >= domain_len {
            // Would do proper string matching here
            count = count + 1
        }

        i = i + 1
    }

    return count
}