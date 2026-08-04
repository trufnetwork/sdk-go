// Tests for the market-implied point forecast (best-quote path).
//
// Ported case-for-case from sdk-py's tests/test_forecast.py, which is itself the
// upstream prediction-bots suite. Keeping the cases aligned is what proves the
// SDKs agree: this package is a hand translation, so behaviour is the only thing
// that can be diffed.

package forecast

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bound is a bucket's [lower, upper) pair, nil meaning open-ended.
type bound struct {
	lower *float64
	upper *float64
}

// msftBounds is the MSFT EPS 2026 Q3 bucket layout, as configured on mainnet.
var msftBounds = []bound{
	{nil, Ptr(4.04)},
	{Ptr(4.04), Ptr(4.33)},
	{Ptr(4.33), Ptr(4.62)},
	{Ptr(4.62), Ptr(4.92)},
	{Ptr(4.92), nil},
}

// build returns books quoting each bucket at the given cents, with a symmetric
// spread. A nil entry leaves the bucket unquoted.
func build(cents []*float64, bounds []bound, spread float64) []BucketBook {
	books := make([]BucketBook, len(bounds))
	for i, b := range bounds {
		books[i] = BucketBook{Lower: b.lower, Upper: b.upper}
		if cents[i] != nil {
			books[i].BestBid = Ptr(math.Max(1, *cents[i]-spread/2))
			books[i].BestAsk = Ptr(math.Min(99, *cents[i]+spread/2))
		}
	}
	return books
}

// cents is shorthand for a quote list; nil entries stay unquoted.
func cents(values ...interface{}) []*float64 {
	out := make([]*float64, len(values))
	for i, v := range values {
		switch n := v.(type) {
		case int:
			out[i] = Ptr(float64(n))
		case float64:
			out[i] = Ptr(n)
		case nil:
			out[i] = nil
		}
	}
	return out
}

// msft builds the standard MSFT-shaped book at a 2c spread.
func msft(values ...interface{}) []BucketBook {
	return build(cents(values...), msftBounds, 2)
}

// mustForecast asserts a forecast was produced and returns it.
func mustForecast(t *testing.T, books []BucketBook) *MarketForecast {
	t.Helper()
	f := FromBuckets(books)
	require.NotNil(t, f)
	return f
}

func hasWarning(f *MarketForecast, substring string) bool {
	for _, w := range f.Warnings {
		if strings.Contains(w, substring) {
			return true
		}
	}
	return false
}

// ═══════════════════════════════════════════════════════════════
// BRACKET LOGIC
// ═══════════════════════════════════════════════════════════════

func TestBucketProbability_TwoSidedQuoteIsTheMidpoint(t *testing.T) {
	p, conf, oneSided, quoted := BucketProbability(Ptr(38.0), Ptr(42.0), DefaultHalfSpread)
	require.NotNil(t, p)
	assert.InDelta(t, 0.40, *p, 1e-12)
	assert.InDelta(t, 0.96, conf, 1e-12)
	assert.False(t, oneSided)
	assert.True(t, quoted)
}

func TestBucketProbability_TighterSpreadMeansMoreConfidence(t *testing.T) {
	_, tight, _, _ := BucketProbability(Ptr(39.0), Ptr(41.0), DefaultHalfSpread)
	_, wide, _, _ := BucketProbability(Ptr(20.0), Ptr(60.0), DefaultHalfSpread)
	assert.Greater(t, tight, wide)
}

func TestBucketProbability_OneSidedBookStepsAcrossByTheHalfSpread(t *testing.T) {
	// A lone bid is real information, but it is a BID: the fair value sits a
	// half-spread above it, not at the midpoint of [bid, 1].
	p, conf, oneSided, quoted := BucketProbability(Ptr(30.0), nil, 0.06)
	require.NotNil(t, p)
	assert.True(t, oneSided)
	assert.True(t, quoted)
	assert.InDelta(t, 0.36, *p, 1e-12)
	assert.Less(t, conf, 0.75, "we inferred one side rather than observing it")
}

func TestBucketProbability_LonePennyBidIsNotACoinFlip(t *testing.T) {
	// The bug live books exposed: the [0.01, 1.0] midpoint made dead tails 50%.
	p, _, _, _ := BucketProbability(Ptr(1.0), nil, 0.06)
	require.NotNil(t, p)
	assert.InDelta(t, 0.07, *p, 1e-12)
	assert.Less(t, *p, 0.15)
}

func TestBucketProbability_EmptyBookReturnsNilNotANumber(t *testing.T) {
	p, conf, oneSided, quoted := BucketProbability(nil, nil, DefaultHalfSpread)
	assert.Nil(t, p)
	assert.Zero(t, conf)
	assert.False(t, quoted)
	assert.False(t, oneSided)
}

func TestBucketProbability_CrossedBookIsDistrusted(t *testing.T) {
	_, conf, _, _ := BucketProbability(Ptr(60.0), Ptr(40.0), DefaultHalfSpread)
	assert.LessOrEqual(t, conf, 1e-3)
}

// ═══════════════════════════════════════════════════════════════
// THE CORE ESTIMATE
// ═══════════════════════════════════════════════════════════════

func TestFromBuckets_SymmetricMarketCentresOnTheMiddleBucket(t *testing.T) {
	// Mass symmetric about bucket 3 must imply its midpoint, (4.33+4.62)/2.
	f := mustForecast(t, msft(5, 20, 50, 20, 5))
	assert.Equal(t, MethodRank, f.Method)
	assert.InDelta(t, 4.475, f.Value, 0.03)
	assert.Equal(t, BasisInterior, f.ValueBasis)
}

func TestFromBuckets_MovesWithTheMass(t *testing.T) {
	low := mustForecast(t, msft(40, 30, 20, 7, 3)).Value
	mid := mustForecast(t, msft(5, 20, 50, 20, 5)).Value
	high := mustForecast(t, msft(3, 7, 20, 30, 40)).Value
	assert.Less(t, low, mid)
	assert.Less(t, mid, high)
}

func TestFromBuckets_LandsInsideTheLeadingBucket(t *testing.T) {
	f := mustForecast(t, msft(5, 10, 60, 20, 5))
	assert.GreaterOrEqual(t, f.Value, 4.33)
	assert.LessOrEqual(t, f.Value, 4.62)
}

func TestFromBuckets_OpenTailsNeedNoAssumedValue(t *testing.T) {
	// Heavy mass in the unbounded top bucket must still give a finite answer
	// that sits above the last boundary.
	f := mustForecast(t, msft(1, 2, 7, 20, 70))
	assert.False(t, math.IsInf(f.Value, 0) || math.IsNaN(f.Value))
	assert.Greater(t, f.Value, 4.92)
}

func TestFromBuckets_ProbabilitiesAreNormalised(t *testing.T) {
	f := mustForecast(t, msft(10, 25, 55, 25, 10)) // sums to 125c
	var total float64
	for _, b := range f.Buckets {
		total += b.Probability
	}
	assert.InDelta(t, 1.0, total, 1e-12)
	assert.True(t, hasWarning(f, "sum to"))
}

// ═══════════════════════════════════════════════════════════════
// THE TWO UNCERTAINTIES
// ═══════════════════════════════════════════════════════════════

func TestFromBuckets_BandIsThePublishedUncertainty(t *testing.T) {
	// Under the rank method the band IS the uncertainty statement: the market
	// says the outcome lands in [p10, p90] with 80% probability. Margin and
	// sigma are derived from it for compatibility.
	f := mustForecast(t, msft(5, 20, 50, 20, 5))
	require.NotNil(t, f.P10)
	require.NotNil(t, f.P90)
	assert.Less(t, *f.P10, f.Value)
	assert.Less(t, f.Value, *f.P90)
	assert.InDelta(t, (*f.P90-*f.P10)/2, f.MarginOfError, 1e-12)
	assert.InDelta(t, (*f.P90-*f.P10)/2.5631, f.Sigma, 1e-12)
}

func TestFromBuckets_TightBooksPinTheValueBetterThanWideOnes(t *testing.T) {
	tight := mustForecast(t, build(cents(5, 20, 50, 20, 5), msftBounds, 2))
	wide := mustForecast(t, build(cents(5, 20, 50, 20, 5), msftBounds, 30))
	assert.Greater(t, wide.MarginOfError, tight.MarginOfError)
}

func TestFromBuckets_OneSidedBooksWidenTheMargin(t *testing.T) {
	twoSided := mustForecast(t, msft(5, 20, 50, 20, 5))

	quotes := []float64{5, 20, 50, 20, 5}
	half := make([]BucketBook, len(msftBounds))
	for i, b := range msftBounds {
		half[i] = BucketBook{Lower: b.lower, Upper: b.upper, BestBid: Ptr(quotes[i])}
	}
	oneSided := mustForecast(t, half)

	assert.Greater(t, oneSided.MarginOfError, twoSided.MarginOfError)
	assert.True(t, hasWarning(oneSided, "one-sided"))
}

func TestFromBuckets_LowHighBracketTheValue(t *testing.T) {
	f := mustForecast(t, msft(5, 20, 50, 20, 5))
	assert.Less(t, f.Low(), f.Value)
	assert.Less(t, f.Value, f.High())
	assert.InDelta(t, 2*f.MarginOfError, f.High()-f.Low(), 1e-12)
}

// ═══════════════════════════════════════════════════════════════
// DEGENERATE INPUT
// ═══════════════════════════════════════════════════════════════

func TestFromBuckets_NoQuotesAtAllReturnsNil(t *testing.T) {
	assert.Nil(t, FromBuckets(msft(nil, nil, nil, nil, nil)))
}

func TestFromBuckets_TooFewBucketsReturnsNil(t *testing.T) {
	assert.Nil(t, FromBuckets([]BucketBook{
		{Lower: nil, Upper: Ptr(4.0), BestBid: Ptr(40.0), BestAsk: Ptr(44.0)},
	}))
}

func TestFromBuckets_PartialBookStillProducesAnEstimate(t *testing.T) {
	f := mustForecast(t, msft(nil, 20, 50, 20, nil))
	assert.False(t, math.IsNaN(f.Value))
	assert.True(t, hasWarning(f, "unquoted"))
}

func TestFromBuckets_UnquotedBucketsTakeTheResidualNotAFlatHalf(t *testing.T) {
	// Quoted buckets sum to ~0.9, so the two unquoted ones share ~0.1 between
	// them, rather than being handed 0.5 each and swamping the distribution.
	f := mustForecast(t, msft(nil, 20, 50, 20, nil))
	assert.InDelta(t, f.Buckets[4].Probability, f.Buckets[0].Probability, 1e-12)
	assert.Less(t, f.Buckets[0].Probability, 0.10, "unquoted tail took too much")
	assert.Greater(t, f.Buckets[2].Probability, 0.4, "the quoted leading bucket must still dominate")
}

func TestFromBuckets_DeadTailsDoNotDominateALiveMiddle(t *testing.T) {
	// Reproduces the live MSFT book: penny-bid one-sided tails around a tight
	// two-sided middle. The tails must not come out level with the leader.
	f := mustForecast(t, []BucketBook{
		{Lower: nil, Upper: Ptr(4.04), BestBid: Ptr(1.0)},
		{Lower: Ptr(4.04), Upper: Ptr(4.33), BestBid: Ptr(16.0), BestAsk: Ptr(28.0)},
		{Lower: Ptr(4.33), Upper: Ptr(4.62), BestBid: Ptr(44.0), BestAsk: Ptr(56.0)},
		{Lower: Ptr(4.62), Upper: Ptr(4.92), BestBid: Ptr(9.0), BestAsk: Ptr(21.0)},
		{Lower: Ptr(4.92), Upper: nil, BestBid: Ptr(1.0)},
	})

	top := 0.0
	for _, b := range f.Buckets {
		top = math.Max(top, b.Probability)
	}
	assert.Equal(t, top, f.Buckets[2].Probability, "the tight 44-56 bucket must lead")
	assert.Less(t, f.Buckets[0].Probability, 0.15)
	assert.Less(t, f.Buckets[4].Probability, 0.15)
	assert.GreaterOrEqual(t, f.Value, 4.2)
	assert.LessOrEqual(t, f.Value, 4.7)
}

func TestFromBuckets_AllMassInOneBucketDoesNotExplode(t *testing.T) {
	f := mustForecast(t, msft(1, 1, 95, 1, 1))
	assert.False(t, math.IsNaN(f.Value) || math.IsInf(f.Value, 0))
	assert.False(t, math.IsNaN(f.MarginOfError) || math.IsInf(f.MarginOfError, 0))
	assert.GreaterOrEqual(t, f.Value, 4.0)
	assert.LessOrEqual(t, f.Value, 5.0)
}

func TestFromBuckets_FlatBookFallsBackRatherThanDividingByZero(t *testing.T) {
	// Identical quotes everywhere say nothing about spread.
	f := mustForecast(t, msft(20, 20, 20, 20, 20))
	assert.False(t, math.IsNaN(f.Value) || math.IsInf(f.Value, 0))
}

func TestFromBuckets_AsMapIsSerialisable(t *testing.T) {
	m := mustForecast(t, msft(5, 20, 50, 20, 5)).AsMap()
	for _, key := range []string{"value", "margin_of_error", "low", "high", "sigma", "method"} {
		assert.Contains(t, m, key)
	}
	assert.Len(t, m["probabilities"], 5)

	encoded, err := json.Marshal(m)
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)
}

func TestFromBuckets_MonotonicCDFIsEnforced(t *testing.T) {
	// Noisy quotes can imply a tiny negative probability step; the CDF walk must
	// not trip over it.
	f := mustForecast(t, msft(30, 1, 40, 1, 28))
	assert.False(t, math.IsNaN(f.Value) || math.IsInf(f.Value, 0))
}
