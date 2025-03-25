package integration

//
//import (
//	"context"
//	"fmt"
//	kwiltypes "github.com/kwilteam/kwil-db/core/types"
//	"testing"
//	"time"
//
//	"github.com/kwilteam/kwil-db/core/crypto"
//	"github.com/kwilteam/kwil-db/core/crypto/auth"
//	// "github.com/kwilteam/kwil-db/core/types/transactions"
//	"github.com/stretchr/testify/assert"
//	"github.com/trufnetwork/sdk-go/core/tnclient"
//	"github.com/trufnetwork/sdk-go/core/types"
//	"github.com/trufnetwork/sdk-go/core/util"
//)
//
//func TestBatchOperations(t *testing.T) {
//	ctx := context.Background()
//
//	// Parse the private key for authentication
//	pk, err := crypto.Secp256k1PrivateKeyFromHex(TestPrivateKey)
//	assertNoErrorOrFail(t, err, "Failed to parse private key")
//
//	// Create a signer using the parsed private key
//	signer := &auth.EthPersonalSigner{Key: *pk}
//
//	// Initialize the TN client with the signer
//	tnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(signer))
//	assertNoErrorOrFail(t, err, "Failed to create client")
//
//	t.Run("TestSequentialSmallBatches", func(t *testing.T) {
//		streamId := util.GenerateStreamId("test-sequential-small")
//		streamLocator := tnClient.OwnStreamLocator(streamId)
//
//		// Set up cleanup
//		t.Cleanup(func() {
//			destroyResult, err := tnClient.DestroyStream(ctx, streamId)
//			assertNoErrorOrFail(t, err, "Failed to destroy stream")
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//		})
//
//		// Deploy and initialize stream
//		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitiveUnix)
//		assertNoErrorOrFail(t, err, "Failed to deploy stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
//
//		deployedStream, err := tnClient.LoadPrimitiveActions(streamLocator)
//		assertNoErrorOrFail(t, err, "Failed to load stream")
//
//		txHashInit, err := deployedStream.InitializeStream(ctx)
//		assertNoErrorOrFail(t, err, "Failed to initialize stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInit)
//
//		const numBatches = 500
//		const recordsPerBatch = 5
//		baseTimestamp := 1672531200 // Start from 2023-01-01
//
//		// Insert multiple batches without waiting
//		txHashes := make([]kwiltypes.Hash, 0, numBatches)
//		startTime := time.Now()
//
//		for batch := 0; batch < numBatches; batch++ {
//			records := make([]types.InsertRecordUnixInput, recordsPerBatch)
//			for i := 0; i < recordsPerBatch; i++ {
//				records[i] = types.InsertRecordUnixInput{
//					EventTime: baseTimestamp + (batch * 86400) + (i * 3600),
//					Value:     float64(batch*100 + i),
//				}
//			}
//
//			txHash, err := deployedStream.InsertRecordsUnix(ctx, records)
//			assertNoErrorOrFail(t, err, "Failed to insert batch")
//			txHashes = append(txHashes, txHash)
//		}
//
//		insertionDuration := time.Since(startTime)
//		fmt.Printf("[Small Batches] All insertions completed in %v (avg %v per batch, %v per record)\n",
//			insertionDuration,
//			insertionDuration/time.Duration(numBatches),
//			insertionDuration/time.Duration(numBatches*recordsPerBatch))
//
//		// Wait for all transactions after sending them all
//		waitStart := time.Now()
//		for _, txHash := range txHashes {
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
//		}
//		waitDuration := time.Since(waitStart)
//		fmt.Printf("[Small Batches] All transactions confirmed in %v\n", waitDuration)
//
//		// Verify total number of records
//		totalRecords := numBatches * recordsPerBatch
//		dateFrom := baseTimestamp
//		dateTo := baseTimestamp + (numBatches * 86400)
//
//		records, err := deployedStream.GetRecordUnix(ctx, types.GetRecordUnixInput{
//			DateFrom: &dateFrom,
//			DateTo:   &dateTo,
//		})
//		assertNoErrorOrFail(t, err, "Failed to query records")
//		assert.Equal(t, totalRecords, len(records), "Unexpected number of records")
//	})
//
//	t.Run("TestSequentialLargeBatches", func(t *testing.T) {
//		streamId := util.GenerateStreamId("test-sequential-large")
//		streamLocator := tnClient.OwnStreamLocator(streamId)
//
//		// Set up cleanup
//		t.Cleanup(func() {
//			destroyResult, err := tnClient.DestroyStream(ctx, streamId)
//			assertNoErrorOrFail(t, err, "Failed to destroy stream")
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//		})
//
//		// Deploy and initialize stream
//		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitiveUnix)
//		assertNoErrorOrFail(t, err, "Failed to deploy stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
//
//		deployedStream, err := tnClient.LoadPrimitiveActions(streamLocator)
//		assertNoErrorOrFail(t, err, "Failed to load stream")
//
//		txHashInit, err := deployedStream.InitializeStream(ctx)
//		assertNoErrorOrFail(t, err, "Failed to initialize stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInit)
//
//		const numBatches = 500
//		const recordsPerBatch = 100
//		baseTimestamp := 1672531200 // Start from 2023-01-01
//
//		// Insert multiple batches without waiting
//		txHashes := make([]kwiltypes.Hash, 0, numBatches)
//		startTime := time.Now()
//
//		for batch := 0; batch < numBatches; batch++ {
//			records := make([]types.InsertRecordUnixInput, recordsPerBatch)
//			for i := 0; i < recordsPerBatch; i++ {
//				records[i] = types.InsertRecordUnixInput{
//					EventTime: baseTimestamp + (batch * 86400) + (i * 300), // 5-minute intervals
//					Value:     float64(batch*1000 + i),
//				}
//			}
//
//			txHash, err := deployedStream.InsertRecordsUnix(ctx, records)
//			assertNoErrorOrFail(t, err, "Failed to insert batch")
//			txHashes = append(txHashes, txHash)
//		}
//
//		insertionDuration := time.Since(startTime)
//		fmt.Printf("[Large Batches] All insertions completed in %v (avg %v per batch, %v per record)\n",
//			insertionDuration,
//			insertionDuration/time.Duration(numBatches),
//			insertionDuration/time.Duration(numBatches*recordsPerBatch))
//
//		// Wait for all transactions after sending them all
//		waitStart := time.Now()
//		for _, txHash := range txHashes {
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
//		}
//		waitDuration := time.Since(waitStart)
//		fmt.Printf("[Large Batches] All transactions confirmed in %v\n", waitDuration)
//
//		// Verify total number of records
//		totalRecords := numBatches * recordsPerBatch
//		dateFrom := baseTimestamp
//		dateTo := baseTimestamp + (numBatches * 86400)
//
//		records, err := deployedStream.GetRecordUnix(ctx, types.GetRecordUnixInput{
//			DateFrom: &dateFrom,
//			DateTo:   &dateTo,
//		})
//		assertNoErrorOrFail(t, err, "Failed to query records")
//		assert.Equal(t, totalRecords, len(records), "Unexpected number of records")
//	})
//
//	t.Run("TestRapidSingleRecordInserts", func(t *testing.T) {
//		streamId := util.GenerateStreamId("test-rapid-singles")
//		streamLocator := tnClient.OwnStreamLocator(streamId)
//
//		// Set up cleanup
//		t.Cleanup(func() {
//			destroyResult, err := tnClient.DestroyStream(ctx, streamId)
//			assertNoErrorOrFail(t, err, "Failed to destroy stream")
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//		})
//
//		// Deploy and initialize stream
//		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitiveUnix)
//		assertNoErrorOrFail(t, err, "Failed to deploy stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
//
//		deployedStream, err := tnClient.LoadPrimitiveActions(streamLocator)
//		assertNoErrorOrFail(t, err, "Failed to load stream")
//
//		txHashInit, err := deployedStream.InitializeStream(ctx)
//		assertNoErrorOrFail(t, err, "Failed to initialize stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInit)
//
//		const numRecords = 500
//		baseTimestamp := 1672531200 // Start from 2023-01-01
//
//		// Rapidly insert individual records without waiting
//		txHashes := make([]kwiltypes.Hash, 0, numRecords)
//		startTime := time.Now()
//
//		for i := 0; i < numRecords; i++ {
//			records := []types.InsertRecordUnixInput{
//				{
//					EventTime: baseTimestamp + (i * 3600),
//					Value:     float64(i),
//				},
//			}
//
//			txHash, err := deployedStream.InsertRecordsUnix(ctx, records)
//			assertNoErrorOrFail(t, err, "Failed to insert record")
//			txHashes = append(txHashes, txHash)
//		}
//
//		insertionDuration := time.Since(startTime)
//		fmt.Printf("[Single Records] All insertions completed in %v (avg %v per record)\n",
//			insertionDuration,
//			insertionDuration/time.Duration(numRecords))
//
//		// Wait for all transactions after sending them all
//		waitStart := time.Now()
//		for _, txHash := range txHashes {
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
//		}
//		waitDuration := time.Since(waitStart)
//		fmt.Printf("[Single Records] All transactions confirmed in %v\n", waitDuration)
//
//		// Verify all records were inserted
//		dateFrom := baseTimestamp
//		dateTo := baseTimestamp + (numRecords * 3600)
//
//		records, err := deployedStream.GetRecordUnix(ctx, types.GetRecordUnixInput{
//			DateFrom: &dateFrom,
//			DateTo:   &dateTo,
//		})
//		assertNoErrorOrFail(t, err, "Failed to query records")
//		assert.Equal(t, numRecords, len(records), "Unexpected number of records")
//	})
//}
