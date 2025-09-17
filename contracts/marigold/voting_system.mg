// Voting System Contract - practical example using maps and arrays
define create_election() : map[string]int {
    // Initialize vote counts for candidates
    votes: map[string]int = {}
    votes["alice"] = 0
    votes["bob"] = 0
    votes["charlie"] = 0

    return votes
}

define cast_vote(votes: map[string]int, candidate: string) : bool {
    // Simple validation - check if candidate exists
    candidates: [3]string = ["alice", "bob", "charlie"]

    // Find candidate in list
    found: bool = false
    i: int = 0
    while i < len(candidates) {
        if candidates[i] == candidate {
            found = true
        }
        i = i + 1
    }

    if found {
        // Increment vote count
        current_votes: int = votes[candidate]
        votes[candidate] = current_votes + 1
        return true
    }

    return false  // Invalid candidate
}

define get_winner(votes: map[string]int) : string {
    candidates: [3]string = ["alice", "bob", "charlie"]

    winner: string = candidates[0]
    max_votes: int = votes[winner]

    i: int = 1
    while i < len(candidates) {
        candidate: string = candidates[i]
        candidate_votes: int = votes[candidate]

        if candidate_votes > max_votes {
            winner = candidate
            max_votes = candidate_votes
        }

        i = i + 1
    }

    return winner
}

define get_total_votes(votes: map[string]int) : int {
    candidates: [3]string = ["alice", "bob", "charlie"]
    total: int = 0

    i: int = 0
    while i < len(candidates) {
        candidate: string = candidates[i]
        candidate_votes: int = votes[candidate]
        total = total + candidate_votes
        i = i + 1
    }

    return total
}

define calculate_percentages(votes: map[string]int) : map[string]float {
    percentages: map[string]float = {}
    candidates: [3]string = ["alice", "bob", "charlie"]

    total_votes: int = get_total_votes(votes)

    if total_votes == 0 {
        // No votes cast yet
        percentages["alice"] = 0.0
        percentages["bob"] = 0.0
        percentages["charlie"] = 0.0
        return percentages
    }

    total_float: float = total_votes  // Convert to float for division

    i: int = 0
    while i < len(candidates) {
        candidate: string = candidates[i]
        candidate_votes: int = votes[candidate]
        candidate_float: float = candidate_votes

        // Calculate percentage (votes/total * 100)
        percentage: float = candidate_float / total_float * 100.0
        percentages[candidate] = percentage

        i = i + 1
    }

    return percentages
}

define is_tied(votes: map[string]int) : bool {
    candidates: [3]string = ["alice", "bob", "charlie"]

    first_count: int = votes[candidates[0]]

    // Check if all candidates have same vote count
    i: int = 1
    while i < len(candidates) {
        candidate_count: int = votes[candidates[i]]
        if candidate_count != first_count {
            return false  // Not tied
        }
        i = i + 1
    }

    return true  // All have same count
}

define run_election_simulation() : string {
    // Create new election
    votes: map[string]int = create_election()

    // Simulate some votes
    cast_vote(votes, "alice")
    cast_vote(votes, "alice")
    cast_vote(votes, "bob")
    cast_vote(votes, "charlie")
    cast_vote(votes, "alice")
    cast_vote(votes, "bob")

    // Get results
    total: int = get_total_votes(votes)
    winner: string = get_winner(votes)
    tied: bool = is_tied(votes)

    if tied {
        return "tie"
    }

    return winner
}