package marigold

// GetBlockchainContextVariable returns the definition for a blockchain context variable
// or nil if the variable name is not a valid blockchain context variable
func GetBlockchainContextVariable(name string) *Variable {
	switch name {
	case "@caller":
		return &Variable{
			Name:  "@caller",
			Value: "",
			Type:  StringType, // Address of message sender
		}
	case "@address":
		return &Variable{
			Name:  "@address",
			Value: "",
			Type:  StringType, // Address of contract currently executing
		}
	case "@balance":
		return &Variable{
			Name:  "@balance",
			Value: "",
			Type:  IntType, // Balance of the current contract
		}
	case "@origin":
		return &Variable{
			Name:  "@origin",
			Value: "",
			Type:  StringType, // Transaction originator address
		}
	case "@gasprice":
		return &Variable{
			Name:  "@gasprice",
			Value: "",
			Type:  IntType, // Price per gas in this transaction
		}
	case "@callvalue":
		return &Variable{
			Name:  "@callvalue",
			Value: "",
			Type:  IntType, // Amount of AUG sent with this transaction
		}
	case "@timestamp":
		return &Variable{
			Name:  "@timestamp",
			Value: "",
			Type:  IntType, // Current block timestamp
		}
	case "@difficulty":
		return &Variable{
			Name:  "@difficulty",
			Value: "",
			Type:  IntType, // Current block difficulty
		}
	case "@coinbase":
		return &Variable{
			Name:  "@coinbase",
			Value: "",
			Type:  StringType, // Current block's beneficiary address
		}
	case "@height":
		return &Variable{
			Name:  "@height",
			Value: "",
			Type:  IntType, // Current block number
		}
	case "@gaslimit":
		return &Variable{
			Name:  "@gaslimit",
			Value: "",
			Type:  IntType, // Current block gas limit
		}
	case "@tsxdata":
		return &Variable{
			Name:  "@tsxdata",
			Value: "",
			Type:  StringType, // Transaction data as hex string
		}
	default:
		return nil // Not a valid blockchain context variable
	}
}
