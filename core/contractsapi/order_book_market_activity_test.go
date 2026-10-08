package contractsapi

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
)

// GetMarketActivity is one ad hoc statement and a parse of the single row it
// returns. The statement's text is the volume definition, and the parse depends
// on the order of its columns, so both are checked here without a node.
//
// The row below is market 782 as the mainnet gateway returned it on 2026-10-08
// for the window [1700000000, 1800000000]: numbers arrive as strings, the
// coverage flag as a JSON bool.

// marketActivityColumnNames is the statement's column order, which is the order
// parseMarketActivityRow reads.
var marketActivityColumnNames = []string{
	"bridge", "volume_cents", "direct_cents", "mint_burn_cents",
	"unique_traders", "fill_count", "direct_fill_count", "shares_traded",
	"first_event_ts", "last_event_ts", "coverage_from_block", "coverage_complete",
}

func market782Row() []any {
	return []any{
		"eth_usdc", "1604", "1504", "100",
		"2", "17", "16", "32",
		"1790491930", "1791025147", "2664626", false,
	}
}

func int64Ptr(v int64) *int64 { return &v }

func TestParseMarketActivityRow(t *testing.T) {
	t.Run("maps every column the statement returns", func(t *testing.T) {
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

	t.Run("reads a node that holds no order events as incomplete coverage", func(t *testing.T) {
		// MIN over an empty ob_order_events is NULL, and so is the comparison
		// built on it. Nothing says whether that market lost fills, so a zero
		// from it must not be trusted.
		row := market782Row()
		row[10] = nil
		row[11] = nil

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
}

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
		// The gateway answers a missing market with null columns and values.
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

func TestMarketActivitySQL(t *testing.T) {
	t.Run("counts volume over an allowlist of fill events", func(t *testing.T) {
		for _, event := range []string{"'direct_buy_fill'", "'mint_fill'", "'burn_fill'"} {
			assert.Contains(t, marketActivitySQL, event)
		}
		// A split is a deposit with no counterparty, never volume.
		assert.NotContains(t, marketActivitySQL, "'split_placed'")
	})

	t.Run("returns its columns in the order the parser reads them", func(t *testing.T) {
		// The first column is q.bridge, which takes its name from the column;
		// every other column is aliased.
		aliases := regexp.MustCompile(`\bAS (\w+)`).FindAllStringSubmatch(marketActivitySQL, -1)
		got := []string{"bridge"}
		for _, m := range aliases {
			got = append(got, m[1])
		}

		require.True(t, strings.HasPrefix(strings.TrimSpace(marketActivitySQL), "SELECT\n  q.bridge,"))
		assert.Equal(t, marketActivityColumnNames, got)
	})

	t.Run("binds exactly the parameters the statement uses", func(t *testing.T) {
		params := marketActivityParams(types.GetMarketActivityInput{QueryID: 782, FromTs: 1, ToTs: 2})

		var bound []string
		for key := range params {
			bound = append(bound, key)
		}
		sort.Strings(bound)

		used := map[string]bool{}
		for _, p := range regexp.MustCompile(`\$\w+`).FindAllString(marketActivitySQL, -1) {
			used[p] = true
		}
		var usedKeys []string
		for p := range used {
			usedKeys = append(usedKeys, p)
		}
		sort.Strings(usedKeys)

		assert.Equal(t, usedKeys, bound)
		assert.Equal(t, 782, params["$query_id"])
		assert.Equal(t, int64(1), params["$from_ts"])
		assert.Equal(t, int64(2), params["$to_ts"])
	})
}
