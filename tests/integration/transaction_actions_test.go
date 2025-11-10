package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

func TestTransactionActions(t *testing.T) {
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

	// Authorize wallet to deploy streams (needed for test setup)
	authorizeWalletToDeployStreams(t, ctx, fixture, wallet)

	// Load transaction actions
	txActions, err := tnClient.LoadTransactionActions()
	require.NoError(t, err, "failed to load transaction actions")

	// Test 1: Get Transaction Event - Success
	t.Run("GetTransactionEvent_Success", func(t *testing.T) {
		// Create a stream to generate a transaction
		streamId := util.GenerateStreamId("tx-test")
		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
		require.NoError(t, err, "failed to deploy stream")

		// Wait for transaction to be mined
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

		// Fetch transaction event
		txEvent, err := txActions.GetTransactionEvent(ctx, types.GetTransactionEventInput{
			TxID: deployTxHash.String(),
		})

		require.NoError(t, err, "failed to get transaction event")
		assert.NotNil(t, txEvent, "transaction event should not be nil")

		// Verify transaction fields
		// Normalize both sides for comparison (node returns with 0x prefix)
		normalizedTxEvent := strings.ToLower(strings.TrimPrefix(txEvent.TxID, "0x"))
		normalizedDeploy := strings.ToLower(strings.TrimPrefix(deployTxHash.String(), "0x"))
		assert.Equal(t, normalizedDeploy, normalizedTxEvent, "tx_id should match")
		assert.Equal(t, "deployStream", txEvent.Method, "method should be deployStream")
		assert.NotEmpty(t, txEvent.Caller, "caller should not be empty")
		assert.Greater(t, txEvent.BlockHeight, int64(0), "block height should be positive")
		assert.NotEmpty(t, txEvent.FeeAmount, "fee amount should not be empty")

		// Fee distributions may be present depending on node configuration
		// Log for debugging
		if len(txEvent.FeeDistributions) > 0 {
			dist := txEvent.FeeDistributions[0]
			t.Logf("Fee distribution: Recipient=%s, Amount=%s", dist.Recipient, dist.Amount)
		} else {
			t.Logf("No fee distributions returned (fee amount: %s)", txEvent.FeeAmount)
		}

		t.Logf("Transaction Event: TX=%s, Method=%s, Fee=%s, Distributions=%d",
			txEvent.TxID, txEvent.Method, txEvent.FeeAmount, len(txEvent.FeeDistributions))

		// Cleanup
		destroyTxHash, err := tnClient.DestroyStream(ctx, streamId)
		require.NoError(t, err, "failed to destroy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyTxHash)
	})

	// Test 2: Get Transaction Event - Without 0x Prefix
	t.Run("GetTransactionEvent_WithoutPrefix", func(t *testing.T) {
		// Create a stream
		streamId := util.GenerateStreamId("tx-test-no-prefix")
		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
		require.NoError(t, err, "failed to deploy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

		// Test with tx hash without 0x prefix
		txHashWithoutPrefix := strings.TrimPrefix(deployTxHash.String(), "0x")
		txEvent, err := txActions.GetTransactionEvent(ctx, types.GetTransactionEventInput{
			TxID: txHashWithoutPrefix,
		})

		require.NoError(t, err, "should accept tx hash without 0x prefix")
		assert.NotNil(t, txEvent, "transaction event should not be nil")
		// The returned hash should have 0x prefix (normalized by node)
		assert.True(t, strings.HasPrefix(txEvent.TxID, "0x"), "returned tx_id should have 0x prefix")

		t.Logf("Query without prefix succeeded. Input=%s, Returned=%s", txHashWithoutPrefix, txEvent.TxID)

		// Cleanup
		destroyTxHash, err := tnClient.DestroyStream(ctx, streamId)
		require.NoError(t, err, "failed to destroy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyTxHash)
	})

	// Test 3: Get Transaction Event - Not Found
	t.Run("GetTransactionEvent_NotFound", func(t *testing.T) {
		// Try to fetch non-existent transaction
		_, err := txActions.GetTransactionEvent(ctx, types.GetTransactionEventInput{
			TxID: "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		})

		assert.Error(t, err, "should return error for non-existent transaction")
		assert.Contains(t, err.Error(), "transaction not found", "error should mention transaction not found")
		t.Logf("Expected error received: %v", err)
	})

	// Test 4: Get Transaction Event - Empty TxID
	t.Run("GetTransactionEvent_EmptyTxID", func(t *testing.T) {
		_, err := txActions.GetTransactionEvent(ctx, types.GetTransactionEventInput{
			TxID: "",
		})

		assert.Error(t, err, "should return error for empty tx_id")
		assert.Contains(t, err.Error(), "tx_id is required", "error should mention tx_id is required")
	})

	// Test 5: List Transaction Fees - Paid Mode
	t.Run("ListTransactionFees_Paid", func(t *testing.T) {
		// Create a transaction
		streamId := util.GenerateStreamId("tx-fee-test")
		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
		require.NoError(t, err, "failed to deploy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

		// Get wallet address
		signerAddress := tnClient.Address()
		walletAddr := signerAddress.Address()

		// List fees paid by this wallet
		limit := 20
		entries, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
			Wallet: walletAddr,
			Mode:   types.TransactionFeeModePaid,
			Limit:  &limit,
		})

		require.NoError(t, err, "failed to list transaction fees")
		assert.NotEmpty(t, entries, "should have at least one fee entry")

		// Verify the deploy transaction is in the results
		found := false
		for _, entry := range entries {
			// Normalize comparison (node may add or remove 0x prefix)
			entryTxID := strings.ToLower(strings.TrimPrefix(entry.TxID, "0x"))
			deployTxID := strings.ToLower(strings.TrimPrefix(deployTxHash.String(), "0x"))

			if entryTxID == deployTxID {
				found = true
				assert.Equal(t, "deployStream", entry.Method, "method should be deployStream")
				assert.Equal(t, strings.ToLower(walletAddr), strings.ToLower(entry.Caller), "caller should match wallet")
				assert.NotEmpty(t, entry.TotalFee, "total fee should not be empty")
				recipient := "<nil>"
				if entry.DistributionRecipient != nil {
					recipient = *entry.DistributionRecipient
				}
				amount := "<nil>"
				if entry.DistributionAmount != nil {
					amount = *entry.DistributionAmount
				}
				t.Logf("Found deploy transaction: Fee=%s, Recipient=%s, Amount=%s",
					entry.TotalFee, recipient, amount)
				break
			}
		}
		assert.True(t, found, "deploy transaction should be in paid fees list")

		t.Logf("Listed %d fee entries in paid mode", len(entries))

		// Cleanup
		destroyTxHash, err := tnClient.DestroyStream(ctx, streamId)
		require.NoError(t, err, "failed to destroy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyTxHash)
	})

	// Test 6: List Transaction Fees - Both Mode
	t.Run("ListTransactionFees_Both", func(t *testing.T) {
		signerAddress := tnClient.Address()
		walletAddr := signerAddress.Address()

		limit := 10
		entries, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
			Wallet: walletAddr,
			Mode:   types.TransactionFeeModeBoth,
			Limit:  &limit,
		})

		require.NoError(t, err, "failed to list transaction fees in both mode")
		// Should have entries from previous tests
		t.Logf("Listed %d fee entries in both mode", len(entries))
	})

	// Test 7: List Transaction Fees - Invalid Mode
	t.Run("ListTransactionFees_InvalidMode", func(t *testing.T) {
		_, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
			Wallet: "0x1234567890123456789012345678901234567890",
			Mode:   "invalid_mode", // Invalid mode
		})

		assert.Error(t, err, "should return error for invalid mode")
		assert.Contains(t, err.Error(), "mode must be one of", "error should mention valid modes")
	})

	// Test 8: List Transaction Fees - Pagination
	t.Run("ListTransactionFees_Pagination", func(t *testing.T) {
		signerAddress := tnClient.Address()
		walletAddr := signerAddress.Address()

		// Test with limit
		limit := 5
		entries, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
			Wallet: walletAddr,
			Mode:   types.TransactionFeeModeBoth,
			Limit:  &limit,
		})

		require.NoError(t, err, "failed to list with limit")
		assert.LessOrEqual(t, len(entries), 5, "should respect limit")
		t.Logf("With limit=5, got %d entries", len(entries))

		// Test with offset
		if len(entries) > 2 {
			offset := 2
			entriesWithOffset, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
				Wallet: walletAddr,
				Mode:   types.TransactionFeeModeBoth,
				Limit:  &limit,
				Offset: &offset,
			})

			require.NoError(t, err, "failed to list with offset")
			assert.LessOrEqual(t, len(entriesWithOffset), 5, "should respect limit with offset")
			t.Logf("With offset=2, got %d entries", len(entriesWithOffset))

			// The first entry with offset should match the third entry without offset
			if len(entriesWithOffset) > 0 && len(entries) > 2 {
				assert.Equal(t, entries[2].TxID, entriesWithOffset[0].TxID,
					"offset should skip the first N entries")
			}
		}
	})

	// Test 9: List Transaction Fees - Invalid Limit
	t.Run("ListTransactionFees_InvalidLimit", func(t *testing.T) {
		invalidLimit := 2000 // Exceeds max of 1000
		_, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
			Wallet: "0x1234567890123456789012345678901234567890",
			Mode:   types.TransactionFeeModePaid,
			Limit:  &invalidLimit,
		})

		assert.Error(t, err, "should return error for limit exceeding max")
		assert.Contains(t, err.Error(), "limit cannot exceed 1000", "error should mention limit constraint")
	})

	// Test 10: Multiple Transactions with Fee Distributions
	t.Run("MultipleTransactions_FeeDistributions", func(t *testing.T) {
		// Create multiple streams to generate multiple transactions
		streamIds := []util.StreamId{
			util.GenerateStreamId("multi-tx-1"),
			util.GenerateStreamId("multi-tx-2"),
		}

		var txHashes []string
		for _, streamId := range streamIds {
			deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
			require.NoError(t, err, "failed to deploy stream")
			waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
			txHashes = append(txHashes, deployTxHash.String())
		}

		signerAddress := tnClient.Address()
		walletAddr := signerAddress.Address()

		// List all fees
		limit := 50
		entries, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
			Wallet: walletAddr,
			Mode:   types.TransactionFeeModePaid,
			Limit:  &limit,
		})

		require.NoError(t, err, "failed to list transaction fees")

		// Verify both transactions are present
		foundCount := 0
		for _, txHash := range txHashes {
			for _, entry := range entries {
				// Normalize comparison
				entryTxID := strings.ToLower(strings.TrimPrefix(entry.TxID, "0x"))
				expectedTxID := strings.ToLower(strings.TrimPrefix(txHash, "0x"))

				if entryTxID == expectedTxID {
					foundCount++
					t.Logf("Found transaction %s in fee list", txHash)
					break
				}
			}
		}
		assert.Equal(t, len(txHashes), foundCount, "all transactions should be found in fee list")

		t.Logf("Successfully verified %d transactions with fee distributions", foundCount)

		// Cleanup
		for _, streamId := range streamIds {
			destroyTxHash, err := tnClient.DestroyStream(ctx, streamId)
			require.NoError(t, err, "failed to destroy stream")
			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyTxHash)
		}
	})
}
