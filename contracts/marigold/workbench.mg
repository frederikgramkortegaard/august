
define init() : int {
  // Contract deployer buys @callvalue tokens
  persistent[@caller] = string(@callvalue)
  return 0
}

define call() : int {

  // Really need better string operations
  if @tsxdata[:3] == "buy" && len(@tsxdata) == 3 { 
    // Simply add to balance (defaults to 0 if not exists)
    persistent[@caller] = string(int(persistent[@caller]) + @callvalue)
    return 0
  }

  // Transfer money to other people
  // @NOTE : Minimum data length for a valid transfer (transfer + 64 char hex address + minimum 1 digit for amount = 73) 
  if len(@tsxdata) >= 73  && @tsxdata[:8] == "transfer" {

    // Extract recipient address (64 chars after "transfer")
    recipient : string = @tsxdata[8:72]

    // Extract amount (everything after position 72)
    amount_str : string = @tsxdata[72:]
    amount : int = int(amount_str)
  
    // Check if sender has sufficient funds (defaults to 0 if not exists)
    if int(persistent[@caller]) < amount {
      return 1
    }

    // Deduct from sender
    persistent[@caller] = string(int(persistent[@caller]) - amount)

    // Add to recipient (defaults to 0 if not exists)
    persistent[recipient] = string(int(persistent[recipient]) + amount)

  }

  return 0
}


