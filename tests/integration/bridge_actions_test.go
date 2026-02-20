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

func TestBridgeActions(t *testing.T) {
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

	// Test 1: GetWalletBalance
	t.Run("GetWalletBalance", func(t *testing.T) {
		// Use a non-existent bridge to verify the SDK makes the call correctly
		// and the node receives it (even if it errors).
		bridgeID := "non_existent_bridge"
		walletAddr := "0x1234567890123456789012345678901234567890"

		_, err := tnClient.GetWalletBalance(ctx, bridgeID, walletAddr)
		
		// We expect an error because the bridge doesn't exist, but getting *an* error 
		// from the node proves the SDK method is wired up and sending the request.
		assert.Error(t, err, "should return error for non-existent bridge")
		t.Logf("GetWalletBalance Response: %v", err)
	})

	// Test 2: Withdraw
	// Note: Withdraw is an async write operation (Execute). The node might accept the transaction into the mempool
	// returning a hash, even if the action name is invalid (validation happens at block execution).
	// Therefore, we assert that we get EITHER an error OR a valid transaction hash.
	t.Run("Withdraw", func(t *testing.T) {
		bridgeID := "non_existent_bridge"
		amount := "1000000000000000000" // 1 token
		recipient := "0x1234567890123456789012345678901234567890"

		hash, err := tnClient.Withdraw(ctx, bridgeID, amount, recipient)

		if err != nil {
			t.Logf("Withdraw returned error (acceptable): %v", err)
		} else {
			require.NotEmpty(t, hash, "must return a non-empty transaction hash when no error")
			t.Logf("Withdraw returned hash (acceptable): %v", hash)
		}
	})

	// Test 3: GetWithdrawalProof
	t.Run("GetWithdrawalProof", func(t *testing.T) {
		bridgeID := "non_existent_bridge"
		walletAddr := "0x1234567890123456789012345678901234567890"

		_, err := tnClient.GetWithdrawalProof(ctx, types.GetWithdrawalProofInput{
			BridgeIdentifier: bridgeID,
			Wallet:           walletAddr,
		})

		assert.Error(t, err, "should return error for non-existent bridge")
		t.Logf("GetWithdrawalProof Response: %v", err)
	})

	// Test 4: Input Validation
	t.Run("InputValidation", func(t *testing.T) {
		// Withdraw Empty Bridge
		_, err := tnClient.Withdraw(ctx, "", "100", "0x123")
		require.Error(t, err) // Use require to stop if nil, preventing panic
		assert.Contains(t, err.Error(), "bridge identifier is required")

		// Withdraw Invalid Amount
		_, err = tnClient.Withdraw(ctx, "bridge", "invalid_amount", "0x123")
		require.Error(t, err)
		// Error might come from the node or SDK decimal parsing
		
		// GetWithdrawalProof Empty Bridge
		_, err = tnClient.GetWithdrawalProof(ctx, types.GetWithdrawalProofInput{
			BridgeIdentifier: "",
			Wallet: "0x123",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bridge identifier is required")
	})
}
