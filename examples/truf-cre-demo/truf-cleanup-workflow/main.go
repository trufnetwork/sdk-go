//go:build wasip1

package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/cre/wasm"
	kwilCrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/util"
)

// Config struct loaded from config.staging.json or config.production.json
type Config struct {
	Schedule     string `json:"schedule"`     // Cron schedule
	TRUFEndpoint string `json:"trufEndpoint"` // TRUF Gateway URL
	PrivateKey   string `json:"privateKey"`   // Ethereum private key (hex)
}

// ExecutionResult contains the workflow execution results
type ExecutionResult struct {
	StreamID     string `json:"streamId" consensus_aggregation:"identical"`
	DataProvider string `json:"dataProvider" consensus_aggregation:"identical"`
	Deleted      bool   `json:"deleted" consensus_aggregation:"identical"`
	Success      bool   `json:"success" consensus_aggregation:"identical"`
	Error        string `json:"error,omitempty" consensus_aggregation:"identical"`
}

// InitWorkflow is the required entry point for a CRE workflow
func InitWorkflow(config *Config, logger *slog.Logger, secretsProvider cre.SecretsProvider) (cre.Workflow[*Config], error) {
	// Create the cron trigger from config
	cronTrigger := cron.Trigger(&cron.Config{Schedule: config.Schedule})

	// Register handler with the trigger
	return cre.Workflow[*Config]{
		cre.Handler(cronTrigger, onCronTrigger),
	}, nil
}

// onCronTrigger executes when the cron trigger fires
// Demonstrates cleanup: Delete Stream
func onCronTrigger(config *Config, runtime cre.Runtime, trigger *cron.Payload) (*ExecutionResult, error) {
	logger := runtime.Logger()
	logger.Info("=== TRUF CRE Cleanup Workflow: Delete Stream Demo ===")

	// Run in NodeMode to access HTTP capabilities
	return cre.RunInNodeMode(config, runtime,
		func(config *Config, nodeRuntime cre.NodeRuntime) (*ExecutionResult, error) {
			result := &ExecutionResult{}

			// Stream configuration (must match write workflow)
			const streamNameStr = "stcreteststream00000000000000000"
			result.StreamID = streamNameStr

			// Create StreamId from string
			streamId, err := util.NewStreamId(streamNameStr)
			if err != nil {
				logger.Error("Invalid stream ID", "error", err)
				result.Error = fmt.Sprintf("Invalid stream ID: %v", err)
				return result, err
			}

			// Create signer from private key
			signer, signerAddr, err := createSigner(config.PrivateKey)
			if err != nil {
				logger.Error("Signer creation failed", "error", err)
				result.Error = fmt.Sprintf("Failed to create signer: %v", err)
				return result, err
			}

			// Data provider is the signer's address
			result.DataProvider = signerAddr
			logger.Info("Signer created", "address", signerAddr, "role", "data provider")

			// Create TRUF client with CRE transport
			client, err := tnclient.NewClient(context.Background(), config.TRUFEndpoint,
				tnclient.WithCRETransportAndSigner(nodeRuntime, config.TRUFEndpoint, signer),
			)
			if err != nil {
				logger.Error("Client creation failed", "error", err)
				result.Error = fmt.Sprintf("Failed to create client: %v", err)
				return result, err
			}

			logger.Info("TRUF client created successfully with CRE transport")

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			// ========================================================
			// STEP 1: DELETE STREAM (cleanup)
			// ========================================================
			logger.Info("=== Step 1: Deleting Stream (cleanup) ===")

			deleteTxHash, delErr := client.DestroyStream(ctx, *streamId)
			if delErr != nil {
				logger.Error("Stream deletion failed", "error", delErr)
				result.Error = fmt.Sprintf("Delete failed: %v", delErr)
				return result, nil
			}

			result.Deleted = true
			result.Success = true
			logger.Info("✅ Stream deletion transaction submitted", "txHash", deleteTxHash.String(), "streamId", streamNameStr)

			logger.Info("=== Cleanup Workflow Completed Successfully ===")
			return result, nil
		},
		cre.ConsensusAggregationFromTags[*ExecutionResult](),
	).Await()
}

// createSigner creates an Ethereum signer from a hex-encoded private key
func createSigner(privateKeyHex string) (auth.Signer, string, error) {
	// Remove 0x prefix if present
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")

	// Validate length
	if len(privateKeyHex) != 64 {
		return nil, "", fmt.Errorf("invalid private key length: expected 64 hex chars, got %d", len(privateKeyHex))
	}

	// Convert hex to bytes first for address derivation
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode hex: %w", err)
	}

	// Get address for logging using go-ethereum
	ecdsaKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to convert key: %w", err)
	}
	publicKey := ecdsaKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, "", fmt.Errorf("failed to cast public key")
	}
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	// Parse private key using kwil-db crypto (for signing)
	privateKey, err := kwilCrypto.Secp256k1PrivateKeyFromHex(privateKeyHex)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create Ethereum personal signer with dereferenced key
	signer := &auth.EthPersonalSigner{Key: *privateKey}

	return signer, address, nil
}

func main() {
	wasm.NewRunner(cre.ParseJSON[Config]).Run(InitWorkflow)
}
