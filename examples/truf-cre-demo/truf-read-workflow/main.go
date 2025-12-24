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
// Demonstrates read operations: Get Records
func onCronTrigger(config *Config, runtime cre.Runtime, trigger *cron.Payload) (*ExecutionResult, error) {
	logger := runtime.Logger()
	logger.Info("=== TRUF CRE Read Workflow: Get Records Demo ===")

	// Run in NodeMode to access HTTP capabilities
	return cre.RunInNodeMode(config, runtime,
		func(config *Config, nodeRuntime cre.NodeRuntime) (*ExecutionResult, error) {
			result := &ExecutionResult{}

			// Stream configuration (must match write workflow)
			const streamNameStr = "stcreteststream00000000000000000"
			result.StreamID = streamNameStr

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
			// STEP 1: GET RECORDS
			// ========================================================
			logger.Info("=== Step 1: Retrieving Records ===")

			recordCount, topRecords, err := getRecordsCount(ctx, client, signerAddr, streamNameStr, logger)
			if err != nil {
				logger.Error("Get records failed", "error", err)
				result.Error = fmt.Sprintf("Get records failed: %v", err)
				return result, nil
			}

			result.RecordsRetrieved = recordCount
			result.TopRecords = topRecords
			result.Success = true
			logger.Info("✅ Records retrieved successfully", "count", recordCount)

			logger.Info("=== Read Workflow Completed Successfully ===")
			return result, nil
		},
		cre.ConsensusAggregationFromTags[*ExecutionResult](),
	).Await()
}

// getRecordsCount retrieves and counts records from the stream, returning top 5 in descending order
func getRecordsCount(ctx context.Context, client *tnclient.Client, dataProvider, streamID string, logger *slog.Logger) (int, []RecordData, error) {
	// Load actions
	actions, err := client.LoadActions()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to load actions: %w", err)
	}

	// Get records from the last 24 hours
	now := int(time.Now().Unix())
	from := int(time.Now().Add(-24 * time.Hour).Unix())

	input := types.GetRecordInput{
		DataProvider: dataProvider,
		StreamId:     streamID,
		From:         &from,
		To:           &now,
	}

	// Get records using the action
	actionResult, err := actions.GetRecord(ctx, input)
	if err != nil {
		return 0, nil, fmt.Errorf("get records failed: %w", err)
	}

	// Sort records in descending order by eventTime (newest first)
	records := actionResult.Results
	for i := 0; i < len(records)-1; i++ {
		for j := i + 1; j < len(records); j++ {
			if records[i].EventTime < records[j].EventTime {
				records[i], records[j] = records[j], records[i]
			}
		}
	}

	// Prepare top 5 records for return
	limit := 5
	if len(records) < limit {
		limit = len(records)
	}

	topRecords := make([]RecordData, limit)
	logger.Info("Top 5 records (descending by eventTime)", "total", len(records))

	for i := 0; i < limit; i++ {
		timestamp := time.Unix(int64(records[i].EventTime), 0).Format(time.RFC3339)
		topRecords[i] = RecordData{
			Rank:      i + 1,
			EventTime: records[i].EventTime,
			Timestamp: timestamp,
			Value:     records[i].Value.String(),
		}

		logger.Info("Record",
			"rank", topRecords[i].Rank,
			"eventTime", topRecords[i].EventTime,
			"timestamp", topRecords[i].Timestamp,
			"value", topRecords[i].Value)
	}

	return len(actionResult.Results), topRecords, nil
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
