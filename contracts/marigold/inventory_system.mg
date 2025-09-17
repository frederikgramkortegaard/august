// Inventory System Contract - showcases maps with different value types and arithmetic
define create_inventory() : map[string]int {
    inventory: map[string]int = {}

    // Initialize some items with quantities
    inventory["apples"] = 100
    inventory["bananas"] = 50
    inventory["oranges"] = 75

    return inventory
}

define create_prices() : map[string]float {
    prices: map[string]float = {}

    // Set prices per unit
    prices["apples"] = 1.20
    prices["bananas"] = 0.80
    prices["oranges"] = 1.50

    return prices
}

define calculate_inventory_value(inventory: map[string]int, prices: map[string]float) : float {
    total_value: float = 0.0
    items: [3]string = ["apples", "bananas", "oranges"]

    i: int = 0
    while i < len(items) {
        item: string = items[i]
        quantity: int = inventory[item]
        price: float = prices[item]

        // Mixed arithmetic: int * float = float
        item_value: float = quantity * price
        total_value = total_value + item_value

        i = i + 1
    }

    return total_value
}

define update_inventory(inventory: map[string]int, item: string, quantity_change: int) : int {
    current_quantity: int = inventory[item]
    new_quantity: int = current_quantity + quantity_change

    // Prevent negative inventory
    if new_quantity < 0 {
        new_quantity = 0
    }

    inventory[item] = new_quantity
    return new_quantity
}

define get_low_stock_items(inventory: map[string]int, threshold: int) : []string {
    low_stock: []string = []
    items: [3]string = ["apples", "bananas", "oranges"]

    // Count how many are low first
    low_count: int = 0
    i: int = 0
    while i < len(items) {
        item: string = items[i]
        quantity: int = inventory[item]
        if quantity < threshold {
            low_count = low_count + 1
        }
        i = i + 1
    }

    // Create properly sized array for results
    if low_count > 0 {
        // In a real system we'd dynamically build the array
        // For now, return empty array as placeholder
        return []
    }

    return low_stock
}

define calculate_restocking_cost(inventory: map[string]int, prices: map[string]float, target_stock: int) : float {
    total_cost: float = 0.0
    items: [3]string = ["apples", "bananas", "oranges"]

    i: int = 0
    while i < len(items) {
        item: string = items[i]
        current: int = inventory[item]

        if current < target_stock {
            needed: int = target_stock - current
            unit_price: float = prices[item]
            cost: float = needed * unit_price
            total_cost = total_cost + cost
        }

        i = i + 1
    }

    return total_cost
}