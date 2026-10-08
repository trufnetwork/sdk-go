// Live-network check that GetMarketActivity reads get_market_activity from a real
// node and that the figures it returns hang together.
//
// The unit tests under core/contractsapi prove the argument order and the parse
// of a captured row, and the node's own tests prove the action's definition on a
// local chain. What they cannot prove is that a live network serves the action
// to this SDK and filters the window the way the definition says. That is the
// gap this file covers.
//
// Gated on an env var rather than the kwiltest build tag, because it wants a
// network carrying markets with real fills rather than a fresh local node:
//
//	TN_LIVE_ENDPOINT=https://gateway.mainnet.truf.network \
//	    go test ./tests/integration/ -run TestMarketActivityLive -v
//
// The network must be running a node that carries get_market_activity (node
// migration 058).
//
// The action is a view, so the signing key needs no funds and controls nothing.
//
// These assert INVARIANTS, not numbers. The node trims order events once they
// are indexed, so figures pinned here would drift.

package integration

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

const (
	// activityWindow is how far back the test looks for fills.
	activityWindow = 30 * 24 * time.Hour

	// maxMarketsForActivity bounds discovery, which reads the action once per
	// market, newest market first.
	maxMarketsForActivity = 100
)

func TestMarketActivityLive(t *testing.T) {
	endpoint := os.Getenv("TN_LIVE_ENDPOINT")
	if endpoint == "" {
		t.Skip("live network not configured; set TN_LIVE_ENDPOINT to run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	key, err := kwilcrypto.Secp256k1PrivateKeyFromHex(liveReadOnlyPK)
	require.NoError(t, err, "failed to parse the read-only key")

	client, err := tnclient.NewClient(ctx, endpoint, tnclient.WithSigner(auth.GetUserSigner(key)))
	require.NoError(t, err, "failed to create client")

	ob, err := client.LoadOrderBook()
	require.NoError(t, err, "failed to load order book")

	toTs := time.Now().Unix()
	fromTs := toTs - int64(activityWindow/time.Second)

	queryID, activity, found := findMarketWithFills(ctx, t, ob, fromTs, toTs)
	if !found {
		t.Skipf("none of the newest %d markets has a fill in the last %s",
			maxMarketsForActivity, activityWindow)
	}
	t.Logf("market %d, window [%d, %d]: %s", queryID, fromTs, toTs, describeActivity(activity))

	t.Run("the figures hang together", func(t *testing.T) {
		assert.NotEmpty(t, activity.Bridge)

		volume := parseCents(t, "volume_cents", activity.VolumeCents)
		direct := parseCents(t, "direct_cents", activity.DirectCents)
		mintBurn := parseCents(t, "mint_burn_cents", activity.MintBurnCents)
		assert.Zero(t, volume.Cmp(new(big.Int).Add(direct, mintBurn)),
			"volume %s is not direct %s + mint/burn %s", volume, direct, mintBurn)

		assert.LessOrEqual(t, activity.DirectFillCount, activity.FillCount)
		assert.GreaterOrEqual(t, activity.UniqueTraders, 1)

		require.NotNil(t, activity.FirstEventTs)
		require.NotNil(t, activity.LastEventTs)
		assert.GreaterOrEqual(t, *activity.FirstEventTs, fromTs)
		assert.LessOrEqual(t, *activity.FirstEventTs, *activity.LastEventTs)
		assert.LessOrEqual(t, *activity.LastEventTs, toTs)
	})

	t.Run("the window includes both of its ends", func(t *testing.T) {
		checkWindowEnds(ctx, t, ob, queryID, fromTs, toTs)
	})

	t.Run("a market that does not exist is an error", func(t *testing.T) {
		_, err := ob.GetMarketActivity(ctx, types.GetMarketActivityInput{
			QueryID: math.MaxInt32, FromTs: fromTs, ToTs: toTs,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// checkWindowEnds narrows the window to the first and last fill inside it. The
// narrow window must count exactly the same fills, which holds only if both
// bounds are inclusive, and the seconds just outside it must count none.
//
// Each read is its own query, and on a live market a fill can land, or a trim
// can drop old events, between them. Re-reading the full window afterwards and
// requiring the same fills closes that: if the full window is unchanged either
// side of the sequence, the reads in between saw the same fills.
func checkWindowEnds(
	ctx context.Context, t *testing.T, ob types.IOrderBook, queryID int, fromTs, toTs int64,
) {
	t.Helper()

	for attempt := 1; attempt <= stableReadAttempts; attempt++ {
		before := readActivity(ctx, t, ob, queryID, fromTs, toTs)
		if before.FirstEventTs == nil {
			t.Skipf("market %d lost its fills in the window before the check ran", queryID)
		}
		first, last := *before.FirstEventTs, *before.LastEventTs

		narrow := readActivity(ctx, t, ob, queryID, first, last)
		var head, tail *types.MarketActivity
		if first > fromTs {
			head = readActivity(ctx, t, ob, queryID, fromTs, first-1)
		}
		if last < toTs {
			tail = readActivity(ctx, t, ob, queryID, last+1, toTs)
		}

		after := readActivity(ctx, t, ob, queryID, fromTs, toTs)
		if fillsOf(before) != fillsOf(after) {
			t.Logf("market %d moved during attempt %d; re-reading", queryID, attempt)
			continue
		}

		assert.Equal(t, fillsOf(before), fillsOf(narrow), "[first, last] must count every fill")
		if head != nil {
			assert.Equal(t, 0, head.FillCount, "no fill can come before the first one")
			assert.Nil(t, head.FirstEventTs)
		}
		if tail != nil {
			assert.Equal(t, 0, tail.FillCount, "no fill can come after the last one")
			assert.Nil(t, tail.LastEventTs)
		}
		return
	}

	t.Skipf("market %d kept moving across %d attempts; nothing to compare against",
		queryID, stableReadAttempts)
}

// findMarketWithFills returns the newest market with a fill inside the window,
// with its activity over that window.
func findMarketWithFills(
	ctx context.Context, t *testing.T, ob types.IOrderBook, fromTs, toTs int64,
) (int, *types.MarketActivity, bool) {
	t.Helper()

	offset, scanned := 0, 0
	for scanned < maxMarketsForActivity {
		limit := 100
		page, err := ob.ListMarkets(ctx, types.ListMarketsInput{Limit: &limit, Offset: &offset})
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}

		for _, summary := range page {
			if scanned >= maxMarketsForActivity {
				break
			}
			scanned++

			activity := readActivity(ctx, t, ob, summary.ID, fromTs, toTs)
			if activity.FillCount > 0 {
				t.Logf("found after %d markets", scanned)
				return summary.ID, activity, true
			}
		}

		if len(page) < limit {
			break
		}
		offset += limit
	}

	return 0, nil, false
}

func readActivity(
	ctx context.Context, t *testing.T, ob types.IOrderBook, queryID int, fromTs, toTs int64,
) *types.MarketActivity {
	t.Helper()

	activity, err := ob.GetMarketActivity(ctx, types.GetMarketActivityInput{
		QueryID: queryID, FromTs: fromTs, ToTs: toTs,
	})
	require.NoError(t, err, "market %d, window [%d, %d]", queryID, fromTs, toTs)
	return activity
}

// fills is the part of a market's activity that depends only on the fills in
// the window. Coverage is left out: it moves whenever the node trims old events.
type fills struct {
	volume, direct, mintBurn            string
	traders, fillCount, directFillCount int
	shares                              int64
	first, last                         int64
	hasFirst, hasLast                   bool
}

func fillsOf(a *types.MarketActivity) fills {
	f := fills{
		volume: a.VolumeCents, direct: a.DirectCents, mintBurn: a.MintBurnCents,
		traders: a.UniqueTraders, fillCount: a.FillCount, directFillCount: a.DirectFillCount,
		shares: a.SharesTraded,
	}
	if a.FirstEventTs != nil {
		f.first, f.hasFirst = *a.FirstEventTs, true
	}
	if a.LastEventTs != nil {
		f.last, f.hasLast = *a.LastEventTs, true
	}
	return f
}

func parseCents(t *testing.T, name, value string) *big.Int {
	t.Helper()

	n, ok := new(big.Int).SetString(value, 10)
	require.True(t, ok, "%s %q is not an integer", name, value)
	return n
}

func describeActivity(a *types.MarketActivity) string {
	ts := func(p *int64) string {
		if p == nil {
			return "null"
		}
		return strconv.FormatInt(*p, 10)
	}
	return fmt.Sprintf("bridge=%s volume_cents=%s direct_cents=%s mint_burn_cents=%s "+
		"unique_traders=%d fill_count=%d direct_fill_count=%d shares_traded=%d "+
		"first_event_ts=%s last_event_ts=%s coverage_from_block=%d coverage_complete=%t",
		a.Bridge, a.VolumeCents, a.DirectCents, a.MintBurnCents,
		a.UniqueTraders, a.FillCount, a.DirectFillCount, a.SharesTraded,
		ts(a.FirstEventTs), ts(a.LastEventTs), a.CoverageFromBlock, a.CoverageComplete)
}
