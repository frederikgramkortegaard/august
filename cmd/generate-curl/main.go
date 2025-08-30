package main

import (
	"encoding/json"
	"fmt"
	"gocuria/blockchain"
	"gocuria/testing"
	"log"
	"os"
	"path/filepath"
)

func main() {
	fmt.Println("Generating curl test scripts with real blockchain data...")

	// Generate a small test chain (3 blocks, 2 accounts, 5 transactions per block)
	blocks := testing.GeneratePrebuiltChain(3, 2, 5)
	
	// Create curl directory if it doesn't exist
	if err := os.MkdirAll("curl", 0755); err != nil {
		log.Fatal("Failed to create curl directory:", err)
	}

	// Generate scripts for each block (skip genesis block at index 0)
	for i, block := range blocks[1:] {
		blockNum := i + 1
		
		// Convert block to pretty JSON
		jsonData, err := json.MarshalIndent(block, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal block %d: %v", blockNum, err)
			continue
		}

		// Create the curl script content
		scriptContent := fmt.Sprintf(`#!/bin/bash
echo "=== Testing POST /api/blocks - Block %d ==="
echo "Block hash: %x"
echo ""

curl -X POST http://localhost:8372/api/blocks \
  -H "Content-Type: application/json" \
  -d '%s' \
  --max-time 2 \
  --connect-timeout 2 \
  --fail-with-body \
  | jq '.' 2>/dev/null || cat
echo -e "\n"
`, blockNum, blockchain.HashBlockHeader(&block.Header), jsonData)

		// Write the script file
		filename := fmt.Sprintf("curl/post_block_%d.sh", blockNum)
		if err := writeScript(filename, scriptContent); err != nil {
			log.Printf("Failed to write script %s: %v", filename, err)
			continue
		}
		
		fmt.Printf("Generated: %s\n", filename)
	}

	// Generate a script that posts all blocks in sequence
	sequentialScript := `#!/bin/bash
echo "=== Testing Sequential Block Submission ==="
echo "Make sure your node is running first!"
echo ""

# Check if server is running
if ! curl -s --connect-timeout 2 --max-time 2 http://localhost:8372/api/chain/height > /dev/null; then
    echo "Server not responding on port 8372"
    echo "Start your node with: go run cmd/node/main.go"
    exit 1
fi

echo "Server is running. Submitting blocks sequentially..."
echo ""

`
	for i := 1; i <= len(blocks)-1; i++ {
		sequentialScript += fmt.Sprintf("echo \"Submitting block %d...\"\n./curl/post_block_%d.sh || echo \"Block %d failed, continuing...\"\nsleep 1\necho \"\"\n\n", i, i, i)
	}

	sequentialScript += `echo "Sequential block submission completed!"
echo "Check chain height:"
curl -s --connect-timeout 2 --max-time 2 http://localhost:8372/api/chain/height | jq '.' 2>/dev/null || cat
echo ""
`

	if err := writeScript("curl/post_all_blocks.sh", sequentialScript); err != nil {
		log.Fatal("Failed to write sequential script:", err)
	}
	fmt.Println("Generated: curl/post_all_blocks.sh")

	fmt.Printf("\nGenerated %d test scripts successfully!\n", len(blocks))
	fmt.Println("Usage:")
	fmt.Println("  1. Start your node: go run cmd/node/main.go")
	fmt.Println("  2. Run individual tests: ./curl/post_block_1.sh")
	fmt.Println("  3. Run all sequentially: ./curl/post_all_blocks.sh")
}

func writeScript(filename, content string) error {
	// Ensure the directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write the file
	if err := os.WriteFile(filename, []byte(content), 0755); err != nil {
		return err
	}

	return nil
}