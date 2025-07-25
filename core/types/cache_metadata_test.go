package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCacheMetadata(t *testing.T) {
	tests := []struct {
		name      string
		logs      []string
		expected  CacheMetadata
		expectErr bool
	}{
		{
			name: "Successful parse with cache hit and height",
			logs: []string{
				`{"cache_hit": true, "cache_height": 1000}`,
			},
			expected: CacheMetadata{
				CacheHit:    true,
				CacheHeight: int64Ptr(1000),
			},
			expectErr: false,
		},
		{
			name: "Parse cache miss",
			logs: []string{
				`{"cache_hit": false}`,
			},
			expected: CacheMetadata{
				CacheHit: false,
			},
			expectErr: false,
		},
		{
			name: "Parse cache disabled",
			logs: []string{
				`{"cache_disabled": true}`,
			},
			expected: CacheMetadata{
				CacheDisabled: true,
			},
			expectErr: false,
		},
		{
			name: "Invalid JSON log",
			logs: []string{
				"invalid json",
			},
			expected:  CacheMetadata{},
			expectErr: false, // Should skip invalid logs
		},
		{
			name:      "No logs",
			logs:      []string{},
			expected:  CacheMetadata{},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCacheMetadata(tt.logs)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
