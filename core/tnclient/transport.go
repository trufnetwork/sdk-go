package tnclient

import (
	"context"
	"time"

	clientType "github.com/trufnetwork/kwil-db/core/client/types"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/kwil-db/core/types"
)

// Transport abstracts the communication layer for TRUF Network operations.
// This interface allows using different transport implementations without changing SDK code.
//
// The default implementation (HTTPTransport) uses standard net/http via kwil-db's
// GatewayClient. Custom implementations can use different protocols such as:
//   - Chainlink CRE's HTTP client (for workflows in CRE environments)
//   - gRPC or other RPC protocols
//   - Mock implementations for testing
//
// Example custom transport usage:
//
//	type MyTransport struct { ... }
//
//	func (t *MyTransport) Call(ctx context.Context, namespace string, action string, inputs []any) (*types.CallResult, error) {
//	    // Custom implementation
//	    return &types.CallResult{...}, nil
//	}
//
//	// Use custom transport
//	client, err := tnclient.NewClient(ctx, endpoint,
//	    tnclient.WithSigner(signer),
//	    tnclient.WithTransport(myTransport),
//	)
//
// All SDK methods internally use the Transport interface, making the entire SDK
// adaptable to different execution environments without code changes.
type Transport interface {
	// Call executes a read-only action and returns results.
	// Namespace is typically "" for global actions.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - namespace: Schema namespace (typically "" for global procedures)
	//   - action: The procedure/action name to call
	//   - inputs: Action input parameters as a slice of any
	//
	// Returns:
	//   - CallResult containing the query results
	//   - Error if the call fails or returns an application error
	Call(ctx context.Context, namespace string, action string, inputs []any) (*types.CallResult, error)

	// Execute performs a write action and returns the transaction hash.
	// Inputs is a slice of argument arrays for batch operations.
	// Options can include nonce, fee, and other transaction parameters.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - namespace: Schema namespace (typically "" for global procedures)
	//   - action: The procedure/action name to execute
	//   - inputs: Batch of input parameter arrays ([][]any for multiple calls)
	//   - opts: Optional transaction options (nonce, fee, etc.)
	//
	// Returns:
	//   - Transaction hash for tracking the transaction
	//   - Error if the execution fails
	Execute(ctx context.Context, namespace string, action string, inputs [][]any, opts ...clientType.TxOpt) (types.Hash, error)

	// WaitTx polls for transaction confirmation with the specified interval.
	// Blocks until the transaction is confirmed or the context is cancelled.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - txHash: The transaction hash to wait for
	//   - interval: Polling interval between status checks
	//
	// Returns:
	//   - TxQueryResponse containing the transaction status and result
	//   - Error if the wait fails or transaction is rejected
	WaitTx(ctx context.Context, txHash types.Hash, interval time.Duration) (*types.TxQueryResponse, error)

	// ChainID returns the network chain identifier.
	// This is used to ensure transactions are sent to the correct network.
	//
	// Returns:
	//   - Chain ID string (e.g., "truf-mainnet")
	ChainID() string

	// Signer returns the cryptographic signer used for transaction authentication.
	// Returns nil if no signer is configured (read-only mode).
	//
	// Returns:
	//   - Signer instance for transaction signing
	Signer() auth.Signer
}
