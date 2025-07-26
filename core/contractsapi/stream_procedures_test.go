package contractsapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLogsForMetadata(t *testing.T) {
	t.Run("prepended numeric logs", func(t *testing.T) {
		input := "1. cache_hit: true\n2. other log\n111. cache_miss: false"
		expected := []string{"cache_hit: true", "other log", "cache_miss: false"}

		result, err := parseLogsForMetadata(input)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("empty logs", func(t *testing.T) {
		input := ""

		result, err := parseLogsForMetadata(input)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})
}
