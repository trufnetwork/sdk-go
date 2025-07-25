package unit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/types"
)

// TestParseLogMetadata tests parsing cache metadata from real log strings
func TestParseLogMetadata(t *testing.T) {
	t.Run("Parse cache hit log", func(t *testing.T) {
		logs := []string{
			`{"cache_hit": true, "cache_height": 12345}`,
		}

		metadata, err := types.ParseCacheMetadata(logs)
		require.NoError(t, err)

		assert.True(t, metadata.CacheHit, "Should parse cache hit as true")
		assert.False(t, metadata.CacheDisabled, "Should not be disabled")
		require.NotNil(t, metadata.CacheHeight, "CacheHeight should not be nil for cache hit")
		assert.Equal(t, int64(12345), *metadata.CacheHeight, "Should parse cache height")
	})

	t.Run("Parse cache miss log", func(t *testing.T) {
		logs := []string{
			`{"cache_hit": false}`,
		}

		metadata, err := types.ParseCacheMetadata(logs)
		require.NoError(t, err)

		assert.False(t, metadata.CacheHit, "Should parse cache hit as false")
	})

	t.Run("Parse cache disabled log", func(t *testing.T) {
		logs := []string{
			`{"cache_disabled": true}`,
		}

		metadata, err := types.ParseCacheMetadata(logs)
		require.NoError(t, err)

		assert.False(t, metadata.CacheHit, "Should not be cache hit")
		assert.True(t, metadata.CacheDisabled, "Should be disabled")
	})

	t.Run("Parse multiple logs", func(t *testing.T) {
		logs := []string{
			`some non-json log line`,
			`{"cache_hit": true, "cache_height": 9876}`,
			`another non-json line`,
		}

		metadata, err := types.ParseCacheMetadata(logs)
		require.NoError(t, err)

		assert.True(t, metadata.CacheHit, "Should parse cache hit from valid JSON")
		require.NotNil(t, metadata.CacheHeight, "CacheHeight should not be nil for cache hit")
		assert.Equal(t, int64(9876), *metadata.CacheHeight, "Should parse cache height")
	})

	t.Run("Parse empty logs", func(t *testing.T) {
		logs := []string{}

		metadata, err := types.ParseCacheMetadata(logs)
		require.NoError(t, err)

		// Should return zero-value metadata
		assert.False(t, metadata.CacheHit)
		assert.False(t, metadata.CacheDisabled)
	})
}

// TestCacheMetadataJSON tests JSON marshaling with actual field names
func TestCacheMetadataJSON(t *testing.T) {
	t.Run("Marshal and unmarshal complete metadata", func(t *testing.T) {
		original := types.CacheMetadata{
			CacheHit:      true,
			CacheDisabled: false,
			StreamId:      "test_stream_id",
			DataProvider:  "0x1234567890abcdef",
			From:          &[]int64{1}[0],
			To:            &[]int64{100}[0],
			FrozenAt:      &[]int64{50}[0],
			RowsServed:    42,
		}

		// Marshal to JSON
		jsonBytes, err := json.Marshal(original)
		require.NoError(t, err)

		// Verify the JSON contains the expected field names
		jsonStr := string(jsonBytes)
		assert.Contains(t, jsonStr, `"cache_hit":true`, "Should contain cache_hit field")
		assert.Contains(t, jsonStr, `"stream_id":"test_stream_id"`, "Should contain stream_id field")
		assert.Contains(t, jsonStr, `"rows_served":42`, "Should contain rows_served field")

		// Unmarshal back to struct
		var unmarshaled types.CacheMetadata
		err = json.Unmarshal(jsonBytes, &unmarshaled)
		require.NoError(t, err)

		// Verify all fields are preserved
		assert.Equal(t, original.CacheHit, unmarshaled.CacheHit)
		assert.Equal(t, original.CacheDisabled, unmarshaled.CacheDisabled)
		assert.Equal(t, original.StreamId, unmarshaled.StreamId)
		assert.Equal(t, original.DataProvider, unmarshaled.DataProvider)
		assert.Equal(t, original.RowsServed, unmarshaled.RowsServed)

		require.NotNil(t, unmarshaled.From)
		assert.Equal(t, *original.From, *unmarshaled.From)

		require.NotNil(t, unmarshaled.To)
		assert.Equal(t, *original.To, *unmarshaled.To)
	})
}

// TestAggregation tests the cache metadata aggregation logic
func TestAggregation(t *testing.T) {
	t.Run("Aggregate mixed hit/miss metadata", func(t *testing.T) {
		metadataList := []types.CacheMetadata{
			{CacheHit: true, RowsServed: 10},
			{CacheHit: false, RowsServed: 5},
			{CacheHit: true, RowsServed: 15},
			{CacheHit: false, RowsServed: 3},
		}

		result := types.AggregateCacheMetadata(metadataList)

		assert.Equal(t, 4, result.TotalQueries, "Should count all queries")
		assert.Equal(t, 2, result.CacheHits, "Should count hits correctly")
		assert.Equal(t, 2, result.CacheMisses, "Should count misses correctly")
		assert.Equal(t, 0.5, result.CacheHitRate, "Hit rate should be 50%")
		assert.Equal(t, 33, result.TotalRowsServed, "Should sum all rows served")
		assert.Len(t, result.Entries, 4, "Should preserve all entries")
	})

	t.Run("Aggregate all hits", func(t *testing.T) {
		metadataList := []types.CacheMetadata{
			{CacheHit: true, RowsServed: 10},
			{CacheHit: true, RowsServed: 20},
		}

		result := types.AggregateCacheMetadata(metadataList)

		assert.Equal(t, 2, result.TotalQueries)
		assert.Equal(t, 2, result.CacheHits)
		assert.Equal(t, 0, result.CacheMisses)
		assert.Equal(t, 1.0, result.CacheHitRate, "Hit rate should be 100%")
		assert.Equal(t, 30, result.TotalRowsServed)
	})

	t.Run("Aggregate all misses", func(t *testing.T) {
		metadataList := []types.CacheMetadata{
			{CacheHit: false, RowsServed: 5},
			{CacheHit: false, RowsServed: 3},
		}

		result := types.AggregateCacheMetadata(metadataList)

		assert.Equal(t, 2, result.TotalQueries)
		assert.Equal(t, 0, result.CacheHits)
		assert.Equal(t, 2, result.CacheMisses)
		assert.Equal(t, 0.0, result.CacheHitRate, "Hit rate should be 0%")
		assert.Equal(t, 8, result.TotalRowsServed)
	})
}
