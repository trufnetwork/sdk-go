package integration

import (
	"context"
	"fmt"
	"sort" // For comparing slices of StreamLocator
	"testing"
	"time"

	// kwilcrypto "github.com/kwilteam/kwil-db/core/crypto" // Will be removed if not used elsewhere
	// "github.com/kwilteam/kwil-db/core/crypto/auth" // Will be removed if not used elsewhere
	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

func TestBatchDeployAndExistenceOperations(t *testing.T) {
	fixture := NewServerFixture(t)
	err := fixture.Setup()
	defer fixture.Teardown()
	require.NoError(t, err, "Failed to setup server fixture")

	tnClient := fixture.Client()
	require.NotNil(t, tnClient, "Client from fixture should not be nil")

	ctx := context.Background()
	signerAddress := tnClient.Address()

	// =========================================================================
	// TestBatchDeployStreams
	// =========================================================================
	t.Run("BatchDeployStreams_Success", func(t *testing.T) {
		streamIdRoot1 := "batch-deploy-prim-1"
		streamIdRoot2 := "batch-deploy-comp-1"
		streamIdRoot3 := "batch-deploy-prim-2"

		streamId1 := util.GenerateStreamId(streamIdRoot1)
		streamId2 := util.GenerateStreamId(streamIdRoot2)
		streamId3 := util.GenerateStreamId(streamIdRoot3)

		streamDef1 := types.StreamDefinition{StreamId: streamId1, StreamType: types.StreamTypePrimitive}
		streamDef2 := types.StreamDefinition{StreamId: streamId2, StreamType: types.StreamTypeComposed}
		streamDef3 := types.StreamDefinition{StreamId: streamId3, StreamType: types.StreamTypePrimitive}

		definitions := []types.StreamDefinition{streamDef1, streamDef2, streamDef3}
		streamLocatorsToDeploy := make([]types.StreamLocator, len(definitions))
		rawStreamIdsToDeploy := make([]util.StreamId, len(definitions))

		for i, def := range definitions {
			rawStreamIdsToDeploy[i] = def.StreamId
			streamLocatorsToDeploy[i] = types.StreamLocator{
				StreamId:     def.StreamId,
				DataProvider: signerAddress,
			}
		}

		t.Cleanup(func() {
			for _, sid := range rawStreamIdsToDeploy {
				// It's possible a stream wasn't created if the batch tx failed,
				// so we check existence before attempting destroy or simply ignore error from DestroyStream.
				// A more robust cleanup might list streams by owner and destroy them.
				// For simplicity, we'll try to destroy and ignore "not found" type errors.
				destroyTx, _ := tnClient.DestroyStream(ctx, sid)
				// Only wait if a transaction hash was returned, implying an attempt was made.
				if destroyTx != (kwiltypes.Hash{}) {
					// We don't care about the success of cleanup tx for test outcome here, but good to wait.
					_, _ = tnClient.WaitForTx(ctx, destroyTx, time.Second)
				}
			}
		})

		txHash, err := tnClient.BatchDeployStreams(ctx, definitions)
		assertNoErrorOrFail(t, err, "BatchDeployStreams failed")
		assert.NotEmpty(t, txHash, "Expected a transaction hash from BatchDeployStreams")

		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)

		// Verify existence using BatchStreamExists
		existenceResults, err := tnClient.BatchStreamExists(ctx, streamLocatorsToDeploy)
		assertNoErrorOrFail(t, err, "BatchStreamExists failed after batch deploy")
		assert.Len(t, existenceResults, len(definitions), "Incorrect number of results from BatchStreamExists")

		for _, result := range existenceResults {
			found := false
			for _, loc := range streamLocatorsToDeploy {
				if result.StreamLocator.StreamId.String() == loc.StreamId.String() &&
					result.StreamLocator.DataProvider.Address() == loc.DataProvider.Address() {
					assert.True(t, result.Exists, "Stream %s should exist after batch deploy", loc.StreamId.String())
					found = true
					break
				}
			}
			assert.True(t, found, "Unexpected stream locator in BatchStreamExists result: %v", result.StreamLocator)
		}

		// Further verification with ListStreams
		listedStreams, err := tnClient.ListStreams(ctx, types.ListStreamsInput{DataProvider: signerAddress.Address(), Limit: 100}) // Increased limit
		assertNoErrorOrFail(t, err, "ListStreams failed")

		for _, def := range definitions {
			foundInList := false
			for _, listed := range listedStreams {
				if listed.StreamId == def.StreamId.String() && listed.DataProvider == signerAddress.Address() {
					assert.Equal(t, string(def.StreamType), listed.StreamType, "Mismatched stream type for %s", def.StreamId)
					foundInList = true
					break
				}
			}
			assert.True(t, foundInList, "Stream %s (def type: %s) not found in ListStreams output or type mismatch", def.StreamId, def.StreamType)
		}
	})

	t.Run("BatchDeployStreams_EmptyInput", func(t *testing.T) {
		txHash, err := tnClient.BatchDeployStreams(ctx, []types.StreamDefinition{})
		assert.Error(t, err, "Expected error for empty definitions")
		assert.Contains(t, err.Error(), "no stream definitions provided", "Unexpected error message")
		assert.Empty(t, txHash, "Expected no transaction hash for empty definitions")
	})

	t.Run("BatchDeployStreams_DuplicateInBatchForSameOwner", func(t *testing.T) {
		existingStreamId := util.GenerateStreamId("batch-deploy-existing-dup")
		newStreamId := util.GenerateStreamId("batch-deploy-new-dup")

		// Deploy S1 first
		deployTx, err := tnClient.DeployStream(ctx, existingStreamId, types.StreamTypePrimitive)
		assertNoErrorOrFail(t, err, "Failed to deploy initial stream S1")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTx)

		t.Cleanup(func() {
			destroyTx, _ := tnClient.DestroyStream(ctx, existingStreamId)
			if destroyTx != (kwiltypes.Hash{}) {
				_, _ = tnClient.WaitForTx(ctx, destroyTx, time.Second)
			}
			destroyTx2, _ := tnClient.DestroyStream(ctx, newStreamId)
			if destroyTx2 != (kwiltypes.Hash{}) {
				_, _ = tnClient.WaitForTx(ctx, destroyTx2, time.Second)
			}
		})

		definitions := []types.StreamDefinition{
			{StreamId: existingStreamId, StreamType: types.StreamTypePrimitive}, // Duplicate
			{StreamId: newStreamId, StreamType: types.StreamTypeComposed},       // New
		}

		batchTxHash, err := tnClient.BatchDeployStreams(ctx, definitions)
		// The client-side call to BatchDeployStreams itself should succeed if the arguments are well-formed.
		// The error occurs when the transaction is processed by the network.
		assertNoErrorOrFail(t, err, "BatchDeployStreams submission should not error for duplicate (error is on-chain)")
		assert.NotEmpty(t, batchTxHash, "Expected a transaction hash from BatchDeployStreams even with potential on-chain failure")

		// Expect the transaction to fail on-chain
		txRes, err := tnClient.WaitForTx(ctx, batchTxHash, 2*time.Second) // Using a slightly longer interval for batch
		assertNoErrorOrFail(t, err, "WaitForTx for batch with duplicate failed")
		assert.NotEqual(t, kwiltypes.CodeOk, kwiltypes.TxCode(txRes.Result.Code), "Transaction with duplicate stream should have failed. Log: %s", txRes.Result.Log)
		// A more specific check for Kwil DB: txRes.Result.Log often contains "UNIQUE constraint failed" or similar for such errors.
		// e.g. assert.Contains(t, txRes.Result.Log, "UNIQUE constraint failed") // This might be too specific/brittle.

		// Verify S2 (the new stream) was NOT created due to atomic transaction failure
		s2Locator := types.StreamLocator{StreamId: newStreamId, DataProvider: signerAddress}
		existsResults, err := tnClient.BatchStreamExists(ctx, []types.StreamLocator{s2Locator})
		assertNoErrorOrFail(t, err, "BatchStreamExists for S2 check failed")
		assert.Len(t, existsResults, 1)
		assert.False(t, existsResults[0].Exists, "Stream S2 (new in batch) should not exist after failed batch deploy")
	})

	// =========================================================================
	// TestBatchStreamExists
	// =========================================================================
	t.Run("BatchStreamExists_Mixed", func(t *testing.T) {
		primStreamId1 := util.GenerateStreamId("bse-prim-1")
		compStreamId1 := util.GenerateStreamId("bse-comp-1")
		nonExistentStreamId1 := util.GenerateStreamId("bse-nonexistent-1")
		nonExistentStreamId2 := util.GenerateStreamId("bse-nonexistent-2")
		otherOwnerAddress := util.Unsafe_NewEthereumAddressFromString("0x1234567890123456789012345678901234567890")

		// Deploy some streams
		deployTx1, err := tnClient.DeployStream(ctx, primStreamId1, types.StreamTypePrimitive)
		assertNoErrorOrFail(t, err, "Failed to deploy bse-prim-1")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTx1)

		deployTx2, err := tnClient.DeployStream(ctx, compStreamId1, types.StreamTypeComposed)
		assertNoErrorOrFail(t, err, "Failed to deploy bse-comp-1")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTx2)

		t.Cleanup(func() {
			dtx1, _ := tnClient.DestroyStream(ctx, primStreamId1)
			if dtx1 != (kwiltypes.Hash{}) {
				waitTxToBeMinedWithSuccess(t, ctx, tnClient, dtx1)
			}
			dtx2, _ := tnClient.DestroyStream(ctx, compStreamId1)
			if dtx2 != (kwiltypes.Hash{}) {
				waitTxToBeMinedWithSuccess(t, ctx, tnClient, dtx2)
			}
		})

		locators := []types.StreamLocator{
			{StreamId: primStreamId1, DataProvider: signerAddress},        // Exists
			{StreamId: compStreamId1, DataProvider: signerAddress},        // Exists
			{StreamId: nonExistentStreamId1, DataProvider: signerAddress}, // Doesn't exist (ID)
			{StreamId: primStreamId1, DataProvider: otherOwnerAddress},    // Doesn't exist (Owner)
			{StreamId: nonExistentStreamId2, DataProvider: signerAddress}, // Doesn't exist (ID)
		}

		// Create a map for easier lookup of expected results
		expectedResultsMap := make(map[string]bool)
		for _, loc := range locators {
			key := fmt.Sprintf("%s-%s", loc.StreamId.String(), loc.DataProvider.Address())
			// Default to false, set to true for known existing ones
			expectedResultsMap[key] = false
		}
		expectedResultsMap[fmt.Sprintf("%s-%s", primStreamId1.String(), signerAddress.Address())] = true
		expectedResultsMap[fmt.Sprintf("%s-%s", compStreamId1.String(), signerAddress.Address())] = true

		results, err := tnClient.BatchStreamExists(ctx, locators)
		assertNoErrorOrFail(t, err, "BatchStreamExists failed")
		assert.Len(t, results, len(locators), "Incorrect number of results from BatchStreamExists")

		for _, res := range results {
			key := fmt.Sprintf("%s-%s", res.StreamLocator.StreamId.String(), res.StreamLocator.DataProvider.Address())
			expectedExist, ok := expectedResultsMap[key]
			assert.True(t, ok, "Unexpected stream locator in result: %v (key: %s)", res.StreamLocator, key)
			assert.Equal(t, expectedExist, res.Exists, "Existence mismatch for %s", key)
		}
	})

	t.Run("BatchStreamExists_EmptyInput", func(t *testing.T) {
		results, err := tnClient.BatchStreamExists(ctx, []types.StreamLocator{})
		assertNoErrorOrFail(t, err, "BatchStreamExists with empty input failed")
		assert.Empty(t, results, "Expected empty result for empty input to BatchStreamExists")
	})

	t.Run("BatchStreamExists_AllExist", func(t *testing.T) {
		s1 := util.GenerateStreamId("bse-all-exist-1")
		s2 := util.GenerateStreamId("bse-all-exist-2")
		deployTestPrimitiveStreamWithData(t, ctx, tnClient, []util.StreamId{s1, s2}, nil)
		t.Cleanup(func() {
			dtx1, _ := tnClient.DestroyStream(ctx, s1)
			if dtx1 != (kwiltypes.Hash{}) {
				waitTxToBeMinedWithSuccess(t, ctx, tnClient, dtx1)
			}
			dtx2, _ := tnClient.DestroyStream(ctx, s2)
			if dtx2 != (kwiltypes.Hash{}) {
				waitTxToBeMinedWithSuccess(t, ctx, tnClient, dtx2)
			}
		})

		locators := []types.StreamLocator{
			{StreamId: s1, DataProvider: signerAddress},
			{StreamId: s2, DataProvider: signerAddress},
		}
		results, err := tnClient.BatchStreamExists(ctx, locators)
		assertNoErrorOrFail(t, err, "BatchStreamExists (all exist) failed")
		assert.Len(t, results, 2)
		for _, res := range results {
			assert.True(t, res.Exists, "Stream %s should exist", res.StreamLocator.StreamId.String())
		}
	})

	t.Run("BatchStreamExists_NoneExist", func(t *testing.T) {
		s1 := util.GenerateStreamId("bse-none-exist-1")
		s2 := util.GenerateStreamId("bse-none-exist-2")
		locators := []types.StreamLocator{
			{StreamId: s1, DataProvider: signerAddress},
			{StreamId: s2, DataProvider: signerAddress},
		}
		results, err := tnClient.BatchStreamExists(ctx, locators)
		assertNoErrorOrFail(t, err, "BatchStreamExists (none exist) failed")
		assert.Len(t, results, 2)
		for _, res := range results {
			assert.False(t, res.Exists, "Stream %s should not exist", res.StreamLocator.StreamId.String())
		}
	})

	// =========================================================================
	// TestBatchFilterStreamsByExistence
	// =========================================================================
	t.Run("BatchFilterStreamsByExistence_FilterExisting", func(t *testing.T) {
		sExisting1 := util.GenerateStreamId("bfse-exist-1")
		sExisting2 := util.GenerateStreamId("bfse-exist-2")
		sNonExisting1 := util.GenerateStreamId("bfse-nonexist-1")
		sNonExisting2 := util.GenerateStreamId("bfse-nonexist-2")

		deployTestPrimitiveStreamWithData(t, ctx, tnClient, []util.StreamId{sExisting1, sExisting2}, nil)
		t.Cleanup(func() {
			dtx1, _ := tnClient.DestroyStream(ctx, sExisting1)
			if dtx1 != (kwiltypes.Hash{}) {
				waitTxToBeMinedWithSuccess(t, ctx, tnClient, dtx1)
			}
			dtx2, _ := tnClient.DestroyStream(ctx, sExisting2)
			if dtx2 != (kwiltypes.Hash{}) {
				waitTxToBeMinedWithSuccess(t, ctx, tnClient, dtx2)
			}
		})

		locExisting1 := types.StreamLocator{StreamId: sExisting1, DataProvider: signerAddress}
		locExisting2 := types.StreamLocator{StreamId: sExisting2, DataProvider: signerAddress}
		locNonExisting1 := types.StreamLocator{StreamId: sNonExisting1, DataProvider: signerAddress}
		locNonExisting2 := types.StreamLocator{StreamId: sNonExisting2, DataProvider: signerAddress}

		allLocators := []types.StreamLocator{locExisting1, locNonExisting1, locExisting2, locNonExisting2}
		// Order matters for direct comparison if not sorting, so define expected in an order that might be returned or sort both
		expectedExisting := []types.StreamLocator{locExisting1, locExisting2} // Assuming procedure might return them in order of existence or input

		filtered, err := tnClient.BatchFilterStreamsByExistence(ctx, allLocators, true)
		assertNoErrorOrFail(t, err, "BatchFilterStreamsByExistence (filter existing) failed")

		// Sort both slices for stable comparison as order from backend is not strictly guaranteed
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].StreamId.String() < filtered[j].StreamId.String()
		})
		sort.Slice(expectedExisting, func(i, j int) bool {
			return expectedExisting[i].StreamId.String() < expectedExisting[j].StreamId.String()
		})
		assert.Equal(t, expectedExisting, filtered, "Filtered existing streams mismatch")
	})

	t.Run("BatchFilterStreamsByExistence_FilterNonExisting", func(t *testing.T) {
		sExisting1 := util.GenerateStreamId("bfse-exist-filter-non-1")
		sNonExisting1 := util.GenerateStreamId("bfse-nonexist-filter-non-1")

		deployTestPrimitiveStreamWithData(t, ctx, tnClient, []util.StreamId{sExisting1}, nil)
		t.Cleanup(func() {
			dtx1, _ := tnClient.DestroyStream(ctx, sExisting1)
			if dtx1 != (kwiltypes.Hash{}) {
				waitTxToBeMinedWithSuccess(t, ctx, tnClient, dtx1)
			}
		})

		locExisting1 := types.StreamLocator{StreamId: sExisting1, DataProvider: signerAddress}
		locNonExisting1 := types.StreamLocator{StreamId: sNonExisting1, DataProvider: signerAddress}

		allLocators := []types.StreamLocator{locExisting1, locNonExisting1}
		expectedNonExisting := []types.StreamLocator{locNonExisting1}

		filtered, err := tnClient.BatchFilterStreamsByExistence(ctx, allLocators, false)
		assertNoErrorOrFail(t, err, "BatchFilterStreamsByExistence (filter non-existing) failed")

		// Sort for stable comparison if multiple non-existing items were expected
		// Here, only one is expected, so sorting isn't strictly necessary but good practice
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].StreamId.String() < filtered[j].StreamId.String()
		})
		sort.Slice(expectedNonExisting, func(i, j int) bool {
			return expectedNonExisting[i].StreamId.String() < expectedNonExisting[j].StreamId.String()
		})
		assert.Equal(t, expectedNonExisting, filtered, "Filtered non-existing streams mismatch")
	})

	t.Run("BatchFilterStreamsByExistence_EmptyInput", func(t *testing.T) {
		filtered, err := tnClient.BatchFilterStreamsByExistence(ctx, []types.StreamLocator{}, true)
		assertNoErrorOrFail(t, err, "BatchFilterStreamsByExistence (empty, true) failed")
		assert.Empty(t, filtered)

		filtered, err = tnClient.BatchFilterStreamsByExistence(ctx, []types.StreamLocator{}, false)
		assertNoErrorOrFail(t, err, "BatchFilterStreamsByExistence (empty, false) failed")
		assert.Empty(t, filtered)
	})
}
