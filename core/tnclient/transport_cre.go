//go:build wasip1

package tnclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clientType "github.com/trufnetwork/kwil-db/core/client/types"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/kwil-db/core/rpc/client/gateway"
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
	runtime            cre.NodeRuntime
	client             *http.Client
	endpoint           string
	signer             auth.Signer
	chainID            string
	chainIDMu          sync.RWMutex
	chainIDInitialized bool
	reqID              atomic.Uint64
	authCookie         string // Cookie value for gateway authentication
	authCookieMu       sync.RWMutex
	currentNonce       int64 // Track nonce for sequential transactions
	nonceMu            sync.Mutex
	nonceFetched       bool
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
	// Append /rpc/v1 if not already present (kwil-db client adds this automatically)
	// First trim trailing slashes to prevent duplication (e.g., "/rpc/v1/" → "/rpc/v1/rpc/v1")
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(endpoint, "/rpc/v1") {
		endpoint = endpoint + "/rpc/v1"
	}

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
// It automatically handles authentication if the endpoint returns 401
func (t *CRETransport) callJSONRPC(ctx context.Context, method string, params any, result any) error {
	// Try the call
	err := t.doJSONRPC(ctx, method, params, result)

	// If we get a 401, try authenticating and retry once
	if err != nil && strings.Contains(err.Error(), "401") {
		if t.signer == nil {
			return fmt.Errorf("%w: signer is nil, cannot authenticate", err)
		}
		// Authenticate with gateway
		authErr := t.authenticate(ctx)
		if authErr != nil {
			return fmt.Errorf("authentication failed: %w (original 401 for method %s)", authErr, method)
		}
		// Retry the call
		retryErr := t.doJSONRPC(ctx, method, params, result)
		if retryErr != nil {
			return fmt.Errorf("retry after auth failed: %w (method: %s)", retryErr, method)
		}
		return nil
	}

	return err
}

// doJSONRPC performs the actual JSON-RPC call without authentication retry
func (t *CRETransport) doJSONRPC(ctx context.Context, method string, params any, result any) error {
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

	// Create headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Add auth cookie if we have one
	t.authCookieMu.RLock()
	if t.authCookie != "" {
		headers["Cookie"] = t.authCookie
	}
	t.authCookieMu.RUnlock()

	// Create CRE HTTP request
	httpReq := &http.Request{
		Url:     t.endpoint,
		Method:  "POST",
		Body:    requestBody,
		Headers: headers,
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
		// For broadcast errors (-201), decode the BroadcastError details
		if rpcResp.Error.Code == -201 && len(rpcResp.Error.Data) > 0 {
			var broadcastErr struct {
				Code    uint32 `json:"code"`
				Hash    string `json:"hash"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rpcResp.Error.Data, &broadcastErr); err == nil {
				return fmt.Errorf("JSON-RPC error: %s (code: %d) [Broadcast: code=%d, hash=%s, msg=%s]",
					rpcResp.Error.Message, rpcResp.Error.Code,
					broadcastErr.Code, broadcastErr.Hash, broadcastErr.Message)
			}
		}
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
	// Use "main" as default namespace if empty (TRUF.NETWORK convention)
	if namespace == "" {
		namespace = "main"
	}

	// Encode inputs to EncodedValue array
	var encodedInputs []*types.EncodedValue
	for _, val := range inputs {
		encoded, err := types.EncodeValue(val)
		if err != nil {
			return nil, fmt.Errorf("failed to encode input value: %w", err)
		}
		encodedInputs = append(encodedInputs, encoded)
	}

	// Build ActionCall payload
	payload := &types.ActionCall{
		Namespace: namespace,
		Action:    action,
		Arguments: encodedInputs,
	}

	// Create CallMessage
	// Call operations are read-only and typically don't require authentication,
	// but we pass the signer (if configured) to support authenticated gateway calls.
	// The challenge is nil for standard calls (vs. Execute which requires it).
	callMsg, err := types.CreateCallMessage(payload, nil, t.signer)
	if err != nil {
		return nil, fmt.Errorf("failed to create call message: %w", err)
	}

	var result types.CallResult
	if err := t.callJSONRPC(ctx, "user.call", callMsg, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Execute performs a write action and returns the transaction hash.
//
// This method builds a signed transaction and broadcasts it to the TRUF.NETWORK.
// The transaction is signed using the configured signer and executed within CRE's
// consensus mechanism. Automatically retries on nonce errors.
func (t *CRETransport) Execute(ctx context.Context, namespace string, action string, inputs [][]any, opts ...clientType.TxOpt) (types.Hash, error) {
	if t.signer == nil {
		return types.Hash{}, fmt.Errorf("signer required for Execute operations")
	}

	// Retry loop for nonce errors
	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		txHash, err := t.executeOnce(ctx, namespace, action, inputs, opts...)
		if err != nil {
			// Check if it's a nonce error
			if strings.Contains(err.Error(), "invalid nonce") && attempt < maxRetries-1 {
				// Reset nonce tracking to refetch on next attempt
				t.nonceMu.Lock()
				t.nonceFetched = false
				t.nonceMu.Unlock()
				continue // Retry
			}
			return types.Hash{}, err
		}
		return txHash, nil
	}

	return types.Hash{}, fmt.Errorf("max retries exceeded")
}

// executeOnce performs a single execute attempt (internal helper)
func (t *CRETransport) executeOnce(ctx context.Context, namespace string, action string, inputs [][]any, opts ...clientType.TxOpt) (types.Hash, error) {
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

	// Auto-manage nonce if not explicitly provided
	if txOpts.Nonce == 0 {
		t.nonceMu.Lock()

		// Fetch nonce from gateway on first transaction only
		if !t.nonceFetched {
			// Create AccountID from signer
			acctID := &types.AccountID{
				Identifier: t.signer.CompactID(),
				KeyType:    t.signer.PubKey().Type(),
			}

			// Fetch account info via user.account RPC call
			params := map[string]any{
				"id": acctID,
			}

			var accountResp struct {
				ID      *types.AccountID `json:"id"`
				Balance string           `json:"balance"`
				Nonce   int64            `json:"nonce"`
			}

			err := t.callJSONRPC(ctx, "user.account", params, &accountResp)
			if err != nil {
				// If account doesn't exist yet, start with nonce 0
				if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
					t.nonceMu.Unlock()
					return types.Hash{}, fmt.Errorf("failed to fetch account nonce: %w", err)
				}
				t.currentNonce = 0
			} else {
				// Account nonce is the LAST used nonce, so NEXT nonce is nonce+1
				t.currentNonce = accountResp.Nonce + 1
			}
			t.nonceFetched = true
		}

		// Use current nonce and increment
		txOpts.Nonce = t.currentNonce
		t.currentNonce++

		t.nonceMu.Unlock()
	}

	// Ensure chain ID is fetched before building transaction
	// This prevents transactions with empty chain IDs
	// Check if already initialized (read lock)
	t.chainIDMu.RLock()
	initialized := t.chainIDInitialized
	chainID := t.chainID
	t.chainIDMu.RUnlock()

	if !initialized {
		// Need to fetch chain ID (write lock)
		t.chainIDMu.Lock()
		// Double-check after acquiring write lock
		if !t.chainIDInitialized {
			if err := t.fetchChainID(ctx); err != nil {
				t.chainIDMu.Unlock()
				return types.Hash{}, fmt.Errorf("failed to fetch chain ID: %w", err)
			}
			// Only mark as initialized if fetchChainID succeeded (returned non-empty chainID)
			t.chainIDInitialized = true
		}
		chainID = t.chainID
		t.chainIDMu.Unlock()
	}

	// Ensure Fee is not nil to prevent signature verification mismatch
	// When Fee is nil, SerializeMsg produces "Fee: <nil>" but after JSON
	// marshaling/unmarshaling it becomes "Fee: 0", causing signature mismatch
	fee := txOpts.Fee
	if fee == nil {
		fee = big.NewInt(0)
	}

	// Build unsigned transaction
	tx := &types.Transaction{
		Body: &types.TransactionBody{
			Payload:     payloadBytes,
			PayloadType: payload.Type(),
			Fee:         fee,
			Nonce:       uint64(txOpts.Nonce),
			ChainID:     chainID,
		},
		Serialization: types.DefaultSignedMsgSerType, // Required for EthPersonalSigner
	}

	// Sign transaction
	if err := tx.Sign(t.signer); err != nil {
		return types.Hash{}, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Pre-serialize transaction to avoid WASM pointer corruption
	// Go WASM uses 64-bit pointers but WASM runtime uses 32-bit pointers.
	// Transaction struct contains pointer fields (Signature, Body) which get
	// corrupted when crossing the WASM boundary (golang/go#59156, golang/go#66984).
	// Solution: Manually construct JSON-RPC request to avoid struct traversal in WASM.
	txJSON, err := json.Marshal(tx)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to marshal transaction: %w", err)
	}

	// Manually construct JSON-RPC request to bypass params map
	reqID := t.nextReqID()
	rpcReqJSON := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":"%s","method":"user.broadcast","params":{"tx":%s}}`,
		reqID, string(txJSON))

	// Create headers
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Add auth cookie if we have one
	t.authCookieMu.RLock()
	if t.authCookie != "" {
		headers["Cookie"] = t.authCookie
	}
	t.authCookieMu.RUnlock()

	// Create CRE HTTP request
	httpReq := &http.Request{
		Url:     t.endpoint,
		Method:  "POST",
		Body:    []byte(rpcReqJSON),
		Headers: headers,
	}

	// Execute via CRE client
	httpResp, err := t.client.SendRequest(t.runtime, httpReq).Await()
	if err != nil {
		return types.Hash{}, fmt.Errorf("CRE HTTP request failed: %w", err)
	}

	// Check HTTP status
	if httpResp.StatusCode != 200 {
		return types.Hash{}, fmt.Errorf("unexpected HTTP status code: %d", httpResp.StatusCode)
	}

	// Parse JSON-RPC response
	var rpcResp jsonrpc.Response
	if err := json.Unmarshal(httpResp.Body, &rpcResp); err != nil {
		return types.Hash{}, fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC errors
	if rpcResp.Error != nil {
		// For broadcast errors (-201), decode the BroadcastError details
		if rpcResp.Error.Code == -201 && len(rpcResp.Error.Data) > 0 {
			var broadcastErr struct {
				Code    uint32 `json:"code"`
				Hash    string `json:"hash"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rpcResp.Error.Data, &broadcastErr); err == nil {
				return types.Hash{}, fmt.Errorf("JSON-RPC error: %s (code: %d) [Broadcast: code=%d, hash=%s, msg=%s]",
					rpcResp.Error.Message, rpcResp.Error.Code,
					broadcastErr.Code, broadcastErr.Hash, broadcastErr.Message)
			}
		}
		return types.Hash{}, fmt.Errorf("JSON-RPC error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	// Unmarshal result
	var result struct {
		TxHash types.Hash `json:"tx_hash"`
	}
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return types.Hash{}, fmt.Errorf("failed to unmarshal result: %w", err)
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
				// Distinguish between transient errors (retry-able) and permanent errors
				if !isTransientTxError(err) {
					// Permanent error - authentication failure, network issues, malformed request
					return nil, fmt.Errorf("transaction query failed: %w", err)
				}
				// Transient error (tx not indexed yet) - continue polling
				continue
			}

			// Check if transaction is finalized (either committed or rejected)
			if result.Height > 0 {
				return &result, nil
			}
		}
	}
}

// isTransientTxError determines if an error from tx_query is transient (retry-able).
//
// Strategy:
// 1. First, try to parse as JSON-RPC error and check error code
// 2. Fall back to substring matching if not a structured JSON-RPC error
//
// Known transient error codes:
//   - -202 (ErrorTxNotFound): Transaction not yet indexed
//   - -32001 (ErrorTimeout): Temporary timeout
//
// Fragility warning: The substring fallback is brittle and may misclassify errors.
// Consider adding structured error codes to the gateway API for better reliability.
func isTransientTxError(err error) bool {
	if err == nil {
		return false
	}

	// Try to extract JSON-RPC error code from the error message
	// The error from callJSONRPC is formatted as "JSON-RPC error: <message> (code: <code>)"
	errMsg := err.Error()

	// Use regex to extract error code from "(code: <number>)" pattern
	// This handles multi-word error messages unlike fmt.Sscanf with %*s
	codePattern := regexp.MustCompile(`\(code:\s*(-?\d+)\)`)
	matches := codePattern.FindStringSubmatch(errMsg)

	if len(matches) >= 2 {
		// Parse the captured code number
		if codeInt, err := strconv.ParseInt(matches[1], 10, 32); err == nil {
			code := jsonrpc.ErrorCode(int32(codeInt))

			// Check known transient error codes
			switch code {
			case jsonrpc.ErrorTxNotFound: // -202: Transaction not indexed yet
				return true
			case jsonrpc.ErrorTimeout: // -32001: Temporary timeout
				return true
			}
			// Other structured errors are likely permanent
			return false
		}
	}

	// Fallback: Check for transient error patterns in message
	// This is fragile and may need updates as error messages change
	transientPatterns := []string{
		"not found",
		"not indexed",
		"pending",
		"unknown transaction",
		"timeout",
	}

	lowerMsg := strings.ToLower(errMsg)
	for _, pattern := range transientPatterns {
		if strings.Contains(lowerMsg, pattern) {
			return true
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

	// Validate that chain ID is not empty
	if result.ChainID == "" {
		return fmt.Errorf("gateway returned empty chain ID")
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
// Unlike sync.Once, this will retry on transient failures.
func (t *CRETransport) ChainID() string {
	// Fast path: check if already initialized (read lock)
	t.chainIDMu.RLock()
	if t.chainIDInitialized {
		chainID := t.chainID
		t.chainIDMu.RUnlock()
		return chainID
	}
	t.chainIDMu.RUnlock()

	// Slow path: fetch chain ID (write lock)
	t.chainIDMu.Lock()
	defer t.chainIDMu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have initialized it)
	if t.chainIDInitialized {
		return t.chainID
	}

	// Fetch chain ID with timeout to prevent indefinite hanging
	// Use a reasonable timeout since this is a lightweight metadata query
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := t.fetchChainID(ctx); err != nil {
		// Don't set initialized flag - allow retry on next call
		return ""
	}

	// Mark as successfully initialized only on success
	t.chainIDInitialized = true
	return t.chainID
}

// Signer returns the cryptographic signer used for transaction authentication.
//
// Returns nil if no signer is configured (read-only mode).
func (t *CRETransport) Signer() auth.Signer {
	return t.signer
}

// authenticate performs gateway authentication and stores the cookie.
// This is called automatically when a 401 error is received.
func (t *CRETransport) authenticate(ctx context.Context) error {
	if t.signer == nil {
		return fmt.Errorf("cannot authenticate without a signer")
	}

	// Get authentication parameters from gateway
	var authParam gateway.AuthnParameterResponse
	if err := t.doJSONRPC(ctx, string(gateway.MethodAuthnParam), &struct{}{}, &authParam); err != nil {
		return fmt.Errorf("failed to get auth parameters (kgw.authn_param): %w", err)
	}

	// Parse endpoint to get domain (remove /rpc/v1 path if present)
	parsedURL, err := url.Parse(t.endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint URL %s: %w", t.endpoint, err)
	}
	// Use just scheme + host, without the /rpc/v1 path
	targetDomain := parsedURL.Scheme + "://" + parsedURL.Host

	// Get chain ID
	chainID := t.ChainID()
	if chainID == "" {
		return fmt.Errorf("failed to get chain ID for authentication")
	}

	// Compose authentication message (SIWE-like format)
	msg := composeGatewayAuthMessage(&authParam, targetDomain, authParam.URI, "1", chainID)

	// Sign the message
	sig, err := t.signer.Sign([]byte(msg))
	if err != nil {
		return fmt.Errorf("failed to sign auth message: %w", err)
	}

	// Send authentication request
	authReq := &gateway.AuthnRequest{
		Nonce:     authParam.Nonce,
		Sender:    t.signer.CompactID(),
		Signature: sig,
	}

	// Make the auth request and capture the response headers
	authResp, err := t.doJSONRPCWithResponse(ctx, string(gateway.MethodAuthn), authReq)
	if err != nil {
		return fmt.Errorf("kgw.authn request failed: %w", err)
	}

	// Extract Set-Cookie header from response
	setCookie, ok := authResp["set-cookie"]
	if !ok || setCookie == "" {
		// Try other common header names
		if sc, exists := authResp["Set-Cookie"]; exists {
			setCookie = sc
			ok = true
		}
	}

	if ok && setCookie != "" {
		// Parse the cookie (just extract the name=value part)
		cookieParts := strings.Split(setCookie, ";")
		if len(cookieParts) > 0 {
			t.authCookieMu.Lock()
			t.authCookie = cookieParts[0] // Store just the name=value part
			t.authCookieMu.Unlock()
		}
	} else {
		return fmt.Errorf("no Set-Cookie header in kgw.authn response")
	}

	return nil
}

// doJSONRPCWithResponse performs a JSON-RPC call and returns the response headers.
// This is used for authentication to extract the Set-Cookie header.
func (t *CRETransport) doJSONRPCWithResponse(ctx context.Context, method string, params any) (map[string]string, error) {
	// Marshal the params
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	// Create JSON-RPC request
	reqID := t.nextReqID()
	rpcReq := jsonrpc.NewRequest(reqID, method, paramsJSON)

	// Marshal the full request
	requestBody, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
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
		return nil, fmt.Errorf("CRE HTTP request failed: %w", err)
	}

	// Check HTTP status
	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected HTTP status code: %d", httpResp.StatusCode)
	}

	// Parse JSON-RPC response
	var rpcResp jsonrpc.Response
	if err := json.Unmarshal(httpResp.Body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC errors
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	// Return the response headers
	return httpResp.GetHeaders(), nil
}

// composeGatewayAuthMessage composes a SIWE-like authentication message.
// This matches the format used by kwil-db gateway client.
// Note: This is a custom format, not standard SIWE - it omits the account address line
// and uses "Issue At" instead of "Issued At" to match kgw's expectations.
func composeGatewayAuthMessage(param *gateway.AuthnParameterResponse, domain string, uri string, version string, chainID string) string {
	var msg bytes.Buffer
	msg.WriteString(domain + " wants you to sign in with your account:\n")
	msg.WriteString("\n")
	if param.Statement != "" {
		msg.WriteString(param.Statement + "\n")
	}
	msg.WriteString("\n")
	msg.WriteString(fmt.Sprintf("URI: %s\n", uri))
	msg.WriteString(fmt.Sprintf("Version: %s\n", version))
	msg.WriteString(fmt.Sprintf("Chain ID: %s\n", chainID))
	msg.WriteString(fmt.Sprintf("Nonce: %s\n", param.Nonce))
	msg.WriteString(fmt.Sprintf("Issue At: %s\n", param.IssueAt)) // Note: "Issue At" not "Issued At" (kgw custom format)
	if param.ExpirationTime != "" {
		msg.WriteString(fmt.Sprintf("Expiration Time: %s\n", param.ExpirationTime))
	}
	return msg.String()
}
