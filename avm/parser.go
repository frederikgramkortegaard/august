package avm

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

func ParseInstructionsFromString(text string) ([]Instruction, error) {
	// Match: non-comma/non-newline content, or commas
	re := regexp.MustCompile(`[^,\n\s]+|,`)
	matches := re.FindAllString(text, -1)

	var insts []Instruction

	curtok := 0
	for curtok < len(matches) {
		if matches[curtok] == "PUSH" {
			if curtok+1 == len(matches) {
				return insts, errors.New("invalid eof")
			}

			hexValue := matches[curtok+1]

			// Validate using big.Int which handles hex properly and checks for 256-bit limit
			testValue := new(big.Int)
			if hexValue[:2] == "0x" {
				_, ok := testValue.SetString(hexValue[2:], 16)
				if !ok {
					return insts, errors.New("invalid hex value: " + hexValue)
				}
			} else {
				_, ok := testValue.SetString(hexValue, 16)
				if !ok {
					return insts, errors.New("invalid hex value: " + hexValue)
				}
				hexValue = "0x" + hexValue
			}

			// Check 256-bit limit
			if testValue.BitLen() > 256 {
				return insts, errors.New("hex value exceeds 256 bits: " + hexValue)
			}

			insts = append(insts, MakePushInstruction(hexValue))
			curtok += 2 // Skip both PUSH and its value
		} else {
			// Parse opcode from string using map
			opcode, exists := opcodeMap[matches[curtok]]
			if !exists {
				return insts, fmt.Errorf("unknown opcode: %s", matches[curtok])
			}

			insts = append(insts, MakeInstruction(opcode))
			curtok++
		}
	}

	return insts, nil
}

// ExportInstructionsAsString converts instructions back to string format
func ExportInstructionsAsString(instructions []Instruction) string {
	var parts []string

	for _, inst := range instructions {
		if inst.Opcode == PUSH && inst.Value != nil {
			parts = append(parts, "PUSH", *inst.Value)
		} else {
			parts = append(parts, inst.Opcode.String())
		}
	}

	return strings.Join(parts, " ")
}
