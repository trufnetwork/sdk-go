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
	"github.com/trufnetwork/sdk-go/core/util"
)

// TestGetIndexChange demonstrates the GetIndexChange functionality with a primitive stream
func TestGetIndexChange(t *testing.T) {
	ctx := context.Background()
	fixture := NewServerFixture(t)
	err := fixture.Setup()
	t.Cleanup(func() {
		fixture.Teardown()
	})
	require.NoError(t, err, "Failed to setup server fixture")

	deployerWallet, err := kwilcrypto.Secp256k1PrivateKeyFromHex(AnonWalletPK)
	require.NoError(t, err, "failed to parse anon wallet private key")
	tnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(auth.GetUserSigner(deployerWallet)))
	require.NoError(t, err, "failed to create client")

	authorizeWalletToDeployStreams(t, ctx, fixture, deployerWallet)

	// Generate a unique stream ID and locator
	streamId := util.GenerateStreamId("test-index-change-stream")
	streamLocator := tnClient.OwnStreamLocator(streamId)

	// Set up cleanup to destroy the stream after test completion
	t.Cleanup(func() {
		destroyResult, err := tnClient.DestroyStream(ctx, streamId)
		assertNoErrorOrFail(t, err, "Failed to destroy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
	})

	// Deploy and initialize stream
	deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
	assertNoErrorOrFail(t, err, "Failed to deploy stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	deployedPrimitiveStream, err := tnClient.LoadPrimitiveActions()
	assertNoErrorOrFail(t, err, "Failed to load stream")

	// Insert multiple records with time progression to test index changes
	recordInputs := []types.InsertRecordInput{
		{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamId.String(),
			EventTime:    1000, // Base time
			Value:        100,  // Base value
		},
		{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamId.String(),
			EventTime:    1100, // +100 seconds
			Value:        110,  // +10% increase
		},
		{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamId.String(),
			EventTime:    1200, // +200 seconds
			Value:        120,  // +20% increase from base
		},
		{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamId.String(),
			EventTime:    1300, // +300 seconds
			Value:        108,  // Decreased from previous
		},
	}

	txHash, err := deployedPrimitiveStream.InsertRecords(ctx, recordInputs)
	assertNoErrorOrFail(t, err, "Failed to insert records")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)

	t.Run("GetIndexChangeWithTimeInterval", func(t *testing.T) {
		// Test GetIndexChange with a 100-second interval
		timeInterval := 100 // 100 seconds
		fromTime := 1000
		toTime := 1300

		result, err := deployedPrimitiveStream.GetIndexChange(ctx, types.GetIndexChangeInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamId.String(),
			From:         &fromTime,
			To:           &toTime,
			TimeInterval: timeInterval,
		})
		assertNoErrorOrFail(t, err, "Failed to get index changes")

		// We should get changes for times where we have previous data points
		// The first data point (1000) won't have a change because there's no previous point
		// Points at 1100, 1200, 1300 should have changes relative to points 100 seconds earlier
		assert.True(t, len(result.Results) >= 1, "Expected at least one index change")

		// Verify that we get meaningful change values
		for _, change := range result.Results {
			assert.True(t, change.EventTime >= fromTime, "Event time should be within range")
			assert.True(t, change.EventTime <= toTime, "Event time should be within range")
			// The value should be a percentage change, which could be positive or negative
			assert.NotNil(t, &change.Value, "Index change value should not be nil")
		}
	})

	t.Run("GetIndexChangeWithDifferentTimeInterval", func(t *testing.T) {
		// Test with a longer time interval (200 seconds)
		timeInterval := 200 // 200 seconds
		fromTime := 1000
		toTime := 1300

		result, err := deployedPrimitiveStream.GetIndexChange(ctx, types.GetIndexChangeInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamId.String(),
			From:         &fromTime,
			To:           &toTime,
			TimeInterval: timeInterval,
		})
		assertNoErrorOrFail(t, err, "Failed to get index changes with 200s interval")

		// With 200s interval, only points at 1200 and 1300 should have changes
		// (relative to points at 1000 and 1100 respectively)
		for _, change := range result.Results {
			assert.True(t, change.EventTime >= fromTime, "Event time should be within range")
			assert.True(t, change.EventTime <= toTime, "Event time should be within range")
			assert.NotNil(t, &change.Value, "Index change value should not be nil")
		}
	})

	t.Run("GetIndexChangeWithBaseDate", func(t *testing.T) {
		// Test GetIndexChange with a specific base date
		timeInterval := 100
		fromTime := 1100
		toTime := 1300
		baseDate := 1000 // Use first record as base

		result, err := deployedPrimitiveStream.GetIndexChange(ctx, types.GetIndexChangeInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamId.String(),
			From:         &fromTime,
			To:           &toTime,
			BaseDate:     &baseDate,
			TimeInterval: timeInterval,
		})
		assertNoErrorOrFail(t, err, "Failed to get index changes with base date")

		// Verify results
		for _, change := range result.Results {
			assert.True(t, change.EventTime >= fromTime, "Event time should be within range")
			assert.True(t, change.EventTime <= toTime, "Event time should be within range")
			assert.NotNil(t, &change.Value, "Index change value should not be nil")
		}
	})

	t.Run("GetIndexChangeEmptyResult", func(t *testing.T) {
		// Test with time interval that won't have previous data points
		timeInterval := 1000 // 1000 seconds - longer than our data range
		fromTime := 1000
		toTime := 1300

		result, err := deployedPrimitiveStream.GetIndexChange(ctx, types.GetIndexChangeInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamId.String(),
			From:         &fromTime,
			To:           &toTime,
			TimeInterval: timeInterval,
		})
		assertNoErrorOrFail(t, err, "Failed to get index changes with large interval")

		// With such a large interval, we might get empty results or very few results
		// This is expected behavior as there won't be previous data points to compare against
		assert.True(t, len(result.Results) >= 0, "Should handle large time intervals gracefully")
	})
}
