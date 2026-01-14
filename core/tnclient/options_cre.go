//go:build wasip1

package tnclient

import (
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
)

// WithCRETransport configures the client to use Chainlink CRE's HTTP client.
//
// This option is used when running the SDK in Chainlink Runtime Environment (CRE)
// workflows. CRE provides its own HTTP client with consensus and caching features
// that must be used instead of standard net/http.
//
// The runtime parameter is provided by the CRE workflow execution context via
// cre.RunInNodeMode(). The endpoint should match the TRUF.NETWORK gateway URL.
//
// Example usage in CRE workflow:
//
//	func onCronTrigger(config *Config, runtime cre.Runtime, trigger *cron.Payload) (*Result, error) {
//	    logger := runtime.Logger()
//
//	    return cre.RunInNodeMode(config, runtime,
//	        func(config *Config, nodeRuntime cre.NodeRuntime) (*Result, error) {
//	            // Create TRUF client with CRE transport
//	            client, err := tnclient.NewClient(context.Background(), config.TRUFEndpoint,
//	                tnclient.WithSigner(signer),
//	                tnclient.WithCRETransport(nodeRuntime, config.TRUFEndpoint),  // ← CRE transport
//	            )
//	            if err != nil {
//	                logger.Error("Failed to create TRUF client", "error", err)
//	                return nil, err
//	            }
//
//	            // Use SDK normally - all methods work!
//	            actions, err := client.LoadActions()
//	            if err != nil {
//	                logger.Error("Failed to load actions", "error", err)
//	                return nil, err
//	            }
//
//	            fromTime := int(time.Now().Add(-24 * time.Hour).Unix())
//	            toTime := int(time.Now().Unix())
//
//	            result, err := actions.GetRecord(context.Background(), types.GetRecordInput{
//	                DataProvider: "0x1234...",
//	                StreamId:     "stai0000000000000000000000000000",
//	                From:         &fromTime,
//	                To:           &toTime,
//	            })
//	            if err != nil {
//	                logger.Error("GetRecord failed", "error", err)
//	                return nil, err
//	            }
//
//	            logger.Info("Successfully fetched records", "count", len(result.Results))
//	            return &Result{Records: result.Results}, nil
//	        },
//	        cre.ConsensusAggregationFromTags[*Result](),
//	    ).Await()
//	}
//
// Parameters:
//   - runtime: CRE NodeRuntime from the workflow execution context
//   - endpoint: TRUF.NETWORK gateway URL (e.g., "https://gateway.example.com")
//
// Note: When using WithCRETransport, you must also provide WithSigner if you need
// to perform write operations (InsertRecords, DeployStream, etc.). The signer should
// be created before the CRE workflow execution.
//
// Note: The provider URL passed to NewClient is ignored when using WithCRETransport,
// since the endpoint is provided directly to this option.
func WithCRETransport(runtime cre.NodeRuntime, endpoint string) Option {
	return func(c *Client) {
		// Note: Transport is created immediately with the current signer (if set)
		// If WithSigner is applied after this option, the signer won't be available yet
		// For guaranteed signer availability, use WithCRETransportAndSigner instead
		c.transport, _ = NewCRETransport(runtime, endpoint, c.signer)
	}
}

// WithCRETransportAndSigner is a convenience function that combines WithSigner
// and WithCRETransport in the correct order.
//
// This ensures the signer is set before creating the CRE transport, which is
// necessary for write operations.
//
// Example:
//
//	client, err := tnclient.NewClient(ctx, endpoint,
//	    tnclient.WithCRETransportAndSigner(nodeRuntime, endpoint, signer),
//	)
//
// This is equivalent to:
//
//	client, err := tnclient.NewClient(ctx, endpoint,
//	    tnclient.WithSigner(signer),
//	    tnclient.WithCRETransport(nodeRuntime, endpoint),
//	)
func WithCRETransportAndSigner(runtime cre.NodeRuntime, endpoint string, signer auth.Signer) Option {
	return func(c *Client) {
		// Set signer first
		c.signer = signer

		// Then create CRE transport with the signer
		c.transport, _ = NewCRETransport(runtime, endpoint, signer)
	}
}

func WithCRETransportAndSignerWithHTTPCache(runtime cre.NodeRuntime, endpoint string, signer auth.Signer, cacheCfg *CREHTTPCacheConfig) Option {
	return func(c *Client) {
		// Set signer first
		c.signer = signer
		// Then create CRE transport with the signer and HTTP cache
		c.transport, _ = NewCRETransportWithHTTPCache(runtime, endpoint, signer, cacheCfg)
	}
}
