package contractsapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
)

// GetMarketActivity calls get_market_activity and parses the single row it
// returns. The node action holds the definition of volume; what is left here is
// the argument order and the parse, both checked without a node.

// marketActivityColumnNames is the action's column order, which is the order
// parseMarketActivityRow reads.
var marketActivityColumnNames = []string{
	"bridge", "volume_cents", "direct_cents", "mint_burn_cents",
	"unique_traders", "fill_count", "direct_fill_count", "shares_traded",
	"first_event_ts", "last_event_ts", "coverage_from_block", "coverage_complete",
}

// market782Row returns market 782's row over [1700000000, 1800000000] as mainnet
// answered on 2026-10-08, in the form a call result carries: numbers as strings,
// the coverage flag as a JSON bool.
func market782Row() []any {
	return []any{
		"eth_usdc", "1604", "1504", "100",
		"2", "17", "16", "32",
		"1790491930", "1791025147", "2664626", false,
	}
}

// int64Ptr returns a pointer to v, for the nullable fill times.
func int64Ptr(v int64) *int64 { return &v }

// TestParseMarketActivityRow checks the parse of each of the action's 12 columns,
// and that a short row, a wrong-typed column or a NULL where none is allowed is an
// error naming the column.
func TestParseMarketActivityRow(t *testing.T) {
	t.Run("maps every column the action returns", func(t *testing.T) {
		activity, err := parseMarketActivityRow(market782Row())

		require.NoError(t, err)
		assert.Equal(t, &types.MarketActivity{
			Bridge:            "eth_usdc",
			VolumeCents:       "1604",
			DirectCents:       "1504",
			MintBurnCents:     "100",
			UniqueTraders:     2,
			FillCount:         17,
			DirectFillCount:   16,
			SharesTraded:      32,
			FirstEventTs:      int64Ptr(1790491930),
			LastEventTs:       int64Ptr(1791025147),
			CoverageFromBlock: 2664626,
			CoverageComplete:  false,
		}, activity)
	})

	t.Run("keeps a volume above int64 as the exact string", func(t *testing.T) {
		row := market782Row()
		row[1] = "92233720368547758070000"
		row[2] = "92233720368547758069900"

		activity, err := parseMarketActivityRow(row)

		require.NoError(t, err)
		assert.Equal(t, "92233720368547758070000", activity.VolumeCents)
		assert.Equal(t, "92233720368547758069900", activity.DirectCents)
	})

	t.Run("maps NULL timestamps to nil pointers when nothing traded", func(t *testing.T) {
		// Market 1176 in the same window: its fills were trimmed, so the row is
		// zeros with no first or last fill.
		row := []any{
			"eth_usdc", "0", "0", "0",
			"0", "0", "0", "0",
			nil, nil, "2664626", false,
		}

		activity, err := parseMarketActivityRow(row)

		require.NoError(t, err)
		assert.Nil(t, activity.FirstEventTs)
		assert.Nil(t, activity.LastEventTs)
		assert.Equal(t, "0", activity.VolumeCents)
		assert.False(t, activity.CoverageComplete)
	})

	t.Run("reads complete coverage", func(t *testing.T) {
		row := market782Row()
		row[11] = true

		activity, err := parseMarketActivityRow(row)

		require.NoError(t, err)
		assert.True(t, activity.CoverageComplete)
	})

	t.Run("reads a node that holds no order events", func(t *testing.T) {
		// The action reports coverage from block 0, incomplete, rather than NULL.
		row := market782Row()
		row[10] = "0"

		activity, err := parseMarketActivityRow(row)

		require.NoError(t, err)
		assert.Equal(t, int64(0), activity.CoverageFromBlock)
		assert.False(t, activity.CoverageComplete)
	})

	t.Run("rejects a row that is missing a column", func(t *testing.T) {
		_, err := parseMarketActivityRow(market782Row()[:11])

		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected 12 columns, got 11")
	})

	t.Run("rejects a wrong-typed column and names it", func(t *testing.T) {
		for i, name := range marketActivityColumnNames {
			row := market782Row()
			switch row[i].(type) {
			case string:
				// A string column holding a bool can parse as none of the types
				// the row uses.
				row[i] = true
			case bool:
				row[i] = int64(1)
			}

			_, err := parseMarketActivityRow(row)

			// Whole word, so fill_count is not satisfied by direct_fill_count.
			require.Error(t, err, "column %d (%s)", i, name)
			assert.Regexp(t, `\b`+name+`\b`, err.Error(), "column %d", i)
		}
	})

	t.Run("rejects NULL in every column but the fill times", func(t *testing.T) {
		for i, name := range marketActivityColumnNames {
			if name == "first_event_ts" || name == "last_event_ts" {
				continue
			}
			row := market782Row()
			row[i] = nil

			_, err := parseMarketActivityRow(row)

			require.Error(t, err, "column %d (%s)", i, name)
			assert.Regexp(t, `\b`+name+`\b`, err.Error(), "column %d", i)
		}
	})
}

// TestMarketActivityFromResult checks what each row count means: one row is the
// market's activity, no rows is a market that does not exist, and a nil result or
// two rows are errors.
func TestMarketActivityFromResult(t *testing.T) {
	t.Run("one row is the market's activity", func(t *testing.T) {
		activity, err := marketActivityFromResult(782, &kwiltypes.QueryResult{
			ColumnNames: marketActivityColumnNames,
			Values:      [][]any{market782Row()},
		})

		require.NoError(t, err)
		assert.Equal(t, "1604", activity.VolumeCents)
	})

	t.Run("no rows means the market does not exist", func(t *testing.T) {
		_, err := marketActivityFromResult(999999999, &kwiltypes.QueryResult{})

		require.Error(t, err)
		assert.Equal(t, "market 999999999 not found", err.Error())
	})

	t.Run("a nil result is an error, not a missing market", func(t *testing.T) {
		_, err := marketActivityFromResult(782, nil)

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "not found")
	})

	t.Run("two rows is an error", func(t *testing.T) {
		_, err := marketActivityFromResult(782, &kwiltypes.QueryResult{
			Values: [][]any{market782Row(), market782Row()},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "2 rows")
	})
}

// TestMarketActivityArgs checks that the arguments go in the action's parameter
// order.
func TestMarketActivityArgs(t *testing.T) {
	// get_market_activity($query_id INT, $from_ts INT8, $to_ts INT8): swapping
	// the two times would read an inverted window, which the action refuses.
	args := marketActivityArgs(types.GetMarketActivityInput{QueryID: 782, FromTs: 1, ToTs: 2})

	assert.Equal(t, []any{782, int64(1), int64(2)}, args)
}
