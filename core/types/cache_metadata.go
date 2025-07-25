package types

import (
	"encoding/json"
)

// CacheMetadata represents cache performance and hit/miss statistics
// Based on actual tn_cache extension implementation fields from NOTICE statements
type CacheMetadata struct {
	// Cache hit/miss statistics (from NOTICE statements in helper_check_cache)
	CacheHit      bool `json:"cache_hit"`
	CacheDisabled bool `json:"cache_disabled,omitempty"`

	// Cache timing information (from tn_cache functions)
	CacheHeight *int64 `json:"cache_height,omitempty"` // Block height when data was cached

	// SDK-provided context (not from logs, but added by SDK)
	StreamId     string `json:"stream_id,omitempty"`
	DataProvider string `json:"data_provider,omitempty"`
	From         *int64 `json:"from,omitempty"`
	To           *int64 `json:"to,omitempty"`
	FrozenAt     *int64 `json:"frozen_at,omitempty"`
	RowsServed   int    `json:"rows_served,omitempty"` // Number of rows returned (calculated by SDK)
}

// CacheMetadataCollection represents aggregated cache metadata from multiple operations
type CacheMetadataCollection struct {
	// Aggregated statistics
	TotalQueries int     `json:"total_queries"`
	CacheHits    int     `json:"cache_hits"`
	CacheMisses  int     `json:"cache_misses"`
	CacheHitRate float64 `json:"cache_hit_rate"`

	// Timing aggregates
	TotalRowsServed int `json:"total_rows_served"`

	// Individual metadata entries
	Entries []CacheMetadata `json:"entries"`
}

// ActionResult extends StreamResult with cache metadata
type ActionResult struct {
	Results  []StreamResult `json:"results"`
	Metadata CacheMetadata  `json:"cache_metadata"`
}

// ParseCacheMetadata extracts cache metadata from action logs
func ParseCacheMetadata(logs []string) (CacheMetadata, error) {
	metadata := CacheMetadata{}

	for _, log := range logs {
		// Look for JSON formatted cache logs
		if len(log) > 0 && (log[0] == '{' || log[0] == '[') {
			var logData map[string]interface{}
			if err := json.Unmarshal([]byte(log), &logData); err != nil {
				continue // Skip non-JSON logs
			}

			// Extract cache hit/miss information
			if cacheHit, ok := logData["cache_hit"].(bool); ok {
				metadata.CacheHit = cacheHit
			}

			if cacheDisabled, ok := logData["cache_disabled"].(bool); ok {
				metadata.CacheDisabled = cacheDisabled
			}

			// Extract cache height (matching SQL field name)
			if heightFloat, ok := logData["cache_height"].(float64); ok {
				height := int64(heightFloat)
				metadata.CacheHeight = &height
			}
		}
	}

	return metadata, nil
}

// AggregateCacheMetadata combines multiple cache metadata entries
func AggregateCacheMetadata(metadataList []CacheMetadata) CacheMetadataCollection {
	collection := CacheMetadataCollection{
		Entries: metadataList,
	}

	totalQueries := len(metadataList)
	cacheHits := 0
	totalRowsServed := 0

	for _, metadata := range metadataList {
		if metadata.CacheHit {
			cacheHits++
		}

		if metadata.RowsServed > 0 {
			totalRowsServed += metadata.RowsServed
		}
	}

	collection.TotalQueries = totalQueries
	collection.CacheHits = cacheHits
	collection.CacheMisses = totalQueries - cacheHits
	collection.TotalRowsServed = totalRowsServed

	if totalQueries > 0 {
		collection.CacheHitRate = float64(cacheHits) / float64(totalQueries)
	}

	return collection
}
