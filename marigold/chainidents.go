package marigold

// PrePopulateBlockchainContext adds blockchain context variables to global scope
// these are cached during runtime so concecutive calls will only charge gas once. Furthermore, if e.g. @address is never called, no gas will be charged
func PrePopulateBlockchainContext(globalScope *Scope) {
	// Add blockchain context variables with @ prefix
	globalScope.Variables["@caller"] = &Variable{
		Name:  "@caller",
		Value: "",
		Type:  StringType, // Address of message sender
	}

	globalScope.Variables["@address"] = &Variable{
		Name:  "@address",
		Value: "",
		Type:  StringType, // Address of contract currently executing
	}

	globalScope.Variables["@balance"] = &Variable{
		Name:  "@balance",
		Value: "",
		Type:  IntType, // Balance of the current contract
	}

	globalScope.Variables["@origin"] = &Variable{
		Name:  "@origin",
		Value: "",
		Type:  StringType, // Transaction originator address
	}

	globalScope.Variables["@gasprice"] = &Variable{
		Name:  "@gasprice",
		Value: "",
		Type:  IntType, // Price per gas in this transaction
	}

	globalScope.Variables["@callvalue"] = &Variable{
		Name:  "@callvalue",
		Value: "",
		Type:  IntType, // Amount of AUG sent with this transaction
	}

	globalScope.Variables["@timestamp"] = &Variable{
		Name:  "@timestamp",
		Value: "",
		Type:  IntType, // Current block timestamp
	}

	globalScope.Variables["@difficulty"] = &Variable{
		Name:  "@difficulty",
		Value: "",
		Type:  IntType, // Current block difficulty
	}

	globalScope.Variables["@coinbase"] = &Variable{
		Name:  "@coinbase",
		Value: "",
		Type:  StringType, // Current block's beneficiary address
	}

	globalScope.Variables["@height"] = &Variable{
		Name:  "@height",
		Value: "",
		Type:  IntType, // Current block number
	}

	globalScope.Variables["@gaslimit"] = &Variable{
		Name:  "@gaslimit",
		Value: "",
		Type:  IntType, // Current block gas limit
	}
}
