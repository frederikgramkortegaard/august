package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
)

func main() {

	// Generate a simple mining key (in real life, this would be persistent)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal("Error", err)
	}
	// Generate a simple mining key (in real life, this would be persistent)

	// Print keys as hex
	fmt.Printf("Private key (64 bytes): %s\n", hex.EncodeToString(privateKey))
	fmt.Printf("Public key  (32 bytes): %s\n", hex.EncodeToString(publicKey))

}
