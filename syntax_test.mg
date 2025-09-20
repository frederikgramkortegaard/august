// Marigold syntax highlighting test file
define init() : int {
  // Initialize contract with deployer balance
  persistent[@caller] = string(@callvalue)
  return 0
}

define transfer(recipient: string, amount: int) : bool {
  // Check sufficient balance (defaults to 0 if not exists)
  if int(persistent[@caller]) < amount {
    return false
  }

  // String slicing example
  addr_prefix : string = recipient[0:8]
  addr_suffix : string = recipient[56:]

  // Perform transfer
  persistent[@caller] = string(int(persistent[@caller]) - amount)
  persistent[recipient] = string(int(persistent[recipient]) + amount)

  // Emit transfer event
  emit(amount)
  return true
}

define call() : int {
  // Parse transaction data
  if len(@tsxdata) >= 73 && @tsxdata[:8] == "transfer" {
    recipient : string = @tsxdata[8:72]
    amount_str : string = @tsxdata[72:]
    amount : int = int(amount_str)

    // Execute transfer
    success : bool = transfer(recipient, amount)
    if success {
      return 0
    } else {
      return 1
    }
  }

  // Buy tokens
  if @tsxdata[:3] == "buy" && len(@tsxdata) == 3 {
    persistent[@caller] = string(int(persistent[@caller]) + @callvalue)
    return 0
  }

  return 1
}