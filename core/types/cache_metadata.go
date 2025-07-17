package types

import (
	"encoding/json"
	"time"
)

// CacheMetadata represents cache performance and hit/miss statistics
// Based on actual tn_cache extension implementation fields from NOTICE statements
type CacheMetadata struct {
	// Cache hit/miss statistics (from NOTICE statements in helper_check_cache)
	CacheHit      bool `json:"cache_hit"`
	CacheDisabled bool `json:"cache_disabled,omitempty"`
	
	// Cache timing information (from tn_cache functions)
	CachedAt      *int64 `json:"cached_at,omitempty"`        // Timestamp when data was cached
	
	// SDK-provided context (not from logs, but added by SDK)
	StreamId     string `json:"stream_id,omitempty"`
	DataProvider string `json:"data_provider,omitempty"`
	From         *int64 `json:"from,omitempty"`
	To           *int64 `json:"to,omitempty"`
	FrozenAt     *int64 `json:"frozen_at,omitempty"`
	RowsServed   int    `json:"rows_served,omitempty"`       // Number of rows returned (calculated by SDK)
}

// CacheMetadataCollection represents aggregated cache metadata from multiple operations
type CacheMetadataCollection struct {
	// Aggregated statistics
	TotalQueries    int     `json:"total_queries"`
	CacheHits       int     `json:"cache_hits"`
	CacheMisses     int     `json:"cache_misses"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
	
	// Timing aggregates
	TotalRowsServed int `json:"total_rows_served"`
	
	// Individual metadata entries
	Entries []CacheMetadata `json:"entries"`
}

// StreamRecordWithMetadata extends StreamRecord with cache metadata
type StreamRecordWithMetadata struct {
	Records  []StreamRecord  `json:"records"`
	Metadata CacheMetadata   `json:"cache_metadata"`
}

// StreamIndexWithMetadata extends StreamIndex with cache metadata
type StreamIndexWithMetadata struct {
	Indices  []StreamIndex   `json:"indices"`
	Metadata CacheMetadata   `json:"cache_metadata"`
}

// StreamIndexChangeWithMetadata extends StreamIndexChange with cache metadata
type StreamIndexChangeWithMetadata struct {
	IndexChanges []StreamIndexChange `json:"index_changes"`
	Metadata     CacheMetadata       `json:"cache_metadata"`
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
			
			// Extract cache timestamp
			if cachedAtFloat, ok := logData["cached_at"].(float64); ok {
				cachedAt := int64(cachedAtFloat)
				metadata.CachedAt = &cachedAt
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

// GetDataAge calculates the age of cached data in human-readable format
func (cm *CacheMetadata) GetDataAge() *time.Duration {
	if cm.CachedAt == nil {
		return nil
	}
	
	age := time.Since(time.Unix(*cm.CachedAt, 0))
	return &age
}

// IsExpired checks if cached data is older than the specified duration
func (cm *CacheMetadata) IsExpired(maxAge time.Duration) bool {
	dataAge := cm.GetDataAge()
	if dataAge == nil {
		return false // No cache timestamp, cannot determine expiration
	}
	
	return *dataAge >= maxAge
}