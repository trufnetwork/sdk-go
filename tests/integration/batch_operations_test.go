package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

func TestBatchOperations(t *testing.T) {
	ctx := context.Background()
	fixture := NewServerFixture(t)
	err := fixture.Setup()
	t.Cleanup(func() {
		fixture.Teardown()
	})
	require.NoError(t, err, "Failed to setup server fixture")

	deployerWallet, err := kwilcrypto.Secp256k1PrivateKeyFromHex(AnonWalletPK)
	require.NoError(t, err, "failed to parse anon wallet private key")

	authorizeWalletToDeployStreams(t, ctx, fixture, deployerWallet)

	t.Run("TestSequentialSmallBatches", func(t *testing.T) {
		// Create fresh client for this subtest to ensure clean nonce management
		tnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(auth.GetUserSigner(deployerWallet)))
		require.NoError(t, err, "failed to create client")

		streamId := util.GenerateStreamId("test-sequential-small")
		streamLocator := tnClient.OwnStreamLocator(streamId)

		// Set up cleanup
		t.Cleanup(func() {
			destroyResult, err := tnClient.DestroyStream(ctx, streamId)
			assertNoErrorOrFail(t, err, "Failed to destroy stream")
			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
		})

		// Deploy and initialize stream
		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
		assertNoErrorOrFail(t, err, "Failed to deploy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

		deployedStream, err := tnClient.LoadPrimitiveActions()
		assertNoErrorOrFail(t, err, "Failed to load stream")

		const numBatches = 50
		const recordsPerBatch = 5
		baseTimestamp := 1672531200 // Start from 2023-01-01

		// Insert batches sequentially — wait for each tx to be confirmed before
		// sending the next. This matches the post-deadlock design ("many small
		// sequential transactions with per-tx PG isolation") and avoids nonce
		// races caused by rapid-fire Execute calls reading stale pending nonces.
		startTime := time.Now()

		for batch := 0; batch < numBatches; batch++ {
			records := make([]types.InsertRecordInput, recordsPerBatch)
			for i := 0; i < recordsPerBatch; i++ {
				records[i] = types.InsertRecordInput{
					DataProvider: streamLocator.DataProvider.Address(),
					StreamId:     streamLocator.StreamId.String(),
					EventTime:    baseTimestamp + (batch * 86400) + (i * 3600),
					Value:        float64(batch*100 + i + 1), // +1 to avoid zero (consensus filters WHERE value != 0)
				}
			}

			txHash, err := deployedStream.InsertRecords(ctx, records)
			assertNoErrorOrFail(t, err, "Failed to insert batch")
			waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
		}

		insertionDuration := time.Since(startTime)
		fmt.Printf("[Small Batches] All %d batches inserted and confirmed in %v (avg %v per batch)\n",
			numBatches, insertionDuration, insertionDuration/time.Duration(numBatches))

		// Verify total number of records
		totalRecords := numBatches * recordsPerBatch
		dateFrom := baseTimestamp
		dateTo := baseTimestamp + (numBatches * 86400)

		result, err := deployedStream.GetRecord(ctx, types.GetRecordInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamLocator.StreamId.String(),
			From:         &dateFrom,
			To:           &dateTo,
		})
		assertNoErrorOrFail(t, err, "Failed to query records")
		assert.Equal(t, totalRecords, len(result.Results), "Unexpected number of records")
	})

	t.Run("TestSequentialLargeBatches", func(t *testing.T) {
		// Create fresh client for this subtest to ensure clean nonce management
		tnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(auth.GetUserSigner(deployerWallet)))
		require.NoError(t, err, "failed to create client")

		streamId := util.GenerateStreamId("test-sequential-large")
		streamLocator := tnClient.OwnStreamLocator(streamId)

		// Set up cleanup
		t.Cleanup(func() {
			destroyResult, err := tnClient.DestroyStream(ctx, streamId)
			assertNoErrorOrFail(t, err, "Failed to destroy stream")
			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
		})

		// Deploy and initialize stream
		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
		assertNoErrorOrFail(t, err, "Failed to deploy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

		deployedStream, err := tnClient.LoadPrimitiveActions()
		assertNoErrorOrFail(t, err, "Failed to load stream")

		const numBatches = 100
		const recordsPerBatch = 10 // node enforces max 10 records per insert_records call
		baseTimestamp := 1672531200 // Start from 2023-01-01

		// Total: 100*10 = 1000 records — under the 10,000-row query cap.
		// Each tx is confirmed before the next to avoid nonce-fetch races.

		startTime := time.Now()

		for batch := 0; batch < numBatches; batch++ {
			records := make([]types.InsertRecordInput, recordsPerBatch)
			for i := 0; i < recordsPerBatch; i++ {
				records[i] = types.InsertRecordInput{
					DataProvider: streamLocator.DataProvider.Address(),
					StreamId:     streamLocator.StreamId.String(),
					EventTime:    baseTimestamp + (batch * 86400) + (i * 300), // 5-minute intervals
					Value:        float64(batch*1000 + i + 1), // +1 to avoid zero (consensus filters WHERE value != 0)
				}
			}

			txHash, err := deployedStream.InsertRecords(ctx, records)
			assertNoErrorOrFail(t, err, "Failed to insert batch")
			waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
		}

		insertionDuration := time.Since(startTime)
		fmt.Printf("[Large Batches] All %d batches inserted and confirmed in %v (avg %v per batch)\n",
			numBatches, insertionDuration, insertionDuration/time.Duration(numBatches))

		// Verify total number of records
		totalRecords := numBatches * recordsPerBatch
		dateFrom := baseTimestamp
		dateTo := baseTimestamp + (numBatches * 86400)

		result, err := deployedStream.GetRecord(ctx, types.GetRecordInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamLocator.StreamId.String(),
			From:         &dateFrom,
			To:           &dateTo,
		})
		assertNoErrorOrFail(t, err, "Failed to query records")
		assert.Equal(t, totalRecords, len(result.Results), "Unexpected number of records")
	})

	t.Run("TestRapidSingleRecordInserts", func(t *testing.T) {
		// Skip this test in CI as it's flaky due to nonce management issues
		if os.Getenv("CI") != "" {
			t.Skip("Skipping flaky rapid single record inserts test in CI")
		}

		// Create fresh client for this subtest to ensure clean nonce management
		tnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(auth.GetUserSigner(deployerWallet)))
		require.NoError(t, err, "failed to create client")

		streamId := util.GenerateStreamId("test-rapid-singles")
		streamLocator := tnClient.OwnStreamLocator(streamId)

		// Set up cleanup
		t.Cleanup(func() {
			destroyResult, err := tnClient.DestroyStream(ctx, streamId)
			assertNoErrorOrFail(t, err, "Failed to destroy stream")
			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
		})

		// Deploy and initialize stream
		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
		assertNoErrorOrFail(t, err, "Failed to deploy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

		deployedStream, err := tnClient.LoadPrimitiveActions()
		assertNoErrorOrFail(t, err, "Failed to load stream")

		const numRecords = 500
		baseTimestamp := 1672531200 // Start from 2023-01-01

		// Rapidly insert individual records without waiting
		txHashes := make([]kwiltypes.Hash, 0, numRecords)
		startTime := time.Now()

		for i := 0; i < numRecords; i++ {
			records := []types.InsertRecordInput{
				{
					DataProvider: streamLocator.DataProvider.Address(),
					StreamId:     streamLocator.StreamId.String(),
					EventTime:    baseTimestamp + (i * 3600),
					Value:        float64(i + 1), // +1 to avoid zero (consensus filters WHERE value != 0)
				},
			}

			txHash, err := deployedStream.InsertRecords(ctx, records)
			assertNoErrorOrFail(t, err, "Failed to insert record")
			txHashes = append(txHashes, txHash)
		}

		insertionDuration := time.Since(startTime)
		fmt.Printf("[Single Results] All insertions completed in %v (avg %v per record)\n",
			insertionDuration,
			insertionDuration/time.Duration(numRecords))

		// Wait for all transactions after sending them all
		waitStart := time.Now()
		for _, txHash := range txHashes {
			waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
		}
		waitDuration := time.Since(waitStart)
		fmt.Printf("[Single Results] All transactions confirmed in %v\n", waitDuration)

		// Verify total number of records
		dateFrom := baseTimestamp
		dateTo := baseTimestamp + (numRecords * 3600) + 3600

		result, err := deployedStream.GetRecord(ctx, types.GetRecordInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamLocator.StreamId.String(),
			From:         &dateFrom,
			To:           &dateTo,
		})
		assertNoErrorOrFail(t, err, "Failed to query records")
		assert.GreaterOrEqual(t, len(result.Results), numRecords, "Unexpected number of records")
	})
}
