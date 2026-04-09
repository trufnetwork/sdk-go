package tnclient

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/kwil-db/core/gatewayclient"
	"github.com/trufnetwork/kwil-db/core/log"
	rpcclient "github.com/trufnetwork/kwil-db/core/rpc/client"
	adminclient "github.com/trufnetwork/kwil-db/core/rpc/client/admin/jsonrpc"
	kwilType "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/kwil-db/node/types"
	tn_api "github.com/trufnetwork/sdk-go/core/contractsapi"
	"github.com/trufnetwork/sdk-go/core/logging"
	clientType "github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"go.uber.org/zap"
)

type Client struct {
	signer    auth.Signer `validate:"required"`
	logger    *log.Logger
	transport Transport `validate:"required"`
	// admin is an optional kwil-db admin JSON-RPC client used by
	// LoadLocalActions() to call tn_local.* methods on the node's admin
	// server (port 8485). nil unless configured via WithAdmin(). Unused by
	// all other Client methods, which go through the gateway.
	admin *adminclient.Client
	// adminErr captures any URL-parsing or validation error from WithAdmin
	// so it can be surfaced at Validate() time rather than silently
	// swallowed inside the functional-option closure.
	adminErr error
}

var _ clientType.Client = (*Client)(nil)

type Option func(*Client)

func NewClient(ctx context.Context, provider string, options ...Option) (*Client, error) {
	c := &Client{}

	// Apply user-provided options
	for _, option := range options {
		option(c)
	}

	// Fail fast if an option (e.g. WithAdmin) already recorded an error —
	// no point building the default transport for a Client we'll reject.
	if c.adminErr != nil {
		return nil, c.adminErr
	}

	// Create default HTTPTransport if no transport was provided via options
	if c.transport == nil {
		var logger log.Logger
		if c.logger != nil {
			logger = *c.logger
		}

		transport, err := NewHTTPTransport(ctx, provider, c.signer, logger)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create default HTTP transport")
		}
		c.transport = transport
	}

	// Validate the client
	if err := c.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	return c, nil
}

func (c *Client) Validate() error {
	if c.adminErr != nil {
		return c.adminErr
	}
	validate := validator.New()
	return validate.Struct(c)
}

func WithSigner(signer auth.Signer) Option {
	return func(c *Client) {
		c.signer = signer
	}
}

func WithLogger(logger log.Logger) Option {
	return func(c *Client) {
		c.logger = &logger
	}
}

// WithTransport configures the client to use a custom transport implementation.
//
// By default, the SDK uses HTTPTransport which communicates via standard net/http.
// This option allows you to substitute a different transport (e.g., for Chainlink CRE,
// mock testing, or custom protocols).
//
// Example:
//
//	transport, _ := NewHTTPTransport(ctx, endpoint, signer, logger)
//	client, err := NewClient(ctx, endpoint,
//	    tnclient.WithTransport(transport),
//	)
//
// Note: When using WithTransport, the provider URL passed to NewClient is ignored
// since the transport is already configured.
func WithTransport(transport Transport) Option {
	return func(c *Client) {
		c.transport = transport
	}
}

// WithAdmin configures the client to also talk to a Kwil admin JSON-RPC
// server (port 8485 by default), enabling LoadLocalActions(). The admin
// server is separate from the gateway used by every other SDK method —
// it exposes the node operator's local stream operations via the
// tn_local extension.
//
// adminURL is the base URL of the admin server — typically
// "http://127.0.0.1:8485" for loopback TCP. For unix sockets use
// "http://unix" with an HTTP client that dials the socket; see
// kwil-db's rpcclient.WithHTTPClient() for passing a custom client.
//
// opts are passed through to kwil-db's admin client unchanged. Common
// options:
//
//   - rpcclient.WithPass("admin-password") for basic auth
//   - rpcclient.WithHTTPClient(customClient) for mTLS or unix sockets
//   - rpcclient.WithLogger(logger) for debug logging
//
// Example — local node with basic auth:
//
//	client, err := tnclient.NewClient(ctx, gatewayURL,
//	    tnclient.WithSigner(signer),
//	    tnclient.WithAdmin("http://127.0.0.1:8485",
//	        rpcclient.WithPass("admin-secret")),
//	)
//	if err != nil { /* ... */ }
//	local, err := client.LoadLocalActions()
//
// If adminURL is malformed or lacks a scheme/host, the error is stored
// in c.adminErr and returned immediately by NewClient (before transport
// construction or Validate). The functional-option shape (no return
// value) is preserved by deferring the error to the NewClient caller.
func WithAdmin(adminURL string, opts ...rpcclient.RPCClientOpts) Option {
	return func(c *Client) {
		// Reset both fields so repeated calls (e.g. duplicate options)
		// don't leave stale state from a prior invocation.
		c.admin = nil
		c.adminErr = nil

		u, err := parseAdminURL(adminURL)
		if err != nil {
			c.adminErr = err
			return
		}
		c.admin = adminclient.NewClient(u, opts...)
	}
}

func (c *Client) GetSigner() auth.Signer {
	return c.transport.Signer()
}

func (c *Client) WaitForTx(ctx context.Context, txHash kwilType.Hash, interval time.Duration) (*kwilType.TxQueryResponse, error) {
	return c.transport.WaitTx(ctx, txHash, interval)
}

// GetKwilClient returns the underlying GatewayClient if using HTTPTransport.
//
// This method provides direct access to the GatewayClient for advanced use cases
// that require low-level control. For most scenarios, prefer using the Client's
// high-level methods (ListStreams, DeployStream, etc.) which are transport-agnostic.
//
// Returns nil if using a non-HTTP transport (e.g., CRE transport).
//
// Example:
//
//	if gwClient := client.GetKwilClient(); gwClient != nil {
//	    // Direct GatewayClient access for advanced use cases
//	    result, err := gwClient.Call(ctx, "", "custom_action", args)
//	}
func (c *Client) GetKwilClient() *gatewayclient.GatewayClient {
	if httpTransport, ok := c.transport.(*HTTPTransport); ok {
		return httpTransport.gatewayClient
	}
	return nil
}

func (c *Client) DeployStream(ctx context.Context, streamId util.StreamId, streamType clientType.StreamType) (types.Hash, error) {
	// For HTTP transport, use the existing implementation (backwards compatible)
	// For custom transports (CRE, etc.), use transport.Execute directly
	if httpTransport, ok := c.transport.(*HTTPTransport); ok {
		return tn_api.DeployStream(ctx, tn_api.DeployStreamInput{
			StreamId:   streamId,
			StreamType: streamType,
			KwilClient: httpTransport.gatewayClient,
		})
	}
	// Use transport.Execute directly for custom transports
	return c.transport.Execute(ctx, "", "create_stream", [][]any{{
		streamId.String(),
		streamType.String(),
	}})
}

func (c *Client) DestroyStream(ctx context.Context, streamId util.StreamId) (types.Hash, error) {
	// For HTTP transport, use the existing implementation (backwards compatible)
	// For custom transports (CRE, etc.), use transport.Execute directly
	if httpTransport, ok := c.transport.(*HTTPTransport); ok {
		return tn_api.DestroyStream(ctx, tn_api.DestroyStreamInput{
			StreamId:   streamId,
			KwilClient: httpTransport.gatewayClient,
		})
	}
	// Use transport.Execute directly for custom transports
	// Derive address from signer for delete_stream call
	addr, _ := auth.EthSecp256k1Authenticator{}.Identifier(c.signer.CompactID())
	return c.transport.Execute(ctx, "", "delete_stream", [][]any{{
		addr,
		streamId.String(),
	}})
}

func (c *Client) LoadActions() (clientType.IAction, error) {
	// For HTTP transport, use the full-featured GatewayClient implementation
	// For custom transports (CRE, etc.), use the minimal transport-aware implementation
	if httpTransport, ok := c.transport.(*HTTPTransport); ok {
		return tn_api.LoadAction(tn_api.NewActionOptions{
			Client: httpTransport.gatewayClient,
		})
	}
	// Return transport-aware implementation for custom transports
	return &TransportAction{transport: c.transport}, nil
}

func (c *Client) LoadPrimitiveActions() (clientType.IPrimitiveAction, error) {
	// For HTTP transport, use the full-featured GatewayClient implementation
	// For custom transports (CRE, etc.), use the minimal transport-aware implementation
	if httpTransport, ok := c.transport.(*HTTPTransport); ok {
		return tn_api.LoadPrimitiveActions(tn_api.NewActionOptions{
			Client: httpTransport.gatewayClient,
		})
	}
	// Return transport-aware implementation for custom transports
	return &TransportPrimitiveAction{
		TransportAction: TransportAction{transport: c.transport},
	}, nil
}

func (c *Client) LoadComposedActions() (clientType.IComposedAction, error) {
	return tn_api.LoadComposedActions(tn_api.NewActionOptions{
		Client: c.GetKwilClient(),
	})
}

func (c *Client) LoadRoleManagementActions() (clientType.IRoleManagement, error) {
	return tn_api.LoadRoleManagementActions(tn_api.NewRoleManagementOptions{
		Client: c.GetKwilClient(),
	})
}

func (c *Client) LoadAttestationActions() (clientType.IAttestationAction, error) {
	return tn_api.LoadAttestationActions(tn_api.AttestationActionOptions{
		Client: c.GetKwilClient(),
	})
}

// LoadLocalActions loads the interface for local (off-chain, node-only)
// stream operations. Requires the client to have been constructed with
// the WithAdmin() option — returns an error otherwise.
//
// Local streams are stored on a single node, bypass consensus, incur no
// transaction fees, and are always owned by the node operator (the server
// derives the data_provider from the node's secp256k1 key). See the
// LocalActions interface in core/types/local_actions_types.go for method
// documentation.
//
// Example:
//
//	local, err := client.LoadLocalActions()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	err = local.CreateStream(ctx, types.LocalCreateStreamInput{
//	    StreamID:   "st00000000000000000000000000demo",
//	    StreamType: types.StreamTypePrimitive,
//	})
func (c *Client) LoadLocalActions() (clientType.ILocalActions, error) {
	if c.adminErr != nil {
		return nil, errors.Wrap(c.adminErr, "LoadLocalActions: invalid admin configuration")
	}
	if c.admin == nil {
		return nil, errors.New("LoadLocalActions requires tnclient.WithAdmin() option; " +
			"construct the client with the admin URL to enable local stream operations")
	}
	return tn_api.LoadLocalActions(tn_api.LocalActionsOptions{Admin: c.admin})
}

// LoadTransactionActions loads the transaction ledger query interface
//
// Example:
//
//	txActions, err := client.LoadTransactionActions()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	txEvent, err := txActions.GetTransactionEvent(ctx, ...)
func (c *Client) LoadTransactionActions() (clientType.ITransactionAction, error) {
	return tn_api.LoadTransactionActions(tn_api.TransactionActionOptions{
		Client: c.GetKwilClient(),
	})
}

// LoadOrderBook loads the prediction market order book interface
//
// Example:
//
//	orderBook, err := client.LoadOrderBook()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	market, err := orderBook.GetMarketInfo(ctx, types.GetMarketInfoInput{QueryID: 1})
func (c *Client) LoadOrderBook() (clientType.IOrderBook, error) {
	if httpTransport, ok := c.transport.(*HTTPTransport); ok {
		return tn_api.LoadOrderBook(tn_api.NewOrderBookOptions{
			Client: httpTransport.gatewayClient,
		})
	}
	return nil, errors.New("OrderBook is only available with HTTP transport")
}

func (c *Client) OwnStreamLocator(streamId util.StreamId) clientType.StreamLocator {
	return clientType.StreamLocator{
		StreamId:     streamId,
		DataProvider: c.Address(),
	}
}

func (c *Client) Address() util.EthereumAddress {
	addr, err := auth.EthSecp256k1Authenticator{}.Identifier(c.transport.Signer().CompactID())
	if err != nil {
		// should never happen
		logging.Logger.Panic("failed to get address from signer", zap.Error(err))
	}
	address, err := util.NewEthereumAddressFromString(addr)
	if err != nil {
		logging.Logger.Panic("failed to create address from string", zap.Error(err))
	}
	return address
}

// BatchDeployStreams deploys multiple streams (primitive and composed).
// It returns the transaction hash of the batch operation.
func (c *Client) BatchDeployStreams(ctx context.Context, streamDefs []clientType.StreamDefinition) (kwilType.Hash, error) {
	// Assuming SchemaName for "create_streams" is obtained from somewhere or is a known constant.
	// For now, using an empty string as a placeholder if it's a root/global procedure.
	schemaName := "" // Or c.config.SchemaName, etc.

	return tn_api.BatchDeployStreams(ctx, tn_api.BatchDeployStreamsInput{
		KwilClient:  c.GetKwilClient(),
		Definitions: streamDefs,
		SchemaName:  schemaName,
	})
}

// BatchStreamExists checks for the existence of multiple streams.
func (c *Client) BatchStreamExists(ctx context.Context, streams []clientType.StreamLocator) ([]clientType.StreamExistsResult, error) {
	actions, err := c.LoadActions()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actions for BatchStreamExists")
	}
	return actions.BatchStreamExists(ctx, streams)
}

// BatchFilterStreamsByExistence filters a list of streams based on their existence in the database.
// Use this instead of BatchStreamExists if you want less data returned.
func (c *Client) BatchFilterStreamsByExistence(ctx context.Context, streams []clientType.StreamLocator, returnExisting bool) ([]clientType.StreamLocator, error) {
	actions, err := c.LoadActions()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actions for BatchFilterStreamsByExistence")
	}
	return actions.BatchFilterStreamsByExistence(ctx, streams, returnExisting)
}

// GetHistory retrieves the transaction history for a wallet on a specific bridge
func (c *Client) GetHistory(ctx context.Context, input clientType.GetHistoryInput) ([]clientType.BridgeHistory, error) {
	actions, err := c.LoadActions()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actions for GetHistory")
	}
	return actions.GetHistory(ctx, input)
}

// GetWalletBalance retrieves the wallet balance for a specific bridge instance
func (c *Client) GetWalletBalance(ctx context.Context, bridgeIdentifier string, walletAddress string) (string, error) {
	actions, err := c.LoadActions()
	if err != nil {
		return "", errors.Wrap(err, "failed to load actions for GetWalletBalance")
	}
	return actions.GetWalletBalance(ctx, bridgeIdentifier, walletAddress)
}

// Withdraw performs a withdrawal operation by bridging tokens from TN to a destination chain
func (c *Client) Withdraw(ctx context.Context, bridgeIdentifier string, amount string, recipient string) (string, error) {
	actions, err := c.LoadActions()
	if err != nil {
		return "", errors.Wrap(err, "failed to load actions for Withdraw")
	}
	return actions.Withdraw(ctx, bridgeIdentifier, amount, recipient)
}

// GetWithdrawalProof retrieves the proofs and signatures needed to claim a withdrawal on EVM.
func (c *Client) GetWithdrawalProof(ctx context.Context, input clientType.GetWithdrawalProofInput) ([]clientType.WithdrawalProof, error) {
	actions, err := c.LoadActions()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actions for GetWithdrawalProof")
	}
	return actions.GetWithdrawalProof(ctx, input)
}
