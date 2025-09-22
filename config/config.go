package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"
)

// Difficulty adjustment constants
const (
	RecalculationFrequency = 2016
	TargetTimespan         = 1209600 // 2 weeks in seconds
	TargetSpacing          = 600     // 10 minutes in seconds
	MaxAdjustmentFactor    = 4       // Maximum 4x adjustment in either direction
)

// Target difficulty constants
const (
	// Bitcoin's max target (difficulty 1) - easiest possible difficulty
	MaxTargetCompact uint32 = 0x1d00ffff

	// Super easy target for testing - requires only 8 leading zero bits
	TestTargetCompact uint32 = 0x1f00ffff
)

// Currency multipliers (like time package)
const (
	Leaf = uint64(1)                // Base unit
	AUG  = uint64(1_000_000) * Leaf // 1 AUG = 1M leaf units
)

// Mining and blockchain constants
const (
	BlockReward           = 50 * AUG   // 50 AUG block reward
	HalvingInterval       = 210000     // Blocks between reward halvings
	MaxAmount             = ^uint64(0) // Maximum uint64 value for overflow checks
	MainnetChainID        = uint64(1)  // Main network chain ID
	TestnetChainID        = uint64(2)  // Test network chain ID
	ContractDeploymentFee = 1 * AUG    // 1 AUG deployment fee

	// Gas constants
	GasTransfer          = uint64(21000)   // Base gas for regular transfer
	GasContractDeploy    = uint64(21000)   // Base gas for contract deployment (before init execution)
	GasContractInitLimit = uint64(1000000) // Max gas for contract init execution
	GasDataPerByte       = uint64(68)      // Gas cost per byte of transaction data

	// AVM runtime limits
	AVMMaxStackSize  = 1024 // Maximum stack depth (0 = unlimited)
	AVMMaxMemorySize = 1024 // Maximum memory slots (0 = unlimited)

	// Network request-response constants
	RequestTimeout     = 10 * time.Second // How long to wait for responses
	MaxPendingRequests = 100              // Maximum number of pending requests

	// Network peer constants
	MaxPeers = 128 // Maximum number of peers

	// Chain management constants
	MaxCandidateChainDepthGap = 6 // Remove candidate chains more than 6 blocks behind current

	// Network TTL constants
	RecentHeadersTTL      = 5 * time.Minute  // How long to keep headers in recent cache
	RecentBlocksTTL       = 5 * time.Minute  // How long to keep blocks in recent cache
	RecentTransactionsTTL = 5 * time.Minute  // How long to keep transactions in recent cache
	CleanupInterval       = 30 * time.Second // How often to cleanup dead peers
	ChainSyncFrequency    = 60 * time.Second // How often to ask peers about their chain information
	DiscoveryInterval     = 30 * time.Second // How often to run peer discovery

	// Mempool constants
	MempoolMaxSize   = 1000               // Maximum number of transactions in mempool
	MempoolMaxExpiry = 7 * 24 * time.Hour // Maximum age before transaction expires (7 days)

	// Blockchain constants
	Difficulty = 1 // Default difficulty

	// Peer request configuration constants (Bitcoin-style defaults)
	MaxRequestsPerPeer    = 16   // Conservative like Bitcoin Core
	MaxPeersForRequest    = 20   // Try up to 20 peers in parallel/sequence
	MinPeersForRequest    = 1    // Can sync from 1 peer (like Bitcoin)
	RequestTimeoutSec     = 30   // 30 second timeout per peer
	RandomizePeerOrder    = true // Whether to randomize peer selection order
	PreferResponsivePeers = true // Prefer peers that have been historically responsive
)

// PublicKey type for blockchain addresses
type PublicKey [ed25519.PublicKeySize]byte // 32

// Custom JSON marshaling for PublicKey
func (pk PublicKey) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.StdEncoding.EncodeToString(pk[:]) + `"`), nil
}

func (pk *PublicKey) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid JSON string for PublicKey: %s", string(data))
	}

	b64String := string(data[1 : len(data)-1])
	decoded, err := base64.StdEncoding.DecodeString(b64String)
	if err != nil {
		return fmt.Errorf("failed to decode base64 '%s': %v", b64String, err)
	}

	if len(decoded) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid PublicKey size for '%s': got %d, want %d", b64String, len(decoded), ed25519.PublicKeySize)
	}

	copy(pk[:], decoded)
	return nil
}

// FirstUser is the genesis account that receives the initial coin supply
// Generated from fixed seed in testing/generator.go (testing only!)
var FirstUser = PublicKey{
	0x99, 0x3f, 0xe6, 0xa3, 0x6d, 0x19, 0xed, 0x52,
	0x78, 0x90, 0xa7, 0x69, 0x8e, 0x94, 0x0c, 0x3c,
	0x1c, 0x62, 0x9c, 0xa0, 0x64, 0x43, 0x83, 0x78,
	0x71, 0xcb, 0x0e, 0xd1, 0x94, 0x35, 0x22, 0x9b,
}
