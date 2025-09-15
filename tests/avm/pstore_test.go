package avm

import (
	"august/avm"
	"math/big"
	"testing"
)

func TestPSTOREBasic(t *testing.T) {
	// Test PSTORE instruction directly
	config := avm.RuntimeConfig{
		StackSize:  1024,
		MemorySize: 65536,
	}

	instructions := []avm.Instruction{
		{Opcode: avm.PUSH, Value: big.NewInt(999)}, // Push value 999
		{Opcode: avm.PUSH, Value: big.NewInt(7)},   // Push address 7
		{Opcode: avm.PSTORE},                       // Store 999 at address 7
		{Opcode: avm.STOP},
	}

	runtime := avm.NewRuntime(1010, instructions, config)
	gasUsed, err := runtime.StartExecution()

	if err != avm.ErrProgramStopped {
		t.Errorf("Expected program stopped, got %v", err)
	}

	t.Logf("Gas used: %d", gasUsed)
	t.Logf("Persistent storage entries: %d", len(runtime.Persistent))

	// Check if value was stored
	if len(runtime.Persistent) != 1 {
		t.Errorf("Expected 1 persistent storage entry, got %d", len(runtime.Persistent))
	}

	value, exists := runtime.Persistent["7"]
	if !exists {
		t.Errorf("Expected value at address 7")
	} else if value != "999" {
		t.Errorf("Expected value 999, got %s", value)
	} else {
		t.Logf("PSTORE working: address 7 = %s", value)
	}
}

func TestPLOADBasic(t *testing.T) {
	// Test PLOAD instruction directly
	config := avm.RuntimeConfig{
		StackSize:  1024,
		MemorySize: 65536,
	}

	instructions := []avm.Instruction{
		{Opcode: avm.PUSH, Value: big.NewInt(3)}, // Push address 3
		{Opcode: avm.PLOAD},                      // Load from address 3 (should be 0)
		{Opcode: avm.EMIT},                       // Emit the loaded value
		{Opcode: avm.STOP},
	}

	runtime := avm.NewRuntime(1110, instructions, config)

	// Pre-populate persistent storage
	runtime.Persistent["3"] = "555"

	gasUsed, err := runtime.StartExecution()

	if err != avm.ErrProgramStopped {
		t.Errorf("Expected program stopped, got %v", err)
	}

	t.Logf("Gas used: %d", gasUsed)
	t.Logf("Should have emitted 555 (0x22B)")
}

func TestPSTOREAndPLOAD(t *testing.T) {
	// Test both PSTORE and PLOAD together
	config := avm.RuntimeConfig{
		StackSize:  1024,
		MemorySize: 65536,
	}

	instructions := []avm.Instruction{
		{Opcode: avm.PUSH, Value: big.NewInt(777)}, // Push value 777
		{Opcode: avm.PUSH, Value: big.NewInt(9)},   // Push address 9
		{Opcode: avm.PSTORE},                       // Store 777 at address 9
		{Opcode: avm.PUSH, Value: big.NewInt(9)},   // Push address 9
		{Opcode: avm.PLOAD},                        // Load from address 9
		{Opcode: avm.EMIT},                         // Emit the loaded value (should be 777)
		{Opcode: avm.STOP},
	}

	runtime := avm.NewRuntime(2110, instructions, config)
	gasUsed, err := runtime.StartExecution()

	if err != avm.ErrProgramStopped {
		t.Errorf("Expected program stopped, got %v", err)
	}

	t.Logf("Gas used: %d", gasUsed)
	t.Logf("Persistent storage entries: %d", len(runtime.Persistent))

	// Check persistent storage
	value, exists := runtime.Persistent["9"]
	if !exists {
		t.Errorf("Expected value at address 9")
	} else if value != "777" {
		t.Errorf("Expected value 777, got %s", value)
	} else {
		t.Logf("PSTORE + PLOAD working: address 9 = %s", value)
		t.Logf("Should have emitted 777 (0x309)")
	}
}