package contractsapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/types"
)

// get_full_market_depth returns both outcomes' ladders in one result set, tagged
// with the outcome each level rests on. Everything between that result set and a
// consolidated book is parsing and a split, so both are testable without a node.
//
// The split is where a mistake would be invisible: putting the requested
// outcome's levels on the opposite pile still produces a plausible-looking book,
// with every quote on the wrong side of the ladder.

func TestParseFullDepthLevelRow(t *testing.T) {
	t.Run("reads the four columns in the order the action returns them", func(t *testing.T) {
		level, err := parseFullDepthLevelRow([]any{true, int64(55), int64(150), int64(0)})

		require.NoError(t, err)
		assert.Equal(t, types.FullDepthLevel{
			Outcome: true, Price: 55, BuyVolume: 150, SellVolume: 0,
		}, level)
	})

	t.Run("keeps a NO level tagged NO", func(t *testing.T) {
		level, err := parseFullDepthLevelRow([]any{false, int64(40), int64(0), int64(100)})

		require.NoError(t, err)
		assert.Equal(t, types.FullDepthLevel{
			Outcome: false, Price: 40, BuyVolume: 0, SellVolume: 100,
		}, level)
	})

	t.Run("accepts the string volumes the gateway can send", func(t *testing.T) {
		level, err := parseFullDepthLevelRow([]any{true, "55", "150", "25"})

		require.NoError(t, err)
		assert.Equal(t, types.FullDepthLevel{
			Outcome: true, Price: 55, BuyVolume: 150, SellVolume: 25,
		}, level)
	})

	t.Run("rejects a row that is missing a column", func(t *testing.T) {
		_, err := parseFullDepthLevelRow([]any{true, int64(55), int64(150)})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected 4 columns")
	})
}

func TestSplitFullDepth(t *testing.T) {
	depth := []types.FullDepthLevel{
		{Outcome: true, Price: 55, BuyVolume: 150},
		{Outcome: false, Price: 30, BuyVolume: 200},
		{Outcome: false, Price: 40, SellVolume: 100},
	}

	t.Run("in the YES frame the NO levels are the opposite ladder", func(t *testing.T) {
		native, opposite := splitFullDepth(depth, true)

		assert.Equal(t, []types.DepthLevel{{Price: 55, BuyVolume: 150}}, native)
		assert.Equal(t, []types.DepthLevel{
			{Price: 30, BuyVolume: 200},
			{Price: 40, SellVolume: 100},
		}, opposite)
	})

	t.Run("in the NO frame the two piles swap", func(t *testing.T) {
		native, opposite := splitFullDepth(depth, false)

		assert.Equal(t, []types.DepthLevel{
			{Price: 30, BuyVolume: 200},
			{Price: 40, SellVolume: 100},
		}, native)
		assert.Equal(t, []types.DepthLevel{{Price: 55, BuyVolume: 150}}, opposite)
	})

	t.Run("an outcome quoted on neither side yields two empty ladders", func(t *testing.T) {
		native, opposite := splitFullDepth(nil, true)

		assert.Empty(t, native)
		assert.Empty(t, opposite)
	})
}
