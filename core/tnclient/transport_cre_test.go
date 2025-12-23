//go:build wasip1

package tnclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Note: These are basic structural tests for CRE transport.
// Full integration tests require running in actual CRE environment.
// See the examples/cre_integration/ directory for complete working examples.

func TestNewCRETransport(t *testing.T) {
	// Note: We cannot create a real NodeRuntime outside of CRE environment,
	// so this test just verifies the function signature and basic structure.

	t.Run("constructor_exists", func(t *testing.T) {
		// This test just verifies that the NewCRETransport function exists
		// and has the expected signature.
		// Actual testing requires CRE simulation environment.

		// Verify the function is not nil
		assert.NotNil(t, NewCRETransport)
	})
}

func TestCRETransport_Implements_Transport_Interface(t *testing.T) {
	// This compile-time check verifies that CRETransport implements Transport
	// The var _ Transport = (*CRETransport)(nil) line in transport_cre.go
	// ensures this at compile time, but we include this test for documentation.

	t.Run("implements_interface", func(t *testing.T) {
		// If this compiles, the interface is implemented
		var _ Transport = (*CRETransport)(nil)
	})
}

func TestWithCRETransport(t *testing.T) {
	t.Run("option_exists", func(t *testing.T) {
		// Verify the WithCRETransport option function exists
		assert.NotNil(t, WithCRETransport)
	})

	t.Run("option_signature", func(t *testing.T) {
		// Verify the function returns an Option
		// This test documents the expected signature
		var _ Option = WithCRETransport(nil, "http://example.com")
	})
}

func TestWithCRETransportAndSigner(t *testing.T) {
	t.Run("option_exists", func(t *testing.T) {
		// Verify the WithCRETransportAndSigner option function exists
		assert.NotNil(t, WithCRETransportAndSigner)
	})

	t.Run("option_signature", func(t *testing.T) {
		// Verify the function returns an Option
		var _ Option = WithCRETransportAndSigner(nil, "http://example.com", nil)
	})
}

// Example documentation for CRE usage

func ExampleWithCRETransport() {
	// This example shows how to use WithCRETransport in a CRE workflow
	// Note: This code will only run in actual CRE environment

	/*
		import (
			"context"
			"time"

			"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
			"github.com/smartcontractkit/cre-sdk-go/cre"
			"github.com/trufnetwork/sdk-go/core/tnclient"
			"github.com/trufnetwork/sdk-go/core/types"
		)

		type Config struct {
			TRUFEndpoint string `json:"trufEndpoint"`
			Schedule     string `json:"schedule"`
		}

		type Result struct {
			Records []types.StreamResult `json:"records"`
		}

		func onCronTrigger(config *Config, runtime cre.Runtime, trigger *cron.Payload) (*Result, error) {
			logger := runtime.Logger()

			return cre.RunInNodeMode(config, runtime,
				func(config *Config, nodeRuntime cre.NodeRuntime) (*Result, error) {
					// Create TN client with CRE transport
					client, err := tnclient.NewClient(context.Background(), config.TRUFEndpoint,
						tnclient.WithCRETransport(nodeRuntime, config.TRUFEndpoint),
					)
					if err != nil {
						logger.Error("Failed to create TRUF client", "error", err)
						return nil, err
					}

					// Load actions
					actions, err := client.LoadActions()
					if err != nil {
						logger.Error("Failed to load actions", "error", err)
						return nil, err
					}

					// Get records
					fromTime := int(time.Now().Add(-24 * time.Hour).Unix())
					toTime := int(time.Now().Unix())

					result, err := actions.GetRecord(context.Background(), types.GetRecordInput{
						DataProvider: "0x1234...",
						StreamId:     "stai0000000000000000000000000000",
						From:         &fromTime,
						To:           &toTime,
					})
					if err != nil {
						logger.Error("GetRecord failed", "error", err)
						return nil, err
					}

					logger.Info("Fetched records", "count", len(result.Results))
					return &Result{Records: result.Results}, nil
				},
				cre.ConsensusAggregationFromTags[*Result](),
			).Await()
		}
	*/
}

func ExampleWithCRETransportAndSigner() {
	// This example shows how to use WithCRETransportAndSigner for write operations
	// Note: This code will only run in actual CRE environment

	/*
		import (
			"context"
			"time"

			"github.com/smartcontractkit/cre-sdk-go/cre"
			"github.com/trufnetwork/kwil-db/core/crypto/auth"
			"github.com/trufnetwork/sdk-go/core/tnclient"
			"github.com/trufnetwork/sdk-go/core/types"
		)

		type Config struct {
			TRUFEndpoint string `json:"trufEndpoint"`
			PrivateKey   string `json:"privateKey"` // Should be from secrets in production
		}

		func deployStream(config *Config, nodeRuntime cre.NodeRuntime) error {
			// Create signer from private key
			signer, err := auth.EthSecp256k1SignerFromKey([]byte(config.PrivateKey))
			if err != nil {
				return err
			}

			// Create client with CRE transport and signer
			client, err := tnclient.NewClient(context.Background(), config.TRUFEndpoint,
				tnclient.WithCRETransportAndSigner(nodeRuntime, config.TRUFEndpoint, signer),
			)
			if err != nil {
				return err
			}

			// Deploy a stream (write operation)
			txHash, err := client.DeployStream(context.Background(),
				"stai0000000000000000000000000000",
				types.PRIMITIVE,
			)
			if err != nil {
				return err
			}

			// Wait for transaction confirmation
			_, err = client.WaitForTx(context.Background(), txHash, 5*time.Second)
			return err
		}
	*/
}
