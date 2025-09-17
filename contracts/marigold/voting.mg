define is_eligible(age: int, is_citizen: bool) : bool {
	age_requirement: bool = age >= 18
	return age_requirement && is_citizen
}

define vote_count(yes_votes: int, no_votes: int) : int {
	return yes_votes + no_votes
}

define is_majority(yes_votes: int, total_votes: int) : bool {
	if total_votes == 0 {
		return false
	}

	majority_threshold: int = total_votes / 2
	return yes_votes > majority_threshold
}

define process_vote(voter_age: int, is_citizen: bool, vote_yes: bool) : string {
	if is_eligible(voter_age, is_citizen) {
		if vote_yes {
			return "YES"
		} else {
			return "NO"
		}
	} else {
		return "INELIGIBLE"
	}
}