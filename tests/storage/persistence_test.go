package tests

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

var tempDir string // Shared temp directory for all tests

// TestMain runs before all tests and sets up temporary directories
func TestMain(m *testing.M) {
	// Create a temporary directory for all tests
	var err error
	tempDir, err = os.MkdirTemp("", "august-storage-tests-*")
	if err != nil {
		panic("Failed to create temp directory: " + err.Error())
	}

	// Run all tests
	code := m.Run()

	// Cleanup: Remove temporary directory
	os.RemoveAll(tempDir)

	// Exit with the same code as the tests
	os.Exit(code)
}

const PORT_START_RANGE = 9370

func createTestNode(idx int) *node.FullNode {

	portAsString := strconv.Itoa(PORT_START_RANGE + idx)

	conf := node.Config{
		Port:      portAsString,
		NodeID:    "test-node-id-" + portAsString,
		SeedPeers: []string{},
		DBName:    filepath.Join(tempDir),
	}

	return &node.NewFullNode(conf)
}
