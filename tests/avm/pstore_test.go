package avm

import (
	"august/avm"
	"testing"
)

func TestPSTOREBasic(t *testing.T) {
	// Test PSTORE instruction directly

	instructions := []avm.Instruction{
		avm.MakePushInstruction("0x3E7"), // Push value 999
		avm.MakePushInstruction("0x7"),   // Push address 7
		avm.MakeInstruction(avm.PSTORE),  // Store 999 at address 7
		avm.MakeInstruction(avm.STOP),
	}

	runtime := avm.NewTestRuntime(1010, instructions)
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

	instructions := []avm.Instruction{
		avm.MakePushInstruction("0x3"), // Push address 3
		avm.MakeInstruction(avm.PLOAD), // Load from address 3 (should be 0)
		avm.MakeInstruction(avm.EMIT),  // Emit the loaded value
		avm.MakeInstruction(avm.STOP),
	}

	runtime := avm.NewTestRuntime(1110, instructions)

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

	instructions := []avm.Instruction{
		avm.MakePushInstruction("0x309"), // Push value 777
		avm.MakePushInstruction("0x9"),   // Push address 9
		avm.MakeInstruction(avm.PSTORE),  // Store 777 at address 9
		avm.MakePushInstruction("0x9"),   // Push address 9
		avm.MakeInstruction(avm.PLOAD),   // Load from address 9
		avm.MakeInstruction(avm.EMIT),    // Emit the loaded value (should be 777)
		avm.MakeInstruction(avm.STOP),
	}

	runtime := avm.NewTestRuntime(2110, instructions)
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