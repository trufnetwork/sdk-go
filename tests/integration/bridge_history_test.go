package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

func TestBridgeHistory(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	fixture := NewServerFixture(t)
	err := fixture.Setup()
	require.NoError(t, err, "Failed to setup server fixture")
	t.Cleanup(func() {
		fixture.Teardown()
	})

	// Create SDK client
	wallet, err := kwilcrypto.Secp256k1PrivateKeyFromHex(AnonWalletPK)
	require.NoError(t, err, "failed to parse wallet private key")

	tnClient, err := tnclient.NewClient(ctx, TestKwilProvider,
		tnclient.WithSigner(auth.GetUserSigner(wallet)))
	require.NoError(t, err, "failed to create client")

	// Test 1: GetHistory - Bridge Not Found (Verification of Call Mechanism)
	// Since we cannot easily deploy a full bridge with Ethereum events in this test suite,
	// we verify that the SDK correctly constructs the call and receives the node's error.
	t.Run("GetHistory_BridgeNotFound", func(t *testing.T) {
		bridgeID := "non_existent_bridge"
		walletAddr := "0x1234567890123456789012345678901234567890"
		
		_, err := tnClient.GetHistory(ctx, types.GetHistoryInput{
			BridgeIdentifier: bridgeID,
			Wallet:           walletAddr,
		})

		assert.Error(t, err, "should return error for non-existent bridge")
		// The error from the node should indicate that the procedure (action) was not found.
		// The exact error message depends on the node's response format for missing actions.
		// Usually it says something about "procedure ... not found" or "schema ... not found".
		// We just verify we got an error from the backend, confirming the path.
		assert.NotEmpty(t, err.Error(), "error message should not be empty")
		t.Logf("Received expected error: %v", err)
	})

	// Test 2: GetHistory - Input Validation
	t.Run("GetHistory_InputValidation", func(t *testing.T) {
		// Empty Bridge ID
		_, err := tnClient.GetHistory(ctx, types.GetHistoryInput{
			BridgeIdentifier: "",
			Wallet:           "0x123",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bridge identifier is required")

		// Empty Wallet
		_, err = tnClient.GetHistory(ctx, types.GetHistoryInput{
			BridgeIdentifier: "bridge",
			Wallet:           "",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wallet address is required")
	})
}
