package tnclient

import (
	"net/url"

	"github.com/pkg/errors"
	rpcclient "github.com/trufnetwork/kwil-db/core/rpc/client"
	adminclient "github.com/trufnetwork/kwil-db/core/rpc/client/admin/jsonrpc"
	tn_api "github.com/trufnetwork/sdk-go/core/contractsapi"
	clientType "github.com/trufnetwork/sdk-go/core/types"
)

// NewLocalClient constructs a standalone client for the tn_local admin API
// only. Use this when the caller does not need any of the full tnclient's
// gateway-backed methods (DeployStream, LoadOrderBook, etc.) and only wants
// to read/write local streams on a node it operates.
//
// Unlike NewClient, this constructor does NOT require an auth.Signer — the
// admin API uses its own auth (unix socket / mTLS / basic password) and has
// no concept of Ethereum signatures. Callers who need both on-chain and
// local operations should use NewClient(..., WithAdmin(adminURL, ...))
// and call client.LoadLocalActions() instead.
//
// adminURL is the base URL of the admin server, e.g. "http://127.0.0.1:8485"
// for loopback TCP. For unix sockets or mTLS, pass a custom *http.Client via
// rpcclient.WithHTTPClient().
//
// opts are forwarded unchanged to kwil-db's admin client. Common options:
//
//   - rpcclient.WithPass("admin-secret") for HTTP basic auth
//   - rpcclient.WithHTTPClient(customClient) for mTLS / unix sockets
//   - rpcclient.WithLogger(logger) for debug logging
//
// Example — local node with basic auth:
//
//	local, err := tnclient.NewLocalClient("http://127.0.0.1:8485",
//	    rpcclient.WithPass("admin-secret"))
//	if err != nil { /* ... */ }
//	err = local.CreateStream(ctx, types.LocalCreateStreamInput{
//	    StreamID:   "st00000000000000000000000000demo",
//	    StreamType: types.StreamTypePrimitive,
//	})
func NewLocalClient(adminURL string, opts ...rpcclient.RPCClientOpts) (clientType.ILocalActions, error) {
	if adminURL == "" {
		return nil, errors.New("adminURL is required")
	}
	u, err := url.Parse(adminURL)
	if err != nil {
		return nil, errors.Wrap(err, "invalid adminURL")
	}
	admin := adminclient.NewClient(u, opts...)
	return tn_api.LoadLocalActions(tn_api.LocalActionsOptions{Admin: admin})
}
