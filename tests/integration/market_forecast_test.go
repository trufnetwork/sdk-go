// Live-network checks that the SDK reads real order books the right way round.
//
// Everything else in the forecast suite feeds the algorithm hand-written
// numbers, which proves the maths but cannot prove the SDK is reading the CHAIN
// correctly. If the node ever flipped the sign convention on bids, every one of
// those tests would still pass while the forecast silently inverted. That is the
// gap this file covers.
//
// Gated on an env var rather than the kwiltest build tag, because it wants a
// network carrying live markets with real orders rather than a fresh local node:
//
//	TN_LIVE_ENDPOINT=https://gateway.mainnet.truf.network \
//	    go test ./tests/integration/ -run TestMarketForecastLive -v
//
// All calls are view actions, so the signing key needs no funds and controls
// nothing.
//
// These assert SHAPES, not numbers. Live books move minute to minute, so pinning
// an expected value would fail by tomorrow. Exact numbers belong in the unit
// tests under core/forecast. A wrong FORMULA will pass here quite happily -- this
// only proves the plumbing between the SDK and the chain is connected the right
// way round.

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/contractsapi"
	"github.com/trufnetwork/sdk-go/core/forecast"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

const (
	// maxMarketsScanned bounds discovery, which costs one call per open market.
	// Mainnet carried ~110 open markets when this was written; the cap is
	// headroom, not a target.
	maxMarketsScanned = 500

	// maxGroupsProbed bounds how many candidates are checked for quotes before
	// giving up. A market can tile the line perfectly and still have nobody
	// quoting it, which is not a code defect, so candidates are ranked by how
	// much of the line is actually quoted rather than by bucket count alone --
	// most open markets carry no orders at all.
	maxGroupsProbed = 25

	// minQuotedBuckets is the point below which the forecast rests on residuals
	// more than on orders, which is not what this test is trying to exercise.
	minQuotedBuckets = 2

	// liveReadOnlyPK signs view actions only. It holds nothing and controls
	// nothing; declared here so this file stays self-contained.
	liveReadOnlyPK = "0000000000000000000000000000000000000000000000000000000000000abc"
)

// liveBucket is one candidate bucket found during discovery.
type liveBucket struct {
	queryID int
	kind    string
}

// tilesTheLine reports one bucket open below, one open above, at least one in
// between.
func tilesTheLine(members []liveBucket) bool {
	below, above := 0, 0
	for _, m := range members {
		switch m.kind {
		case "below":
			below++
		case "above":
			above++
		}
	}
	return len(members) >= 3 && below == 1 && above == 1
}

// discoverGroups returns every open market grouped into candidate bucket sets.
//
// A market's buckets share a data stream and a settlement time, which is enough
// to reassemble them from chain data alone -- no local config needed.
func discoverGroups(ctx context.Context, t *testing.T, ob types.IOrderBook) [][]liveBucket {
	t.Helper()

	groups := map[string][]liveBucket{}
	var order []string
	offset, scanned := 0, 0

	// FALSE means active only, per 032-order-book-actions.sql:358. Settled
	// markets cannot move and would only waste probes.
	active := false

	for scanned < maxMarketsScanned {
		limit := 100
		page, err := ob.ListMarkets(ctx, types.ListMarketsInput{
			SettledFilter: &active, Limit: &limit, Offset: &offset,
		})
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}

		for _, summary := range page {
			scanned++
			info, err := ob.GetMarketInfo(ctx, types.GetMarketInfoInput{QueryID: summary.ID})
			if err != nil || len(info.QueryComponents) == 0 {
				continue
			}
			market, err := contractsapi.DecodeMarketData(info.QueryComponents)
			if err != nil {
				continue
			}
			// Legacy or non-bucket markets are not what this test is about.
			if _, _, err := contractsapi.BucketBoundsFromMarketData(market); err != nil {
				continue
			}
			key := fmt.Sprintf("%s@%d", market.StreamID, summary.SettleTime)
			if _, seen := groups[key]; !seen {
				order = append(order, key)
			}
			groups[key] = append(groups[key], liveBucket{queryID: summary.ID, kind: market.Type})
		}

		if len(page) < limit {
			break
		}
		offset += limit
	}

	var tiling [][]liveBucket
	for _, key := range order {
		if tilesTheLine(groups[key]) {
			tiling = append(tiling, groups[key])
		}
	}
	return tiling
}

// medianBucket returns the bucket where the implied CDF crosses 50%.
//
// NOT the most probable bucket: with probabilities [0.40, 0.35, 0.25] the leader
// is the first, but the cumulative only reaches half way through the second, so
// that is where the median belongs.
func medianBucket(f *forecast.MarketForecast) forecast.BucketEstimate {
	running := 0.0
	for _, b := range f.Buckets {
		running += b.Probability
		if running >= 0.5 {
			return b
		}
	}
	return f.Buckets[len(f.Buckets)-1]
}

func TestMarketForecastLive(t *testing.T) {
	endpoint := os.Getenv("TN_LIVE_ENDPOINT")
	if endpoint == "" {
		t.Skip("live network not configured; set TN_LIVE_ENDPOINT to run")
	}

	ctx := context.Background()

	key, err := kwilcrypto.Secp256k1PrivateKeyFromHex(liveReadOnlyPK)
	require.NoError(t, err, "failed to parse the read-only key")

	client, err := tnclient.NewClient(ctx, endpoint, tnclient.WithSigner(auth.GetUserSigner(key)))
	require.NoError(t, err, "failed to create client")

	ob, err := client.LoadOrderBook()
	require.NoError(t, err, "failed to load order book")

	candidates := discoverGroups(ctx, t, ob)
	if len(candidates) == 0 {
		t.Skip("no open market on this network currently tiles the line")
	}

	// One cheap call per bucket to find where the orders are. Ranking on this
	// rather than on bucket count matters: most open markets tile the line
	// perfectly and carry no orders whatsoever.
	type ranked struct {
		quoted   int
		queryIDs []int
	}
	var usable []ranked
	probed := len(candidates)
	if probed > maxGroupsProbed {
		probed = maxGroupsProbed
	}
	for _, members := range candidates[:probed] {
		queryIDs := make([]int, len(members))
		quoted := 0
		for i, m := range members {
			queryIDs[i] = m.queryID
			best, err := ob.GetBestPrices(ctx, types.GetBestPricesInput{QueryID: m.queryID, Outcome: true})
			if err == nil && best != nil && (best.BestBid != nil || best.BestAsk != nil) {
				quoted++
			}
		}
		if quoted >= minQuotedBuckets {
			usable = append(usable, ranked{quoted: quoted, queryIDs: queryIDs})
		}
	}
	if len(usable) == 0 {
		t.Skipf("no market among %d tiling candidates had %d+ quoted buckets",
			probed, minQuotedBuckets)
	}

	// Most quoted first. Settled markets are already excluded, and on mainnet the
	// newest markets are routinely the ones with no orders yet, so recency is a
	// poor signal to rank on.
	for i := 0; i < len(usable); i++ {
		for j := i + 1; j < len(usable); j++ {
			if usable[j].quoted > usable[i].quoted {
				usable[i], usable[j] = usable[j], usable[i]
			}
		}
	}

	var queryIDs []int
	var f *forecast.MarketForecast
	for _, candidate := range usable {
		result, err := ob.GetMarketForecast(ctx, types.GetMarketForecastInput{QueryIDs: candidate.queryIDs})
		require.NoError(t, err)
		if result != nil {
			queryIDs, f = candidate.queryIDs, result
			break
		}
	}
	if f == nil {
		t.Skip("quoted markets found, but none yielded a usable forecast")
	}
	t.Logf("forecasting market %v: value=%.4f [%s]", queryIDs, f.Value, f.ValueBasis)

	t.Run("a real market produces a forecast", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(queryIDs), 3)
		assert.False(t, f.Value != f.Value, "value is NaN")
		assert.Contains(t, []string{forecast.MethodRank, forecast.MethodDiscrete}, f.Method)
		assert.Contains(t,
			[]string{forecast.BasisInterior, forecast.BasisTail, forecast.BasisUnresolved},
			f.ValueBasis)
	})

	t.Run("the probabilities form a distribution", func(t *testing.T) {
		total := 0.0
		for _, b := range f.Buckets {
			assert.GreaterOrEqual(t, b.Probability, 0.0)
			assert.LessOrEqual(t, b.Probability, 1.0)
			total += b.Probability
		}
		assert.InDelta(t, 1.0, total, 1e-9)
	})

	t.Run("the value falls in the bucket where the CDF crosses half", func(t *testing.T) {
		// The point estimate is the median, so it must land in the median
		// bucket. This is the assertion that would catch a bounds-derivation or
		// ordering mistake: get the buckets in the wrong order and the value
		// lands in the wrong one.
		if f.ValueBasis == forecast.BasisUnresolved {
			t.Skip("median unresolvable from this market's strikes")
		}
		bucket := medianBucket(f)
		if bucket.Lower != nil {
			assert.GreaterOrEqual(t, f.Value, *bucket.Lower)
		}
		if bucket.Upper != nil {
			assert.LessOrEqual(t, f.Value, *bucket.Upper)
		}
	})

	t.Run("the band brackets the value", func(t *testing.T) {
		if f.P10 == nil || f.P90 == nil {
			t.Skip("band unresolvable from this market's strikes")
		}
		assert.LessOrEqual(t, *f.P10, f.Value)
		assert.LessOrEqual(t, f.Value, *f.P90)
		assert.Greater(t, f.MarginOfError, 0.0)
	})

	t.Run("the buckets are ordered and tile the line", func(t *testing.T) {
		// Assembly must hand the algorithm an ascending, gap-free ladder.
		assert.Nil(t, f.Buckets[0].Lower, "bottom bucket should be open below")
		assert.Nil(t, f.Buckets[len(f.Buckets)-1].Upper, "top bucket should be open above")
		for i := 0; i+1 < len(f.Buckets); i++ {
			upper, lower := f.Buckets[i].Upper, f.Buckets[i+1].Lower
			require.NotNil(t, upper, "gap between buckets")
			require.NotNil(t, lower, "gap between buckets")
			assert.Equal(t, *upper, *lower, "gap or overlap between buckets")
		}
	})

	t.Run("the bid sign convention still holds", func(t *testing.T) {
		// The canary.
		//
		// GetOrderBook marks bids with a NEGATIVE price; GetBestPrices reports
		// the same bid as POSITIVE. Two independent node paths for one fact, so
		// if either convention ever changes they stop agreeing here -- loudly --
		// instead of silently inverting every one-sided bucket in the forecast.
		checked := 0
		for _, queryID := range queryIDs {
			for _, outcome := range []bool{true, false} {
				entries, err := ob.GetOrderBook(ctx, types.GetOrderBookInput{
					QueryID: queryID, Outcome: outcome,
				})
				require.NoError(t, err)

				bestBid, bestAsk := 0, 0
				for _, e := range entries {
					if e.Price == 0 {
						continue // a holding, not a resting order
					}
					price := e.Price
					if price < 0 {
						price = -price
					}
					assert.Greater(t, price, 0)
					assert.Less(t, price, 100, "outside the 1-99 cent convention")
					assert.Greater(t, e.Amount, int64(0))

					if e.Price < 0 && -e.Price > bestBid {
						bestBid = -e.Price
					}
					if e.Price > 0 && (bestAsk == 0 || e.Price < bestAsk) {
						bestAsk = e.Price
					}
				}

				best, err := ob.GetBestPrices(ctx, types.GetBestPricesInput{
					QueryID: queryID, Outcome: outcome,
				})
				require.NoError(t, err)
				require.NotNil(t, best)

				if bestBid > 0 && best.BestBid != nil {
					assert.Equal(t, *best.BestBid, bestBid, "bid sign convention disagrees")
					checked++
				}
				if bestAsk > 0 && best.BestAsk != nil {
					assert.Equal(t, *best.BestAsk, bestAsk, "ask convention disagrees")
					checked++
				}
			}
		}
		if checked == 0 {
			t.Skip("no resting orders to cross-check the sign convention against")
		}
	})
}
