package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// TestCacheParameterSupport tests that all SDK methods support the UseCache parameter
// and maintain backward compatibility when the parameter is omitted
func TestCacheParameterSupport(t *testing.T) {
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
	streamId := util.GenerateStreamId("test-cache-functionality")
	streamLocator := tnClient.OwnStreamLocator(streamId)

	// Set up cleanup
	t.Cleanup(func() {
		destroyResult, err := tnClient.DestroyStream(ctx, streamId)
		assertNoErrorOrFail(t, err, "Failed to destroy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
	})

	// Deploy stream
	deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
	assertNoErrorOrFail(t, err, "Failed to deploy stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	// Load primitive actions
	primitiveActions, err := tnClient.LoadPrimitiveActions()
	assertNoErrorOrFail(t, err, "Failed to load primitive actions")

	// Insert test data
	insertTxHash, err := primitiveActions.InsertRecord(ctx, types.InsertRecordInput{
		DataProvider: streamLocator.DataProvider.Address(),
		StreamId:     streamLocator.StreamId.String(),
		EventTime:    1,
		Value:        100.50,
	})
	assertNoErrorOrFail(t, err, "Failed to insert record")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, insertTxHash)

	// Wait a moment for data to be processed
	time.Sleep(100 * time.Millisecond)

	t.Run("GetRecord with UseCache parameter", func(t *testing.T) {
		// Test 1: Omitted UseCache (should default to false)
		result1, err := primitiveActions.GetRecord(ctx, types.GetRecordInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamLocator.StreamId.String(),
			From:         &[]int{1}[0],
			To:           &[]int{2}[0],
			// UseCache omitted - should work
		})
		require.NoError(t, err, "GetRecord with omitted UseCache should work")
		assert.NotEmpty(t, result1, "Should return records")

		// Test 2: Explicit UseCache = false
		useCacheFalse := false
		result2, err := primitiveActions.GetRecord(ctx, types.GetRecordInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamLocator.StreamId.String(),
			From:         &[]int{1}[0],
			To:           &[]int{2}[0],
			UseCache:     &useCacheFalse,
		})
		require.NoError(t, err, "GetRecord with UseCache=false should work")
		assert.NotEmpty(t, result2, "Should return records")

		// Test 3: Explicit UseCache = true
		useCacheTrue := true
		result3, err := primitiveActions.GetRecord(ctx, types.GetRecordInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamLocator.StreamId.String(),
			From:         &[]int{1}[0],
			To:           &[]int{2}[0],
			UseCache:     &useCacheTrue,
		})
		require.NoError(t, err, "GetRecord with UseCache=true should work")
		assert.NotEmpty(t, result3, "Should return records")

		// Verify results are consistent (all should return the same data)
		assert.Equal(t, len(result1), len(result2), "Results1 and results2 should have same length")
		assert.Equal(t, len(result2), len(result3), "Results2 and results3 should have same length")
	})

	t.Run("GetFirstRecord with UseCache parameter", func(t *testing.T) {
		// Test with various UseCache values
		testCases := []struct {
			name     string
			useCache *bool
		}{
			{"omitted", nil},
			{"false", &[]bool{false}[0]},
			{"true", &[]bool{true}[0]},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := primitiveActions.GetFirstRecord(ctx, types.GetFirstRecordInput{
					DataProvider: streamLocator.DataProvider.Address(),
					StreamId:     streamLocator.StreamId.String(),
					UseCache:     tc.useCache,
				})
				require.NoError(t, err, "GetFirstRecord should work with UseCache %v", tc.useCache)
				assert.NotNil(t, result, "Should return a record")
			})
		}
	})
}

// TestCacheMetadataExtraction tests the cache metadata extraction functionality
func TestCacheMetadataExtraction(t *testing.T) {
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
	streamId := util.GenerateStreamId("test-cache-metadata")
	streamLocator := tnClient.OwnStreamLocator(streamId)

	// Set up cleanup
	t.Cleanup(func() {
		destroyResult, err := tnClient.DestroyStream(ctx, streamId)
		assertNoErrorOrFail(t, err, "Failed to destroy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
	})

	// Deploy stream
	deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
	assertNoErrorOrFail(t, err, "Failed to deploy stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	// Load primitive actions
	primitiveActions, err := tnClient.LoadPrimitiveActions()
	assertNoErrorOrFail(t, err, "Failed to load primitive actions")

	// Insert test data
	insertTxHash, err := primitiveActions.InsertRecord(ctx, types.InsertRecordInput{
		DataProvider: streamLocator.DataProvider.Address(),
		StreamId:     streamLocator.StreamId.String(),
		EventTime:    1,
		Value:        100.50,
	})
	assertNoErrorOrFail(t, err, "Failed to insert record")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, insertTxHash)

	// Wait for data to be processed
	time.Sleep(100 * time.Millisecond)

	t.Run("GetRecordWithMetadata", func(t *testing.T) {
		useCache := true
		result, err := primitiveActions.GetRecordWithMetadata(ctx, types.GetRecordInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamLocator.StreamId.String(),
			From:         &[]int{1}[0],
			To:           &[]int{2}[0],
			UseCache:     &useCache,
		})
		require.NoError(t, err, "GetRecordWithMetadata should work")
		assert.NotEmpty(t, result.Records, "Should return records")

		// Verify metadata is populated
		assert.NotNil(t, result.Metadata, "Metadata should not be nil")
		assert.Equal(t, streamLocator.StreamId.String(), result.Metadata.StreamId, "StreamId should be set")
		assert.Equal(t, streamLocator.DataProvider.Address(), result.Metadata.DataProvider, "DataProvider should be set")
		assert.Equal(t, len(result.Records), result.Metadata.RowsServed, "RowsServed should match record count")

		// Check that metadata fields are properly typed
		assert.IsType(t, bool(false), result.Metadata.CacheHit, "CacheHit should be bool")
		assert.IsType(t, bool(false), result.Metadata.CacheDisabled, "CacheDisabled should be bool")
		if result.Metadata.CachedAt != nil {
			assert.IsType(t, int64(0), *result.Metadata.CachedAt, "CachedAt should be *int64")
		}
	})

	t.Run("GetIndexWithMetadata", func(t *testing.T) {
		useCache := true
		result, err := primitiveActions.GetIndexWithMetadata(ctx, types.GetIndexInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamLocator.StreamId.String(),
			From:         &[]int{1}[0],
			To:           &[]int{2}[0],
			UseCache:     &useCache,
		})
		require.NoError(t, err, "GetIndexWithMetadata should work")

		// Verify metadata structure
		assert.NotNil(t, result.Metadata, "Metadata should not be nil")
		assert.Equal(t, streamLocator.StreamId.String(), result.Metadata.StreamId, "StreamId should be set")
		assert.Equal(t, streamLocator.DataProvider.Address(), result.Metadata.DataProvider, "DataProvider should be set")
	})

	t.Run("GetFirstRecordWithMetadata", func(t *testing.T) {
		useCache := true
		record, metadata, err := primitiveActions.GetFirstRecordWithMetadata(ctx, types.GetFirstRecordInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamLocator.StreamId.String(),
			UseCache:     &useCache,
		})
		require.NoError(t, err, "GetFirstRecordWithMetadata should work")
		assert.NotNil(t, record, "Should return a record")
		assert.NotNil(t, metadata, "Should return metadata")

		// Verify metadata structure
		assert.Equal(t, streamLocator.StreamId.String(), metadata.StreamId, "StreamId should be set")
		assert.Equal(t, streamLocator.DataProvider.Address(), metadata.DataProvider, "DataProvider should be set")
		assert.Equal(t, 1, metadata.RowsServed, "RowsServed should be 1 for single record")
	})
}

// TestCacheMetadataMarshaling tests JSON marshaling/unmarshaling of cache metadata
func TestCacheMetadataMarshaling(t *testing.T) {
	t.Run("CacheMetadata marshaling", func(t *testing.T) {
		metadata := types.CacheMetadata{
			CacheHit:      true,
			CacheDisabled: false,
			CachedAt:      &[]int64{1234567890}[0],
			StreamId:      "test_stream",
			DataProvider:  "0x123456789abcdef",
			From:          &[]int64{1}[0],
			To:            &[]int64{10}[0],
			RowsServed:    5,
		}

		// Test JSON marshaling
		jsonData, err := json.Marshal(metadata)
		require.NoError(t, err, "Should marshal to JSON without error")

		// Test JSON unmarshaling
		var unmarshaledMetadata types.CacheMetadata
		err = json.Unmarshal(jsonData, &unmarshaledMetadata)
		require.NoError(t, err, "Should unmarshal from JSON without error")

		// Verify data integrity
		assert.Equal(t, metadata.CacheHit, unmarshaledMetadata.CacheHit, "CacheHit should be preserved")
		assert.Equal(t, metadata.CacheDisabled, unmarshaledMetadata.CacheDisabled, "CacheDisabled should be preserved")
		assert.Equal(t, metadata.StreamId, unmarshaledMetadata.StreamId, "StreamId should be preserved")
		assert.Equal(t, metadata.DataProvider, unmarshaledMetadata.DataProvider, "DataProvider should be preserved")
		assert.Equal(t, metadata.RowsServed, unmarshaledMetadata.RowsServed, "RowsServed should be preserved")

		if metadata.CachedAt != nil && unmarshaledMetadata.CachedAt != nil {
			assert.Equal(t, *metadata.CachedAt, *unmarshaledMetadata.CachedAt, "CachedAt should be preserved")
		}
		if metadata.From != nil && unmarshaledMetadata.From != nil {
			assert.Equal(t, *metadata.From, *unmarshaledMetadata.From, "From should be preserved")
		}
		if metadata.To != nil && unmarshaledMetadata.To != nil {
			assert.Equal(t, *metadata.To, *unmarshaledMetadata.To, "To should be preserved")
		}
	})
}

// TestCacheMetadataAggregation tests the aggregation of multiple cache metadata entries
func TestCacheMetadataAggregation(t *testing.T) {
	t.Run("AggregateCacheMetadata", func(t *testing.T) {
		metadata1 := types.CacheMetadata{
			CacheHit:   true,
			RowsServed: 5,
		}
		metadata2 := types.CacheMetadata{
			CacheHit:   false,
			RowsServed: 3,
		}
		metadata3 := types.CacheMetadata{
			CacheHit:   true,
			RowsServed: 7,
		}

		aggregated := types.AggregateCacheMetadata([]types.CacheMetadata{
			metadata1, metadata2, metadata3,
		})

		assert.Equal(t, 3, aggregated.TotalQueries, "Should have 3 total queries")
		assert.Equal(t, 2, aggregated.CacheHits, "Should have 2 cache hits")
		assert.Equal(t, 1, aggregated.CacheMisses, "Should have 1 cache miss")
		assert.Equal(t, 15, aggregated.TotalRowsServed, "Should serve 15 total rows")
		
		expectedHitRate := float64(2) / float64(3)
		assert.InDelta(t, expectedHitRate, aggregated.CacheHitRate, 0.001, "Cache hit rate should be 2/3")

		// Verify entries are preserved
		assert.Len(t, aggregated.Entries, 3, "Should preserve all metadata entries")
	})

	t.Run("Empty aggregation", func(t *testing.T) {
		aggregated := types.AggregateCacheMetadata([]types.CacheMetadata{})

		assert.Equal(t, 0, aggregated.TotalQueries, "Should have 0 total queries")
		assert.Equal(t, 0, aggregated.CacheHits, "Should have 0 cache hits")
		assert.Equal(t, 0, aggregated.CacheMisses, "Should have 0 cache misses")
		assert.Equal(t, 0.0, aggregated.CacheHitRate, "Cache hit rate should be 0")
		assert.Len(t, aggregated.Entries, 0, "Should have no entries")
	})
}

// TestBackwardCompatibility ensures existing code continues to work without UseCache
func TestBackwardCompatibility(t *testing.T) {
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
	streamId := util.GenerateStreamId("test-backward-compatibility")
	streamLocator := tnClient.OwnStreamLocator(streamId)

	// Set up cleanup
	t.Cleanup(func() {
		destroyResult, err := tnClient.DestroyStream(ctx, streamId)
		assertNoErrorOrFail(t, err, "Failed to destroy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
	})

	// Deploy stream
	deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
	assertNoErrorOrFail(t, err, "Failed to deploy stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	// Load primitive actions
	primitiveActions, err := tnClient.LoadPrimitiveActions()
	assertNoErrorOrFail(t, err, "Failed to load primitive actions")

	// Insert test data
	insertTxHash, err := primitiveActions.InsertRecord(ctx, types.InsertRecordInput{
		DataProvider: streamLocator.DataProvider.Address(),
		StreamId:     streamLocator.StreamId.String(),
		EventTime:    1,
		Value:        100.50,
	})
	assertNoErrorOrFail(t, err, "Failed to insert record")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, insertTxHash)

	// Wait for data to be processed
	time.Sleep(100 * time.Millisecond)

	t.Run("Old-style API calls should still work", func(t *testing.T) {
		// Test that existing code without UseCache still works
		oldStyleInput := types.GetRecordInput{
			DataProvider: streamLocator.DataProvider.Address(),
			StreamId:     streamLocator.StreamId.String(),
			From:         &[]int{1}[0],
			To:           &[]int{2}[0],
			// No UseCache field - should work with existing code
		}

		result, err := primitiveActions.GetRecord(ctx, oldStyleInput)
		require.NoError(t, err, "Old-style GetRecord should work")
		assert.NotEmpty(t, result, "Should return records")

		// Test interface compatibility
		var actionInterface types.IAction = primitiveActions
		_, err = actionInterface.GetRecord(ctx, oldStyleInput)
		require.NoError(t, err, "Interface should be compatible")
	})
}

// TestCacheMetadataDataAge tests the GetDataAge functionality
func TestCacheMetadataDataAge(t *testing.T) {
	t.Run("GetDataAge with valid timestamp", func(t *testing.T) {
		now := time.Now()
		cachedAt := now.Unix() - 300 // 5 minutes ago
		
		metadata := types.CacheMetadata{
			CachedAt: &cachedAt,
		}

		dataAge := metadata.GetDataAge()
		require.NotNil(t, dataAge, "DataAge should not be nil")
		
		// Should be approximately 5 minutes (allowing for test execution time)
		assert.Greater(t, dataAge.Minutes(), 4.9, "Data age should be at least 4.9 minutes")
		assert.Less(t, dataAge.Minutes(), 5.1, "Data age should be less than 5.1 minutes")
	})

	t.Run("GetDataAge with nil timestamp", func(t *testing.T) {
		metadata := types.CacheMetadata{
			CachedAt: nil,
		}

		dataAge := metadata.GetDataAge()
		assert.Nil(t, dataAge, "DataAge should be nil when CachedAt is nil")
	})

	t.Run("IsExpired functionality", func(t *testing.T) {
		now := time.Now()
		
		// Test with old data (expired)
		oldCachedAt := now.Unix() - 3600 // 1 hour ago
		oldMetadata := types.CacheMetadata{
			CachedAt: &oldCachedAt,
		}
		assert.True(t, oldMetadata.IsExpired(30*time.Minute), "1-hour-old data should be expired after 30 minutes")

		// Test with fresh data (not expired)
		freshCachedAt := now.Unix() - 60 // 1 minute ago
		freshMetadata := types.CacheMetadata{
			CachedAt: &freshCachedAt,
		}
		assert.False(t, freshMetadata.IsExpired(30*time.Minute), "1-minute-old data should not be expired after 30 minutes")

		// Test with nil timestamp (not expired)
		nilMetadata := types.CacheMetadata{
			CachedAt: nil,
		}
		assert.False(t, nilMetadata.IsExpired(30*time.Minute), "Data with nil timestamp should not be considered expired")
	})
}