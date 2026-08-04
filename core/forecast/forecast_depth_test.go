// Tests for the depth-weighted forecast path.
//
// The depth path consolidates the YES and NO ladders inside the package (the
// inversion is executable via exact-inverse mint/burn matching, verified on
// testnet 2026-08-03) and weights each bucket by committed notional rather than
// reading best quotes alone.
//
// Ported case-for-case from sdk-py's tests/test_forecast_depth.py.

package forecast

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// depthBounds is the MSFT-shaped layout, as on mainnet.
var depthBounds = msftBounds

// ladder returns symmetric YES ladders plus the exact NO mirror our MM would
// post.
func ladder(mid float64, size float64) (yesBids, yesAsks, noBids, noAsks []BookLevel) {
	const levels, spread, step = 3, 2.0, 1.0
	for k := 0; k < levels; k++ {
		yesBids = append(yesBids, BookLevel{Price: round4(mid - spread/2 - float64(k)*step), Size: size})
		yesAsks = append(yesAsks, BookLevel{Price: round4(mid + spread/2 + float64(k)*step), Size: size})
	}
	for _, l := range yesAsks {
		noBids = append(noBids, BookLevel{Price: round4(100 - l.Price), Size: l.Size})
	}
	for _, l := range yesBids {
		noAsks = append(noAsks, BookLevel{Price: round4(100 - l.Price), Size: l.Size})
	}
	return yesBids, yesAsks, noBids, noAsks
}

// buildDepth mirrors the quote-path build helper, with full ladders.
func buildDepth(mids []*float64, sizes []float64, bounds []bound) []BucketDepth {
	out := make([]BucketDepth, len(bounds))
	for i, b := range bounds {
		out[i] = BucketDepth{Lower: b.lower, Upper: b.upper}
		if mids[i] == nil {
			continue
		}
		size := 100.0
		if sizes != nil {
			size = sizes[i]
		}
		yb, ya, nb, na := ladder(*mids[i], size)
		out[i].YesBids, out[i].YesAsks, out[i].NoBids, out[i].NoAsks = yb, ya, nb, na
	}
	return out
}

var threeBounds = []bound{
	{nil, Ptr(1.0)},
	{Ptr(1.0), Ptr(2.0)},
	{Ptr(2.0), nil},
}

// ═══════════════════════════════════════════════════════════════
// CONSOLIDATION
// ═══════════════════════════════════════════════════════════════

func TestConsolidation_NoBidBecomesExecutableAsk(t *testing.T) {
	asks := BucketDepth{
		Lower: nil, Upper: Ptr(4.04),
		NoBids: []BookLevel{{Price: 83, Size: 50}},
	}.ConsolidatedAsks()
	require.Len(t, asks, 1)
	assert.InDelta(t, 17.0, asks[0].Price, 1e-12)
	assert.Equal(t, 50.0, asks[0].Size)
}

func TestConsolidation_NoAskBecomesExecutableBid(t *testing.T) {
	bids := BucketDepth{
		Lower: nil, Upper: Ptr(4.04),
		NoAsks: []BookLevel{{Price: 95, Size: 40}},
	}.ConsolidatedBids()
	require.Len(t, bids, 1)
	assert.InDelta(t, 5.0, bids[0].Price, 1e-12)
}

func TestConsolidation_SidesAreSortedBestFirst(t *testing.T) {
	d := BucketDepth{
		Lower: nil, Upper: Ptr(4.04),
		YesBids: []BookLevel{{Price: 30, Size: 10}},
		YesAsks: []BookLevel{{Price: 40, Size: 10}},
		NoBids:  []BookLevel{{Price: 65, Size: 10}}, // -> ask at 35, better than the YES ask
		NoAsks:  []BookLevel{{Price: 68, Size: 10}}, // -> bid at 32, better than the YES bid
	}
	assert.InDelta(t, 32.0, d.ConsolidatedBids()[0].Price, 1e-12)
	assert.InDelta(t, 35.0, d.ConsolidatedAsks()[0].Price, 1e-12)
}

// ═══════════════════════════════════════════════════════════════
// THE WALK MID
// ═══════════════════════════════════════════════════════════════

func TestWalkMid_SymmetricDepthGivesTheMid(t *testing.T) {
	yb, ya, nb, na := ladder(40.0, 100)
	p, _, notional, oneSided, quoted := EstimateFromDepth(BucketDepth{
		Lower: nil, Upper: Ptr(4.04),
		YesBids: yb, YesAsks: ya, NoBids: nb, NoAsks: na,
	}, DefaultHalfSpread)

	require.NotNil(t, p)
	assert.InDelta(t, 0.40, *p, 1e-6)
	assert.True(t, quoted)
	assert.False(t, oneSided)
	assert.Greater(t, notional, 0.0)
}

func TestWalkMid_LadderImbalanceDoesNotMoveThePrice(t *testing.T) {
	// Size imbalance is denomination and inventory, not opinion (issue #36): on
	// this venue all resting flow is the MM's own, so extra size behind the walk
	// must leave the estimate exactly where it was.
	yb, ya, _, _ := ladder(40.0, 100)
	heavy := make([]BookLevel, len(yb))
	for i, l := range yb {
		heavy[i] = BookLevel{Price: l.Price, Size: l.Size * 10}
	}

	pHeavy, _, _, _, _ := EstimateFromDepth(
		BucketDepth{Lower: nil, Upper: Ptr(4.04), YesBids: heavy, YesAsks: ya}, DefaultHalfSpread)
	pFlat, _, _, _, _ := EstimateFromDepth(
		BucketDepth{Lower: nil, Upper: Ptr(4.04), YesBids: yb, YesAsks: ya}, DefaultHalfSpread)

	require.NotNil(t, pHeavy)
	require.NotNil(t, pFlat)
	assert.InDelta(t, *pFlat, *pHeavy, 1e-12)
}

func TestWalkMid_StaysBetweenBestBidAndAsk(t *testing.T) {
	// The book only pins the truth to [best bid, best ask]; a published
	// probability outside the executable bracket is dutch-bookable. The old
	// microprice gave 18.4 on this real NVDA bucket against a best ask of 17.
	d := BucketDepth{
		Lower: nil, Upper: Ptr(1.91),
		YesBids: []BookLevel{{Price: 4, Size: 386}, {Price: 3, Size: 519}, {Price: 1, Size: 1558}},
		NoBids:  []BookLevel{{Price: 83, Size: 6}, {Price: 81, Size: 25}},
		NoAsks:  []BookLevel{{Price: 97, Size: 3}, {Price: 99, Size: 20}},
	}
	p, _, _, _, _ := EstimateFromDepth(d, DefaultHalfSpread)
	require.NotNil(t, p)
	assert.GreaterOrEqual(t, *p, d.ConsolidatedBids()[0].Price/100)
	assert.LessOrEqual(t, *p, d.ConsolidatedAsks()[0].Price/100)
}

func TestWalkMid_FragmentedSideIsPricedNotDropped(t *testing.T) {
	// $5.77 of real asks split across two sub-$5 levels must still price the
	// side (the floor is on the side total, never per level), so splitting an
	// order into small clips cannot silence a side.
	p, _, _, oneSided, quoted := EstimateFromDepth(BucketDepth{
		Lower: nil, Upper: Ptr(1.91),
		YesBids: []BookLevel{{Price: 4, Size: 386}},
		// asks at 17 ($1.02) and 19 ($4.75)
		NoBids: []BookLevel{{Price: 83, Size: 6}, {Price: 81, Size: 25}},
	}, DefaultHalfSpread)

	require.NotNil(t, p)
	assert.True(t, quoted)
	assert.False(t, oneSided)
	// sell $5 at 4c; buy $5 = $1.02 at 17 then $3.98 at 19 -> 18.59; mid 11.3
	assert.InDelta(t, 0.113, *p, 0.001)
}

func TestWalkMid_SizeBehindTheWalkIsWeightless(t *testing.T) {
	// 100k shares added at 1c behind the first $5 of bids must not move the
	// estimate by a single bit.
	yb, ya, nb, na := ladder(40.0, 100)
	base, _, _, _, _ := EstimateFromDepth(BucketDepth{
		Lower: nil, Upper: Ptr(4.04), YesBids: yb, YesAsks: ya, NoBids: nb, NoAsks: na,
	}, DefaultHalfSpread)

	spammed := append(append([]BookLevel{}, yb...), BookLevel{Price: 1, Size: 100000})
	p, _, _, _, _ := EstimateFromDepth(BucketDepth{
		Lower: nil, Upper: Ptr(4.04), YesBids: spammed, YesAsks: ya, NoBids: nb, NoAsks: na,
	}, DefaultHalfSpread)

	require.NotNil(t, base)
	require.NotNil(t, p)
	assert.Equal(t, *base, *p, "must be identical to the bit, not merely close")
}

func TestWalkMid_ThinTouchWalksToTheRealDepth(t *testing.T) {
	// A $1.68 best ask cannot speak for the side alone: the $5 walk carries into
	// the next level, blending 42 and 45 by their committed dollars.
	p, _, _, _, _ := EstimateFromDepth(BucketDepth{
		Lower: Ptr(2.06), Upper: Ptr(2.21),
		YesBids: []BookLevel{{Price: 30, Size: 7}, {Price: 29, Size: 54}, {Price: 27, Size: 58}},
		YesAsks: []BookLevel{{Price: 42, Size: 4}, {Price: 45, Size: 44}},
	}, DefaultHalfSpread)

	require.NotNil(t, p)
	// sell $5: $2.10 at 30 + $2.90 at 29 = 29.42; buy $5: $1.68 at 42 +
	// $3.32 at 45 = 43.99; mid 36.7
	assert.InDelta(t, 0.367, *p, 0.001)
}

func TestWalkMid_ConfidenceIsStandardisedWithinTheMarket(t *testing.T) {
	// Confidence compares each bucket to its OWN market's median depth, so a
	// bucket at typical depth earns the full spread quality and a bucket at a
	// fraction of typical depth is discounted proportionally. No external dollar
	// constant is involved, and one whale bucket cannot crush the rest because
	// the standard is the median.
	f := FromDepth(buildDepth(cents(62.5, 31.25, 6.25), []float64{500, 500, 20}, threeBounds))
	require.NotNil(t, f)
	confs := []float64{f.Buckets[0].Confidence, f.Buckets[1].Confidence, f.Buckets[2].Confidence}
	assert.InDelta(t, confs[1], confs[0], 1e-12)
	assert.Less(t, confs[2], confs[1]*0.2, "the thin bucket must be discounted")

	whale := FromDepth(buildDepth(cents(62.5, 31.25, 6.25), []float64{500, 50000, 500}, threeBounds))
	require.NotNil(t, whale)
	assert.InEpsilon(t, confs[0], whale.Buckets[0].Confidence, 0.05,
		"a whale in one bucket must not crush its siblings' confidence")
}

// ═══════════════════════════════════════════════════════════════
// THE ISSUE #36 BOOK, FROZEN
// ═══════════════════════════════════════════════════════════════

// nvdaFrozen is the live NVDA EPS book of 2026-08-03. Under the old
// share-weighted microprice the five raw estimates summed to 1.359 and two
// buckets priced above their own best ask. The walk mid must keep every bucket
// inside its touch and the raw sum near 1 (spread-wide, not ladder-wide).
var nvdaFrozen = []BucketDepth{
	{
		Lower: nil, Upper: Ptr(1.91),
		YesBids: []BookLevel{{Price: 4, Size: 386}, {Price: 3, Size: 519}, {Price: 1, Size: 1558}},
		NoBids:  []BookLevel{{Price: 83, Size: 6}, {Price: 81, Size: 25}},
		NoAsks:  []BookLevel{{Price: 97, Size: 3}, {Price: 99, Size: 20}},
	},
	{
		Lower: Ptr(1.91), Upper: Ptr(2.06),
		YesBids: []BookLevel{{Price: 17, Size: 96}, {Price: 16, Size: 18}, {Price: 14, Size: 122}},
		NoBids:  []BookLevel{{Price: 70, Size: 19}, {Price: 68, Size: 29}},
		NoAsks:  []BookLevel{{Price: 84, Size: 5}},
	},
	{
		Lower: Ptr(2.06), Upper: Ptr(2.21),
		YesBids: []BookLevel{{Price: 30, Size: 7}, {Price: 29, Size: 54}, {Price: 27, Size: 58}},
		YesAsks: []BookLevel{{Price: 42, Size: 4}, {Price: 45, Size: 44}},
		NoBids:  []BookLevel{{Price: 57, Size: 7}, {Price: 55, Size: 28}},
		NoAsks:  []BookLevel{{Price: 70, Size: 16}},
	},
	{
		Lower: Ptr(2.21), Upper: Ptr(2.35),
		YesBids: []BookLevel{{Price: 15, Size: 62}, {Price: 14, Size: 111}, {Price: 12, Size: 130}},
		YesAsks: []BookLevel{{Price: 27, Size: 35}},
		NoBids:  []BookLevel{{Price: 72, Size: 16}, {Price: 70, Size: 29}},
		NoAsks:  []BookLevel{{Price: 99, Size: 6}},
	},
	{
		Lower: Ptr(2.35), Upper: nil,
		YesBids: []BookLevel{{Price: 4, Size: 499}, {Price: 3, Size: 667}, {Price: 1, Size: 2000}},
		NoBids:  []BookLevel{{Price: 75, Size: 37}, {Price: 74, Size: 24}, {Price: 73, Size: 51}},
		NoAsks:  []BookLevel{{Price: 97, Size: 3}, {Price: 99, Size: 20}},
	},
}

func TestFrozenBook_EveryBucketInsideItsTouch(t *testing.T) {
	for _, d := range nvdaFrozen {
		p, _, _, _, _ := EstimateFromDepth(d, DefaultHalfSpread)
		require.NotNil(t, p)
		assert.GreaterOrEqual(t, *p, d.ConsolidatedBids()[0].Price/100)
		assert.LessOrEqual(t, *p, d.ConsolidatedAsks()[0].Price/100)
	}
}

func TestFrozenBook_RawSumIsSpreadWideNotInflated(t *testing.T) {
	var total, lo, hi float64
	for _, d := range nvdaFrozen {
		p, _, _, _, _ := EstimateFromDepth(d, DefaultHalfSpread)
		require.NotNil(t, p)
		total += *p
		lo += d.ConsolidatedBids()[0].Price
		hi += d.ConsolidatedAsks()[0].Price
	}
	assert.GreaterOrEqual(t, total, lo/100)
	assert.LessOrEqual(t, total, hi/100)
	assert.Less(t, total, 1.10, "was 1.359 under the microprice")
}

// ═══════════════════════════════════════════════════════════════
// GRIEFING
// ═══════════════════════════════════════════════════════════════

var griefBounds = []bound{
	{nil, Ptr(-2.0)},
	{Ptr(-2.0), Ptr(-1.0)},
	{Ptr(-1.0), Ptr(1.0)},
	{Ptr(1.0), Ptr(2.0)},
	{Ptr(2.0), nil},
}

func TestGriefing_DustLadderIsWeightless(t *testing.T) {
	// A 1-share 90c bid on a dead tail moved the old estimator by 8 units.
	// Summed over the full book it is $0.90, under the side floor, so absent.
	base := buildDepth(cents(45, 5, 1, 5, 45), nil, griefBounds)
	f0 := FromDepth(base)
	require.NotNil(t, f0)

	griefed := append([]BucketDepth{}, base...)
	griefed[4] = BucketDepth{
		Lower: Ptr(2.0), Upper: nil,
		YesBids: []BookLevel{{Price: 90, Size: 1}},
	}
	f1 := FromDepth(griefed)
	require.NotNil(t, f1)

	// the dusted bucket must contribute exactly what an unquoted one would
	unquoted := FromDepth(append(append([]BucketDepth{}, base[:4]...),
		BucketDepth{Lower: Ptr(2.0), Upper: nil}))
	require.NotNil(t, unquoted)
	assert.InDelta(t, unquoted.Value, f1.Value, 1e-12)
	assert.Less(t, math.Abs(f1.Value-f0.Value), 8.0, "must not swing like before")
}

func TestGriefing_RealSizeAtTheSamePriceDoesCount(t *testing.T) {
	base := buildDepth(cents(45, 5, 1, 5, 45), nil, griefBounds)
	sized := append([]BucketDepth{}, base...)
	sized[4] = BucketDepth{
		Lower: Ptr(2.0), Upper: nil,
		YesBids: []BookLevel{{Price: 90, Size: 50}}, // $45 resting
	}

	f0, f1 := FromDepth(base), FromDepth(sized)
	require.NotNil(t, f0)
	require.NotNil(t, f1)
	assert.Greater(t, math.Abs(f1.Value-f0.Value), 1e-6, "a funded quote must matter")
}

// ═══════════════════════════════════════════════════════════════
// DEGENERATE INPUT
// ═══════════════════════════════════════════════════════════════

func TestFromDepth_AllEmptyReturnsNil(t *testing.T) {
	empty := make([]BucketDepth, len(depthBounds))
	for i, b := range depthBounds {
		empty[i] = BucketDepth{Lower: b.lower, Upper: b.upper}
	}
	assert.Nil(t, FromDepth(empty))
}

func TestFromDepth_ThinMarketIsDisclosed(t *testing.T) {
	// Relative standardisation compares each bucket to its own market's median
	// depth, so it cannot see a market that is poor EVERYWHERE. That gets a
	// warning instead, which is also the only exercise grouped0 gets.
	// Threading a narrow window: six consolidated sides each need to clear the
	// $5 side floor to stay quoted, which puts the floor of a fully quoted
	// market at $30, and the total has to land under the $50 warning line.
	// These sizes give $37.50 spread as 5.45/5.81, 5.27/5.99, 5.10/9.90.
	thin := buildDepth(cents(62.5, 31.25, 6.25), []float64{1.5, 3, 20}, threeBounds)
	f := FromDepth(thin)
	require.NotNil(t, f)
	assert.True(t, hasWarning(f, "total resting notional only $"),
		"a market under LowTotalNotionalWarnUSD must say so: %v", f.Warnings)

	// A market comfortably above the threshold says nothing.
	rich := buildDepth(cents(62.5, 31.25, 6.25), []float64{5000, 5000, 5000}, threeBounds)
	fr := FromDepth(rich)
	require.NotNil(t, fr)
	assert.False(t, hasWarning(fr, "total resting notional only $"))
}

func TestGrouped0_SeparatesThousands(t *testing.T) {
	// Go's fmt has no grouping verb, so this is hand-rolled and worth pinning.
	assert.Equal(t, "0", grouped0(0))
	assert.Equal(t, "999", grouped0(999))
	assert.Equal(t, "1,000", grouped0(1000))
	assert.Equal(t, "12,345", grouped0(12345.4))
	assert.Equal(t, "1,234,567", grouped0(1234567))
	assert.Equal(t, "-1,234", grouped0(-1234))
}

func TestFromDepth_OneSidedBucketSurvives(t *testing.T) {
	buckets := buildDepth(cents(62.5, 31.25, nil), nil, threeBounds)
	// ask-only after inversion
	buckets[2] = BucketDepth{
		Lower: Ptr(2.0), Upper: nil,
		NoBids: []BookLevel{{Price: 93, Size: 100}},
	}
	f := FromDepth(buckets)
	require.NotNil(t, f)
	assert.False(t, math.IsNaN(f.Value) || math.IsInf(f.Value, 0))
}

func TestFromDepth_CrossedConsolidatedTouchIsUnpriced(t *testing.T) {
	// YES bid 61.5 vs NO bid 45 -> consolidated ask 55 < bid 61.5: the arbitrage
	// state. Must be quarantined, not averaged.
	buckets := buildDepth(cents(62.5, 31.25, 6.25), nil, threeBounds)
	crossed := BucketDepth{
		Lower: nil, Upper: Ptr(1.0),
		YesBids: []BookLevel{{Price: 61.5, Size: 100}},
		NoBids:  []BookLevel{{Price: 45, Size: 100}}, // -> consolidated ask at 55, crossed
	}
	f := FromDepth(append([]BucketDepth{crossed}, buckets[1:]...))
	require.NotNil(t, f)
	assert.True(t, hasWarning(f, "crossed"))
}

func TestFromDepth_MatchesQuotePathOnSymmetricBooks(t *testing.T) {
	// On a symmetric mirrored book the depth path and the best-quote path must
	// agree on the probabilities (same mids, same residuals).
	btcBounds := []bound{
		{nil, Ptr(65000.0)},
		{Ptr(65000.0), Ptr(70000.0)},
		{Ptr(70000.0), nil},
	}
	mids := []float64{62.5, 31.25, 6.25}

	depth := buildDepth(cents(62.5, 31.25, 6.25), nil, btcBounds)
	quotes := make([]BucketBook, len(btcBounds))
	for i, b := range btcBounds {
		quotes[i] = BucketBook{
			Lower: b.lower, Upper: b.upper,
			BestBid: Ptr(mids[i] - 1), BestAsk: Ptr(mids[i] + 1),
		}
	}

	fd, fq := FromDepth(depth), FromBuckets(quotes)
	require.NotNil(t, fd)
	require.NotNil(t, fq)
	for i := range fd.Buckets {
		assert.InDelta(t, fq.Buckets[i].Probability, fd.Buckets[i].Probability, 1e-9)
	}
	assert.InDelta(t, fq.Value, fd.Value, 1e-6)
}
