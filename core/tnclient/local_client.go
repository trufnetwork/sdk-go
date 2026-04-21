package tnclient

import (
	"crypto/ecdsa"
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	rpcclient "github.com/trufnetwork/kwil-db/core/rpc/client"
	adminclient "github.com/trufnetwork/kwil-db/core/rpc/client/admin/jsonrpc"
	tn_api "github.com/trufnetwork/sdk-go/core/contractsapi"
	clientType "github.com/trufnetwork/sdk-go/core/types"
)

// parseAdminURL parses and validates an admin server URL. Returns an error
// if the URL is unparseable or missing a scheme/host, which would produce a
// broken admin client that silently fails at request time.
func parseAdminURL(adminURL string) (*url.URL, error) {
	if adminURL == "" {
		return nil, errors.New("adminURL is required")
	}
	u, err := url.Parse(adminURL)
	if err != nil {
		return nil, errors.Wrap(err, "invalid adminURL")
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("adminURL must be an absolute URL with scheme and host (e.g. http://127.0.0.1:8485), got %q", adminURL)
	}
	return u, nil
}

// NewLocalClient constructs a standalone client for the tn_local admin API
// only. Use this when the caller does not need any of the full tnclient's
// gateway-backed methods (DeployStream, LoadOrderBook, etc.) and only wants
// to read/write local streams on a node it operates.
//
// Unlike NewClient, this constructor does NOT require an auth.Signer — the
// admin server handles its own transport auth (unix socket by default, mTLS
// for remote TCP). tn_local itself has no auth concept — if you can reach
// the admin server, you can operate on local streams. Callers who need both
// on-chain and local operations should use NewClient(..., WithAdmin(adminURL))
// and call client.LoadLocalActions() instead.
//
// adminURL is the base URL of the admin server, e.g. "http://127.0.0.1:8485"
// for loopback TCP.
//
// opts are forwarded unchanged to kwil-db's admin client. Common options:
//
//   - rpcclient.WithHTTPClient(customClient) for mTLS or unix sockets
//   - rpcclient.WithLogger(logger) for debug logging
//
// Example — local node (default, no auth needed):
//
//	local, err := tnclient.NewLocalClient("http://127.0.0.1:8485")
//	if err != nil { /* ... */ }
//	err = local.CreateStream(ctx, types.LocalCreateStreamInput{
//	    StreamID:   "st00000000000000000000000000demo",
//	    StreamType: types.StreamTypePrimitive,
//	})
func NewLocalClient(adminURL string, opts ...rpcclient.RPCClientOpts) (clientType.ILocalActions, error) {
	u, err := parseAdminURL(adminURL)
	if err != nil {
		return nil, err
	}
	admin := adminclient.NewClient(u, opts...)
	return tn_api.LoadLocalActions(tn_api.LocalActionsOptions{Admin: admin})
}

// NewLocalClientWithSigner is NewLocalClient with an operator secp256k1
// private key for signing tn_local admin requests. Use this when the target
// node has require_signature=true on the tn_local extension. The SDK
// attaches a `_auth` envelope to every call; the server recovers the
// signing address and rejects calls that don't match its operator address.
//
// Pass nil for priv to behave identically to NewLocalClient (no signing,
// only nodes with the flag off will accept the request).
//
// Example:
//
//	priv, _ := ethcrypto.HexToECDSA(operatorHexKey)
//	local, err := tnclient.NewLocalClientWithSigner("http://127.0.0.1:8485", priv)
//	if err != nil { /* ... */ }
//	err = local.CreateStream(ctx, types.LocalCreateStreamInput{
//	    StreamID:   "st00000000000000000000000000demo",
//	    StreamType: types.StreamTypePrimitive,
//	})
func NewLocalClientWithSigner(adminURL string, priv *ecdsa.PrivateKey, opts ...rpcclient.RPCClientOpts) (clientType.ILocalActions, error) {
	u, err := parseAdminURL(adminURL)
	if err != nil {
		return nil, err
	}
	admin := adminclient.NewClient(u, opts...)
	return tn_api.LoadLocalActions(tn_api.LocalActionsOptions{Admin: admin, Signer: priv})
}
