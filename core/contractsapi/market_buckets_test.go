// Tests for the SDK-side glue around the forecast algorithm.
//
// The algorithm itself is covered by core/forecast. What is tested here is only
// what the SDK adds: deriving bucket bounds from decoded market data, splitting
// raw order book rows into ladders, and assembling a market's books into a
// forecast.
//
// Ported from sdk-py's tests/test_market_buckets.py. The client method itself
// needs a node, so it is split: everything below the network call is exercised
// here, and GetMarketForecast's plumbing is covered by the live integration
// suite.

package contractsapi

import (
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/forecast"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ═══════════════════════════════════════════════════════════════
// BUCKET BOUNDS FROM DECODED MARKET DATA
// ═══════════════════════════════════════════════════════════════

func TestBucketBounds_BelowMarketIsTheOpenBottomBucket(t *testing.T) {
	lower, upper, err := BucketBoundsFromMarketData(
		&MarketData{Type: "below", Thresholds: []string{"4.04"}})
	require.NoError(t, err)
	assert.Nil(t, lower)
	require.NotNil(t, upper)
	assert.InDelta(t, 4.04, *upper, 1e-12)
}

func TestBucketBounds_AboveMarketIsTheOpenTopBucket(t *testing.T) {
	lower, upper, err := BucketBoundsFromMarketData(
		&MarketData{Type: "above", Thresholds: []string{"4.92"}})
	require.NoError(t, err)
	require.NotNil(t, lower)
	assert.InDelta(t, 4.92, *lower, 1e-12)
	assert.Nil(t, upper)
}

func TestBucketBounds_BetweenMarketIsAnInteriorBucket(t *testing.T) {
	lower, upper, err := BucketBoundsFromMarketData(
		&MarketData{Type: "between", Thresholds: []string{"4.33", "4.62"}})
	require.NoError(t, err)
	require.NotNil(t, lower)
	require.NotNil(t, upper)
	assert.InDelta(t, 4.33, *lower, 1e-12)
	assert.InDelta(t, 4.62, *upper, 1e-12)
}

func TestBucketBounds_EqualsMarketIsTargetPlusMinusTolerance(t *testing.T) {
	// Its thresholds are (target, tolerance), NOT (lower, upper). Reading them
	// positionally the way "between" is read would yield the bucket
	// (5.25, 0.10): inverted, and silently wrong rather than loud.
	lower, upper, err := BucketBoundsFromMarketData(
		&MarketData{Type: "equals", Thresholds: []string{"5.25", "0.10"}})
	require.NoError(t, err)
	require.NotNil(t, lower)
	require.NotNil(t, upper)
	assert.InDelta(t, 5.15, *lower, 1e-12)
	assert.InDelta(t, 5.35, *upper, 1e-12)
	assert.Less(t, *lower, *upper)
}

func TestBucketBounds_UnknownMarketTypeIsRejected(t *testing.T) {
	_, _, err := BucketBoundsFromMarketData(&MarketData{Type: "unknown"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot derive bucket bounds")
}

func TestBucketBounds_MissingThresholdsAreRejected(t *testing.T) {
	_, _, err := BucketBoundsFromMarketData(
		&MarketData{Type: "between", Thresholds: []string{"4.33"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "threshold")
}

func TestBucketBounds_RoundTripsThroughRealQueryComponents(t *testing.T) {
	// The bounds are derived from bytes on chain, so decode the real encoding
	// rather than trusting a hand-built struct.
	const dataProvider = "0x4710a8d8f0d845da110086812a32de6d90d7ff5c"
	const streamID = "stmsft00000000000000000000000000"

	args, err := EncodeActionArgs([]any{dataProvider, streamID, int64(1700000000), "4.04", nil})
	require.NoError(t, err)
	encoded, err := EncodeQueryComponents(dataProvider, streamID, "price_below_threshold", args)
	require.NoError(t, err)

	market, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.Equal(t, "below", market.Type)

	lower, upper, err := BucketBoundsFromMarketData(market)
	require.NoError(t, err)
	assert.Nil(t, lower)
	require.NotNil(t, upper)
	assert.InDelta(t, 4.04, *upper, 1e-12)
}

// ═══════════════════════════════════════════════════════════════
// LADDER CONVERSION (THE SIGN CONVENTION)
// ═══════════════════════════════════════════════════════════════

func TestLaddersFromEntries_NegativePriceIsABidPositiveIsAnAsk(t *testing.T) {
	// Get this backwards and every one-sided bucket in the forecast inverts.
	bids, asks := laddersFromEntries([]types.OrderBookEntry{
		{Price: -44, Amount: 1000},
		{Price: 56, Amount: 500},
		{Price: -43, Amount: 200},
	})

	require.Len(t, bids, 2)
	require.Len(t, asks, 1)
	assert.Equal(t, forecast.BookLevel{Price: 44, Size: 1000}, bids[0])
	assert.Equal(t, forecast.BookLevel{Price: 43, Size: 200}, bids[1])
	assert.Equal(t, forecast.BookLevel{Price: 56, Size: 500}, asks[0])
}

func TestLaddersFromEntries_HoldingsAreNotPartOfTheBook(t *testing.T) {
	// Price 0 means shares owned with no resting order.
	bids, asks := laddersFromEntries([]types.OrderBookEntry{
		{Price: 0, Amount: 9999},
	})
	assert.Empty(t, bids)
	assert.Empty(t, asks)
}

// ═══════════════════════════════════════════════════════════════
// ASSEMBLY
// ═══════════════════════════════════════════════════════════════

// Big enough that even a 1c level clears DepthMinSideNotionalUSD, so the dust
// floor never silently empties a side these tests meant to be quoted.
const testSize = 1000

// msftMarket is one bucket of the live mainnet MSFT book, as a separate market.
type msftMarket struct {
	queryID int
	market  *MarketData
	yesBid  *float64
	yesAsk  *float64
}

var msftMarkets = []msftMarket{
	{101, &MarketData{Type: "below", Thresholds: []string{"4.04"}}, forecast.Ptr(1.0), nil},
	{102, &MarketData{Type: "between", Thresholds: []string{"4.04", "4.33"}}, forecast.Ptr(16.0), forecast.Ptr(28.0)},
	{103, &MarketData{Type: "between", Thresholds: []string{"4.33", "4.62"}}, forecast.Ptr(44.0), forecast.Ptr(56.0)},
	{104, &MarketData{Type: "between", Thresholds: []string{"4.62", "4.92"}}, forecast.Ptr(9.0), forecast.Ptr(21.0)},
	{105, &MarketData{Type: "above", Thresholds: []string{"4.92"}}, forecast.Ptr(1.0), nil},
}

// booksFor builds the BucketDepth slice GetMarketForecast would have fetched for
// the given query_ids, taking the same bounds path off each market's data.
func booksFor(t *testing.T, queryIDs []int, noBooks map[int][]types.OrderBookEntry) []forecast.BucketDepth {
	t.Helper()
	var books []forecast.BucketDepth
	for _, queryID := range queryIDs {
		var found *msftMarket
		for i := range msftMarkets {
			if msftMarkets[i].queryID == queryID {
				found = &msftMarkets[i]
			}
		}
		require.NotNil(t, found, "unknown query_id %d", queryID)

		lower, upper, err := BucketBoundsFromMarketData(found.market)
		require.NoError(t, err)

		var yesEntries []types.OrderBookEntry
		if found.yesBid != nil {
			yesEntries = append(yesEntries, types.OrderBookEntry{Price: int(-*found.yesBid), Amount: testSize})
		}
		if found.yesAsk != nil {
			yesEntries = append(yesEntries, types.OrderBookEntry{Price: int(*found.yesAsk), Amount: testSize})
		}
		yesBids, yesAsks := laddersFromEntries(yesEntries)
		noBids, noAsks := laddersFromEntries(noBooks[queryID])

		id := queryID
		books = append(books, forecast.BucketDepth{
			Lower: lower, Upper: upper,
			YesBids: yesBids, YesAsks: yesAsks,
			NoBids: noBids, NoAsks: noAsks,
			QueryID: &id,
		})
	}
	return books
}

func allQueryIDs() []int {
	ids := make([]int, len(msftMarkets))
	for i, m := range msftMarkets {
		ids[i] = m.queryID
	}
	return ids
}

func hasWarning(f *forecast.MarketForecast, substring string) bool {
	for _, w := range f.Warnings {
		if strings.Contains(w, substring) {
			return true
		}
	}
	return false
}

func TestAssembly_MatchesADirectCall(t *testing.T) {
	// Assembly must not change the answer: same books in, same number out.
	books := booksFor(t, allQueryIDs(), nil)
	assembled := forecastFromBooks(books)
	require.NotNil(t, assembled)

	direct := forecast.FromDepth(booksFor(t, allQueryIDs(), nil))
	require.NotNil(t, direct)

	assert.InDelta(t, direct.Value, assembled.Value, 1e-12)
	assert.InDelta(t, *direct.P10, *assembled.P10, 1e-12)
	assert.InDelta(t, *direct.P90, *assembled.P90, 1e-12)
	for i := range direct.Buckets {
		assert.InDelta(t, direct.Buckets[i].Probability, assembled.Buckets[i].Probability, 1e-12)
	}
}

func TestAssembly_LandsInsideTheLeadingBucket(t *testing.T) {
	// The live MSFT book: the tight 44-56 bucket leads, so the value belongs
	// inside it.
	f := forecastFromBooks(booksFor(t, allQueryIDs(), nil))
	require.NotNil(t, f)

	top := 0.0
	for _, b := range f.Buckets {
		top = math.Max(top, b.Probability)
	}
	assert.Equal(t, top, f.Buckets[2].Probability)
	assert.GreaterOrEqual(t, f.Value, 4.33)
	assert.LessOrEqual(t, f.Value, 4.62)
}

func TestAssembly_NoSideLiquidityIsConsolidatedIntoTheEstimate(t *testing.T) {
	// A resting BUY NO at p is hittable by a BUY YES at 100-p, so NO depth is
	// executable YES depth. Adding a NO bid must therefore supply the missing
	// YES ask and stop the bucket reading as one-sided.
	bare := forecastFromBooks(booksFor(t, allQueryIDs(), nil))
	require.NotNil(t, bare)

	// A NO bid at 84 mirrors to a YES ask at 16 on the open bottom bucket.
	withNo := forecastFromBooks(booksFor(t, allQueryIDs(), map[int][]types.OrderBookEntry{
		101: {{Price: -84, Amount: testSize}},
	}))
	require.NotNil(t, withNo)

	assert.True(t, bare.Buckets[0].OneSided)
	assert.False(t, withNo.Buckets[0].OneSided)
	assert.Greater(t, withNo.Buckets[0].Confidence, bare.Buckets[0].Confidence)
}

func TestAssembly_QueryIDsMayBeGivenInAnyOrder(t *testing.T) {
	// Buckets are sorted by bound here, so callers need not track which query_id
	// is the bottom of the line.
	ordered := forecastFromBooks(booksFor(t, []int{101, 102, 103, 104, 105}, nil))
	shuffled := forecastFromBooks(booksFor(t, []int{103, 105, 101, 104, 102}, nil))
	require.NotNil(t, ordered)
	require.NotNil(t, shuffled)

	assert.InDelta(t, ordered.Value, shuffled.Value, 1e-12)
	got := make([]int, len(shuffled.Buckets))
	for i, b := range shuffled.Buckets {
		got[i] = *b.QueryID
	}
	assert.Equal(t, []int{101, 102, 103, 104, 105}, got)
	assert.True(t, sort.IntsAreSorted(got))
}

func TestAssembly_GapInTheBucketLayoutIsReportedNotHidden(t *testing.T) {
	// A market missing an interior bucket still gets an estimate, but the caller
	// must be able to see that the line was not fully tiled.
	f := forecastFromBooks(booksFor(t, []int{101, 102, 104, 105}, nil))
	require.NotNil(t, f)
	assert.True(t, hasWarning(f, "gap"))
}

func TestAssembly_LayoutWarningWhenTheLineIsNotOpenEnded(t *testing.T) {
	// Interior buckets only: nothing covers the tails, so the mass beyond them
	// is unrepresented and the caller should know.
	f := forecastFromBooks(booksFor(t, []int{102, 103, 104}, nil))
	require.NotNil(t, f)
	assert.True(t, hasWarning(f, "not open below"))
	assert.True(t, hasWarning(f, "not open above"))
}

func TestAssembly_UnquotedMarketYieldsNoForecast(t *testing.T) {
	dead := booksFor(t, allQueryIDs(), nil)
	for i := range dead {
		dead[i].YesBids, dead[i].YesAsks = nil, nil
		dead[i].NoBids, dead[i].NoAsks = nil, nil
	}
	assert.Nil(t, forecastFromBooks(dead))
}

// ═══════════════════════════════════════════════════════════════
// INPUT VALIDATION
// ═══════════════════════════════════════════════════════════════

func TestGetMarketForecastInput_SingleBucketIsRejected(t *testing.T) {
	input := types.GetMarketForecastInput{QueryIDs: []int{103}}
	err := input.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2")
}

func TestGetMarketForecastInput_NonPositiveQueryIDIsRejected(t *testing.T) {
	input := types.GetMarketForecastInput{QueryIDs: []int{101, 0}}
	require.Error(t, input.Validate())
}

func TestGetMarketForecastInput_ValidMarketIsAccepted(t *testing.T) {
	input := types.GetMarketForecastInput{QueryIDs: allQueryIDs()}
	assert.NoError(t, input.Validate())
}
