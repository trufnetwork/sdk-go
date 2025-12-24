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
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// Config struct loaded from config.staging.json or config.production.json
type Config struct {
	Schedule     string `json:"schedule"`     // Cron schedule
	TRUFEndpoint string `json:"trufEndpoint"` // TRUF Gateway URL
	PrivateKey   string `json:"privateKey"`   // Ethereum private key (hex)
}

// RecordData represents a single record for display
type RecordData struct {
	Rank      int    `json:"rank" consensus_aggregation:"identical"`
	EventTime int    `json:"eventTime" consensus_aggregation:"identical"`
	Timestamp string `json:"timestamp" consensus_aggregation:"identical"`
	Value     string `json:"value" consensus_aggregation:"identical"`
}

// ExecutionResult contains the workflow execution results
type ExecutionResult struct {
	StreamID         string       `json:"streamId" consensus_aggregation:"identical"`
	DataProvider     string       `json:"dataProvider" consensus_aggregation:"identical"`
	Deployed         bool         `json:"deployed" consensus_aggregation:"identical"`
	RecordsInserted  int          `json:"recordsInserted" consensus_aggregation:"identical"`
	RecordsRetrieved int          `json:"recordsRetrieved" consensus_aggregation:"identical"`
	Deleted          bool         `json:"deleted" consensus_aggregation:"identical"`
	Success          bool         `json:"success" consensus_aggregation:"identical"`
	Error            string       `json:"error,omitempty" consensus_aggregation:"identical"`
	TopRecords       []RecordData `json:"topRecords,omitempty" consensus_aggregation:"identical"`
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
// Demonstrates write operations: Deploy → Insert
func onCronTrigger(config *Config, runtime cre.Runtime, trigger *cron.Payload) (*ExecutionResult, error) {
	logger := runtime.Logger()
	logger.Info("=== TRUF CRE Write Workflow: Deploy & Insert Demo ===")

	// Run in NodeMode to access HTTP capabilities
	return cre.RunInNodeMode(config, runtime,
		func(config *Config, nodeRuntime cre.NodeRuntime) (*ExecutionResult, error) {
			result := &ExecutionResult{}

			// Stream configuration
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

			// Data provider is the signer's address (derived from private key)
			result.DataProvider = signerAddr
			logger.Info("Signer created", "address", signerAddr, "role", "data provider")

			// ========================================================
			// KEY CRE + TRUF INTEGRATION: Using CRETransport with signer
			// ========================================================
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

			// Load primitive actions
			primitiveActions, err := client.LoadPrimitiveActions()
			if err != nil {
				logger.Error("Failed to load primitive actions", "error", err)
				result.Error = fmt.Sprintf("Failed to load primitive actions: %v", err)
				return result, nil
			}

			// ========================================================
			// STEP 1: DEPLOY PRIMITIVE STREAM
			// ========================================================
			logger.Info("=== Step 1: Deploying Primitive Stream ===", "streamId", streamNameStr)

			deployTxHash, err := client.DeployStream(ctx, *streamId, types.StreamTypePrimitive)
			if err != nil {
				// Check if it's a duplicate key error (stream already exists)
				if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "already exists") {
					logger.Info("Stream already exists, continuing with existing stream", "streamId", streamNameStr)
					result.Deployed = true
				} else {
					logger.Error("Stream deployment failed", "error", err)
					result.Error = fmt.Sprintf("Deployment failed: %v", err)
					return result, nil
				}
			} else {
				result.Deployed = true
				logger.Info("✅ Stream deployment transaction submitted", "txHash", deployTxHash.String(), "streamId", streamNameStr)
			}

			// Note: Final cleanup (delete stream) is skipped due to CRE simulation's 5 HTTP request limit
			// In production, you would delete the stream here using primitiveActions.DestroyStream()

			// ========================================================
			// STEP 2: INSERT RECORDS
			// ========================================================
			logger.Info("=== Step 2: Inserting Records ===")

			// Insert 1 sample record (limited by CRE simulation's 5 HTTP request limit)
			currentTime := time.Now().Unix()
			recordsToInsert := []struct {
				timestamp int64
				value     float64
			}{
				{currentTime, 102.7}, // now
			}

			for i, record := range recordsToInsert {
				insertTxHash, insertErr := primitiveActions.InsertRecords(ctx, []types.InsertRecordInput{
					{
						DataProvider: signerAddr,
						StreamId:     streamNameStr,
						EventTime:    int(record.timestamp),
						Value:        record.value,
					},
				})
				if insertErr != nil {
					logger.Error("Record insertion failed", "index", i, "error", insertErr)
					result.Error = fmt.Sprintf("Insert record %d failed: %v", i, insertErr)
					return result, nil
				}

				logger.Info("Record insert transaction submitted", "index", i+1, "txHash", insertTxHash.String(), "timestamp", record.timestamp, "value", record.value)
			}

			result.RecordsInserted = len(recordsToInsert)
			result.Success = true
			logger.Info("✅ Records inserted successfully", "count", result.RecordsInserted)

			logger.Info("=== Write Workflow Completed Successfully ===")
			logger.Info("Note: Stream remains active for read workflow to query")
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
