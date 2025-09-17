define max(a: int, b: int) : int {
	if a > b {
		return a
	} else {
		return b
	}
}

define min(a: int, b: int) : int {
	if a < b {
		return a
	} else {
		return b
	}
}

define is_in_range(value: int, min_val: int, max_val: int) : bool {
	return value >= min_val && value <= max_val
}

define calculate_damage(base_damage: int, critical_hit: bool, armor: int) : int {
	damage: int = base_damage

	if critical_hit {
		damage = damage * 2
	}

	final_damage: int = damage - armor
	return max(final_damage, 0)
}

define level_up_check(current_xp: int, required_xp: int) : bool {
	return current_xp >= required_xp
}