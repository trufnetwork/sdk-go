//go:build wasip1

package tnclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	clientType "github.com/trufnetwork/kwil-db/core/client/types"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	jsonrpc "github.com/trufnetwork/kwil-db/core/rpc/json"
	"github.com/trufnetwork/kwil-db/core/types"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http"
	"github.com/smartcontractkit/cre-sdk-go/cre"
)

// CRETransport implements Transport using Chainlink CRE's HTTP client.
//
// This transport is designed for use in Chainlink Runtime Environment (CRE)
// workflows where standard net/http is not available. It uses CRE's consensus-aware
// HTTP client with Promise-based async operations.
//
// Example usage in CRE workflow:
//
//	func onCronTrigger(config *Config, runtime cre.Runtime, trigger *cron.Payload) (*Result, error) {
//	    return cre.RunInNodeMode(config, runtime,
//	        func(config *Config, nodeRuntime cre.NodeRuntime) (*Result, error) {
//	            // Create TN client with CRE transport
//	            client, err := tnclient.NewClient(context.Background(), config.TRUFEndpoint,
//	                tnclient.WithCRETransport(nodeRuntime),
//	            )
//	            if err != nil {
//	                return nil, err
//	            }
//
//	            // Use client normally - all methods work!
//	            actions, _ := client.LoadActions()
//	            result, err := actions.GetRecord(context.Background(), ...)
//	            return &Result{Records: result.Results}, nil
//	        },
//	        cre.ConsensusAggregationFromTags[*Result](),
//	    ).Await()
//	}
type CRETransport struct {
	runtime     cre.NodeRuntime
	client      *http.Client
	endpoint    string
	signer      auth.Signer
	chainID     string
	chainIDOnce sync.Once
	chainIDErr  error
	reqID       atomic.Uint64
}

// Verify CRETransport implements Transport interface at compile time
var _ Transport = (*CRETransport)(nil)

// NewCRETransport creates a new CRE transport for use in Chainlink workflows.
//
// Parameters:
//   - runtime: The CRE NodeRuntime provided by the workflow execution context
//   - endpoint: HTTP(S) endpoint URL (e.g., "https://gateway.example.com")
//   - signer: Cryptographic signer for transaction authentication (can be nil for read-only)
//
// Returns:
//   - Configured CRETransport instance
//   - Error if initialization fails
//
// Example:
//
//	transport, err := NewCRETransport(nodeRuntime, "https://gateway.example.com", signer)
//	if err != nil {
//	    return err
//	}
func NewCRETransport(runtime cre.NodeRuntime, endpoint string, signer auth.Signer) (*CRETransport, error) {
	return &CRETransport{
		runtime:  runtime,
		client:   &http.Client{},
		endpoint: endpoint,
		signer:   signer,
		chainID:  "", // Will be fetched on first call if needed
	}, nil
}

// nextReqID generates the next JSON-RPC request ID
func (t *CRETransport) nextReqID() string {
	id := t.reqID.Add(1)
	return strconv.FormatUint(id, 10)
}

// callJSONRPC makes a JSON-RPC call via CRE HTTP client
func (t *CRETransport) callJSONRPC(ctx context.Context, method string, params any, result any) error {
	// Marshal the params
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	// Create JSON-RPC request
	reqID := t.nextReqID()
	rpcReq := jsonrpc.NewRequest(reqID, method, paramsJSON)

	// Marshal the full request
	requestBody, err := json.Marshal(rpcReq)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
	}

	// Create CRE HTTP request
	httpReq := &http.Request{
		Url:    t.endpoint,
		Method: "POST",
		Body:   requestBody,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	// Execute via CRE client (returns Promise)
	httpResp, err := t.client.SendRequest(t.runtime, httpReq).Await()
	if err != nil {
		return fmt.Errorf("CRE HTTP request failed: %w", err)
	}

	// Check HTTP status
	if httpResp.StatusCode != 200 {
		return fmt.Errorf("unexpected HTTP status code: %d", httpResp.StatusCode)
	}

	// Parse JSON-RPC response
	var rpcResp jsonrpc.Response
	if err := json.Unmarshal(httpResp.Body, &rpcResp); err != nil {
		return fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC errors
	if rpcResp.Error != nil {
		return fmt.Errorf("JSON-RPC error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	// Verify JSON-RPC version
	if rpcResp.JSONRPC != "2.0" {
		return fmt.Errorf("invalid JSON-RPC response version: %s", rpcResp.JSONRPC)
	}

	// Unmarshal result into provided struct
	if result != nil {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}

// Call executes a read-only action and returns results.
//
// This method uses CRE's HTTP client to make a JSON-RPC call to the TRUF.NETWORK gateway.
// The call is executed within CRE's consensus mechanism, ensuring all nodes in the DON
// reach agreement on the result.
func (t *CRETransport) Call(ctx context.Context, namespace string, action string, inputs []any) (*types.CallResult, error) {
	// Build call params matching kwil-db's user/call endpoint
	params := map[string]any{
		"dbid":   namespace,
		"action": action,
		"inputs": inputs,
	}

	var result types.CallResult
	if err := t.callJSONRPC(ctx, "user.call", params, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Execute performs a write action and returns the transaction hash.
//
// This method builds a signed transaction and broadcasts it to the TRUF.NETWORK.
// The transaction is signed using the configured signer and executed within CRE's
// consensus mechanism.
func (t *CRETransport) Execute(ctx context.Context, namespace string, action string, inputs [][]any, opts ...clientType.TxOpt) (types.Hash, error) {
	if t.signer == nil {
		return types.Hash{}, fmt.Errorf("signer required for Execute operations")
	}

	// Convert inputs to EncodedValue arrays
	var encodedInputs [][]*types.EncodedValue
	for _, inputRow := range inputs {
		var encodedRow []*types.EncodedValue
		for _, val := range inputRow {
			encoded, err := types.EncodeValue(val)
			if err != nil {
				return types.Hash{}, fmt.Errorf("failed to encode input value: %w", err)
			}
			encodedRow = append(encodedRow, encoded)
		}
		encodedInputs = append(encodedInputs, encodedRow)
	}

	// Build transaction payload using ActionExecution
	payload := &types.ActionExecution{
		Namespace: namespace,
		Action:    action,
		Arguments: encodedInputs,
	}

	// Serialize payload
	payloadBytes, err := payload.MarshalBinary()
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Apply transaction options (nonce, fee, etc.)
	txOpts := &clientType.TxOptions{}
	for _, opt := range opts {
		opt(txOpts)
	}

	// Ensure chain ID is fetched before building transaction
	// This prevents transactions with empty chain IDs
	t.chainIDOnce.Do(func() {
		t.chainIDErr = t.fetchChainID(ctx)
	})
	if t.chainIDErr != nil {
		return types.Hash{}, fmt.Errorf("failed to get chain ID: %w", t.chainIDErr)
	}
	if t.chainID == "" {
		return types.Hash{}, fmt.Errorf("chain ID is empty")
	}

	// Build unsigned transaction
	tx := &types.Transaction{
		Body: &types.TransactionBody{
			Payload:     payloadBytes,
			PayloadType: payload.Type(),
			Fee:         txOpts.Fee,
			Nonce:       uint64(txOpts.Nonce),
			ChainID:     t.chainID,
		},
	}

	// Sign transaction
	if err := tx.Sign(t.signer); err != nil {
		return types.Hash{}, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Broadcast transaction
	params := map[string]any{
		"tx": tx,
	}

	var result struct {
		TxHash types.Hash `json:"tx_hash"`
	}

	if err := t.callJSONRPC(ctx, "user.broadcast", params, &result); err != nil {
		return types.Hash{}, err
	}

	return result.TxHash, nil
}

// WaitTx polls for transaction confirmation with the specified interval.
//
// This method repeatedly queries the transaction status until it's confirmed,
// rejected, or the context is cancelled. It uses CRE's HTTP client for each poll.
func (t *CRETransport) WaitTx(ctx context.Context, txHash types.Hash, interval time.Duration) (*types.TxQueryResponse, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			// Query transaction status
			params := map[string]any{
				"tx_hash": txHash,
			}

			var result types.TxQueryResponse
			if err := t.callJSONRPC(ctx, "user.tx_query", params, &result); err != nil {
				// Distinguish between transient errors (not indexed yet) and permanent errors
				// JSON-RPC errors with specific codes or messages about "not found" are transient
				errMsg := err.Error()
				// Common transient error indicators: not found, not indexed, pending
				isTransient := containsAny(errMsg, []string{"not found", "not indexed", "pending", "unknown transaction"})

				if !isTransient {
					// Permanent error - authentication failure, network issues, malformed request
					return nil, fmt.Errorf("transaction query failed: %w", err)
				}
				// Transient error - continue polling
				continue
			}

			// Check if transaction is finalized (either committed or rejected)
			if result.Height > 0 {
				return &result, nil
			}
		}
	}
}

// containsAny checks if a string contains any of the specified substrings (case-insensitive)
func containsAny(s string, substrings []string) bool {
	lowerS := s
	for _, substr := range substrings {
		if len(substr) == 0 {
			continue
		}
		// Simple case-insensitive substring check
		for i := 0; i <= len(lowerS)-len(substr); i++ {
			match := true
			for j := 0; j < len(substr); j++ {
				c1 := lowerS[i+j]
				c2 := substr[j]
				// Convert to lowercase for comparison
				if c1 >= 'A' && c1 <= 'Z' {
					c1 += 'a' - 'A'
				}
				if c2 >= 'A' && c2 <= 'Z' {
					c2 += 'a' - 'A'
				}
				if c1 != c2 {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// fetchChainID fetches and caches the chain ID from the gateway.
// This is called once via sync.Once to ensure thread-safe lazy initialization.
// Returns error if the fetch fails, which can be checked before critical operations.
func (t *CRETransport) fetchChainID(ctx context.Context) error {
	// Fetch chain info from gateway
	var result struct {
		ChainID string `json:"chain_id"`
	}

	if err := t.callJSONRPC(ctx, "user.chain_info", map[string]any{}, &result); err != nil {
		return fmt.Errorf("failed to fetch chain ID: %w", err)
	}

	// Cache the chain ID
	t.chainID = result.ChainID
	return nil
}

// ChainID returns the network chain identifier.
//
// The chain ID is fetched from the gateway on first call and cached.
// This is used to ensure transactions are sent to the correct network.
// Returns empty string if the chain ID fetch fails.
func (t *CRETransport) ChainID() string {
	// Use sync.Once to ensure thread-safe lazy initialization
	t.chainIDOnce.Do(func() {
		// Use a background context since this is a cached operation
		t.chainIDErr = t.fetchChainID(context.Background())
	})

	// Return cached chain ID (will be empty if fetch failed)
	return t.chainID
}

// Signer returns the cryptographic signer used for transaction authentication.
//
// Returns nil if no signer is configured (read-only mode).
func (t *CRETransport) Signer() auth.Signer {
	return t.signer
}
