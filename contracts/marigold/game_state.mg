// Game State Contract - showcases complex data structures and state management
define create_player() : map[string]int {
    player: map[string]int = {}

    // Player stats
    player["health"] = 100
    player["mana"] = 50
    player["level"] = 1
    player["experience"] = 0
    player["gold"] = 100

    // Player position
    player["x"] = 0
    player["y"] = 0

    return player
}

define create_inventory_slots() : [10]string {
    inventory: [10]string

    // Empty inventory (would be item IDs in real game)
    i: int = 0
    while i < 10 {
        inventory[i] = "empty"
        i = i + 1
    }

    return inventory
}

define calculate_damage(attacker_level: int, base_damage: int, weapon_bonus: int) : int {
    // Damage formula: base + (level * 2) + weapon
    level_bonus: int = attacker_level * 2
    total_damage: int = base_damage + level_bonus + weapon_bonus

    // Min damage is 1
    if total_damage < 1 {
        total_damage = 1
    }

    return total_damage
}

define apply_damage(player: map[string]int, damage: int) : bool {
    current_health: int = player["health"]
    new_health: int = current_health - damage

    if new_health <= 0 {
        new_health = 0
        player["health"] = new_health
        return true  // Player died
    }

    player["health"] = new_health
    return false  // Player survived
}

define gain_experience(player: map[string]int, exp_amount: int) : bool {
    current_exp: int = player["experience"]
    current_level: int = player["level"]

    new_exp: int = current_exp + exp_amount
    player["experience"] = new_exp

    // Check for level up (need 100 * level exp for next level)
    exp_needed: int = current_level * 100

    if new_exp >= exp_needed {
        // Level up!
        player["level"] = current_level + 1
        player["experience"] = new_exp - exp_needed

        // Increase stats on level up
        player["health"] = player["health"] + 10
        player["mana"] = player["mana"] + 5

        return true  // Leveled up
    }

    return false  // No level up
}

define move_player(player: map[string]int, dx: int, dy: int, map_width: int, map_height: int) : bool {
    current_x: int = player["x"]
    current_y: int = player["y"]

    new_x: int = current_x + dx
    new_y: int = current_y + dy

    // Boundary checking
    if new_x < 0 || new_x >= map_width {
        return false  // Invalid move
    }

    if new_y < 0 || new_y >= map_height {
        return false  // Invalid move
    }

    // Valid move
    player["x"] = new_x
    player["y"] = new_y
    return true
}

define calculate_distance_to_target(player: map[string]int, target_x: int, target_y: int) : int {
    player_x: int = player["x"]
    player_y: int = player["y"]

    // Manhattan distance (simpler than Euclidean)
    dx: int = target_x - player_x
    dy: int = target_y - player_y

    // Absolute values (simplified)
    if dx < 0 {
        dx = 0 - dx
    }
    if dy < 0 {
        dy = 0 - dy
    }

    distance: int = dx + dy
    return distance
}

define simulate_battle(player1: map[string]int, player2: map[string]int) : string {
    // Simple turn-based battle simulation
    p1_health: int = player1["health"]
    p2_health: int = player2["health"]

    p1_level: int = player1["level"]
    p2_level: int = player2["level"]

    // Calculate damage for each player
    p1_damage: int = calculate_damage(p1_level, 10, 0)
    p2_damage: int = calculate_damage(p2_level, 10, 0)

    // Simple battle: whoever has higher effective power wins
    p1_power: int = p1_health + p1_damage
    p2_power: int = p2_health + p2_damage

    if p1_power > p2_power {
        return "player1"
    } else if p2_power > p1_power {
        return "player2"
    } else {
        return "tie"
    }
}

define create_game_state() : map[string]map[string]int {
    game_state: map[string]map[string]int = {}

    // Create two players
    player1: map[string]int = create_player()
    player2: map[string]int = create_player()

    // Set different starting positions
    player2["x"] = 10
    player2["y"] = 10

    // In full implementation, we'd store players in game_state
    // game_state["player1"] = player1
    // game_state["player2"] = player2

    return game_state
}