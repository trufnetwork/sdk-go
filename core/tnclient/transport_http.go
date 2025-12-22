package tnclient

import (
	"context"
	"fmt"
	"time"

	clientType "github.com/trufnetwork/kwil-db/core/client/types"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/kwil-db/core/gatewayclient"
	"github.com/trufnetwork/kwil-db/core/log"
	"github.com/trufnetwork/kwil-db/core/types"
)

// HTTPTransport implements Transport using standard net/http via kwil-db's GatewayClient.
// This is the default transport used by the SDK.
//
// HTTPTransport provides:
//   - Standard HTTP/HTTPS communication via net/http
//   - Gateway authentication with cookie management
//   - JSON-RPC 2.0 protocol handling
//   - Automatic retry on authentication errors
//
// HTTPTransport wraps the existing GatewayClient to maintain backwards compatibility
// while enabling the Transport abstraction pattern.
type HTTPTransport struct {
	gatewayClient *gatewayclient.GatewayClient
}

// Verify HTTPTransport implements Transport interface at compile time
var _ Transport = (*HTTPTransport)(nil)

// NewHTTPTransport creates a new HTTP transport using standard net/http.
//
// This function initializes a GatewayClient with the provided configuration
// and wraps it to implement the Transport interface.
//
// Parameters:
//   - ctx: Context for client initialization
//   - provider: HTTP(S) endpoint URL (e.g., "https://gateway.example.com")
//   - signer: Cryptographic signer for transaction authentication (can be nil for read-only operations)
//   - logger: Optional logger for debugging (can be nil)
//
// Returns:
//   - Configured HTTPTransport instance
//   - Error if client creation fails
//
// Example:
//
//	transport, err := NewHTTPTransport(ctx, "https://gateway.example.com", signer, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewHTTPTransport(ctx context.Context, provider string, signer auth.Signer, logger log.Logger) (*HTTPTransport, error) {
	// Create GatewayOptions with defaults
	opts := &gatewayclient.GatewayOptions{
		Options: *clientType.DefaultOptions(),
	}

	// Configure signer if provided
	if signer != nil {
		opts.Signer = signer
		// AuthSignFunc is already set to defaultGatewayAuthSignFunc by DefaultOptions()
	}

	// Configure logger if provided
	if logger != nil {
		opts.Logger = logger
	}

	// Create the underlying GatewayClient
	gwClient, err := gatewayclient.NewClient(ctx, provider, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway client: %w", err)
	}

	return &HTTPTransport{
		gatewayClient: gwClient,
	}, nil
}

// Call executes a read-only action and returns results.
// This method delegates to the underlying GatewayClient's Call method.
//
// The call is authenticated if a signer is configured. If authentication
// fails with a 401 error, the transport will automatically re-authenticate
// and retry the request.
func (t *HTTPTransport) Call(ctx context.Context, namespace string, action string, inputs []any) (*types.CallResult, error) {
	return t.gatewayClient.Call(ctx, namespace, action, inputs)
}

// Execute performs a write action and returns the transaction hash.
// This method delegates to the underlying GatewayClient's Execute method.
//
// The transaction will be signed using the configured signer. Options can
// include custom nonce, fee, and other transaction parameters.
func (t *HTTPTransport) Execute(ctx context.Context, namespace string, action string, inputs [][]any, opts ...clientType.TxOpt) (types.Hash, error) {
	return t.gatewayClient.Execute(ctx, namespace, action, inputs, opts...)
}

// WaitTx polls for transaction confirmation with the specified interval.
// This method delegates to the underlying GatewayClient's WaitTx method.
//
// The method blocks until the transaction is confirmed, rejected, or the
// context is cancelled. It polls the transaction status at the specified interval.
func (t *HTTPTransport) WaitTx(ctx context.Context, txHash types.Hash, interval time.Duration) (*types.TxQueryResponse, error) {
	return t.gatewayClient.WaitTx(ctx, txHash, interval)
}

// ChainID returns the network chain identifier.
// This method delegates to the underlying GatewayClient's ChainID method.
//
// The chain ID is used to ensure transactions are sent to the correct network
// and prevent replay attacks across different networks.
func (t *HTTPTransport) ChainID() string {
	return t.gatewayClient.ChainID()
}

// Signer returns the cryptographic signer used for transaction authentication.
// This method delegates to the underlying GatewayClient's Signer method.
//
// Returns nil if no signer is configured (read-only mode).
func (t *HTTPTransport) Signer() auth.Signer {
	return t.gatewayClient.Signer()
}
