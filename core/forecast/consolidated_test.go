// Tests for folding a market's two outcome books into one executable ladder.
//
// What matters here is that the SIDES swap (a NO ask is a YES bid, not a YES
// ask) and that each level keeps its native/inverse split, since mint and burn
// only fire at the exact complement and a caller quoting a fill needs to know
// which volume is which.

package forecast

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsolidateSide_NoAskBecomesAYesBid(t *testing.T) {
	// The case from truflation/website#4385: an ask for 4 shares of NO at 93c is
	// economically a bid for 4 shares of YES at 7c, and the chain burns the pair
	// to fill it.
	bids := ConsolidateSide(nil, []BookLevel{{Price: 93, Size: 4}}, BidSide)

	require.Len(t, bids, 1)
	assert.Equal(t, ConsolidatedLevel{Price: 7, Total: 4, Native: 0, Inverse: 4}, bids[0])
}

func TestConsolidateSide_NoBidBecomesAYesAsk(t *testing.T) {
	asks := ConsolidateSide(nil, []BookLevel{{Price: 41, Size: 200}}, AskSide)

	require.Len(t, asks, 1)
	assert.Equal(t, ConsolidatedLevel{Price: 59, Total: 200, Native: 0, Inverse: 200}, asks[0])
}

func TestConsolidateSide_MergesOnePriceAndKeepsTheSplit(t *testing.T) {
	asks := ConsolidateSide(
		[]BookLevel{{Price: 59, Size: 100}},
		[]BookLevel{{Price: 41, Size: 200}},
		AskSide,
	)

	require.Len(t, asks, 1)
	assert.Equal(t, ConsolidatedLevel{Price: 59, Total: 300, Native: 100, Inverse: 200}, asks[0])
}

func TestConsolidateSide_Sorts(t *testing.T) {
	asks := ConsolidateSide(
		[]BookLevel{{Price: 60, Size: 100}},
		[]BookLevel{{Price: 41, Size: 200}, {Price: 45, Size: 50}},
		AskSide,
	)
	assert.Equal(t, []float64{55, 59, 60}, prices(asks))

	bids := ConsolidateSide(
		[]BookLevel{{Price: 30, Size: 10}},
		[]BookLevel{{Price: 55, Size: 25}},
		BidSide,
	)
	assert.Equal(t, []float64{45, 30}, prices(bids))
}

func TestConsolidateSide_DropsEmptyLevels(t *testing.T) {
	levels := ConsolidateSide(
		[]BookLevel{{Price: 45, Size: 0}, {Price: 44, Size: 10}},
		[]BookLevel{{Price: 20, Size: -5}},
		BidSide,
	)

	require.Len(t, levels, 1)
	assert.Equal(t, float64(44), levels[0].Price)
}

func TestConsolidateSide_MirrorsBetweenFrames(t *testing.T) {
	yesBids := []BookLevel{{Price: 30, Size: 10}}
	yesAsks := []BookLevel{{Price: 60, Size: 100}}
	noBids := []BookLevel{{Price: 41, Size: 200}}
	noAsks := []BookLevel{{Price: 55, Size: 25}}

	yesFrame := ConsolidateSide(yesBids, noAsks, BidSide)
	noFrame := ConsolidateSide(noAsks, yesBids, AskSide)

	// Price q on one side is 100-q on the other, bids become asks, and what was
	// native in the YES frame is inverse in the NO frame.
	require.Len(t, noFrame, len(yesFrame))
	for i, l := range yesFrame {
		assert.Equal(t, ConsolidatedLevel{
			Price:   100 - l.Price,
			Total:   l.Total,
			Native:  l.Inverse,
			Inverse: l.Native,
		}, noFrame[i])
	}

	// And the same the other way, so the reflection is not an artifact of which
	// side happened to be listed first.
	yesAskFrame := ConsolidateSide(yesAsks, noBids, AskSide)
	noBidFrame := ConsolidateSide(noBids, yesAsks, BidSide)
	require.Len(t, noBidFrame, len(yesAskFrame))
	for i, l := range yesAskFrame {
		assert.Equal(t, 100-l.Price, noBidFrame[i].Price)
		assert.Equal(t, l.Total, noBidFrame[i].Total)
	}
}

func TestConsolidateSide_InvertsFractionalPricesWithoutFloatDust(t *testing.T) {
	levels := ConsolidateSide(nil, []BookLevel{{Price: 0.1, Size: 1}}, BidSide)

	require.Len(t, levels, 1)
	assert.Equal(t, 99.9, levels[0].Price)
}

func TestCrossed(t *testing.T) {
	// A YES bid at 61 and a NO bid at 45 read as a bid at 61 over an ask at 55.
	// 61 + 45 is 106, so no mint fires and the crossing rests indefinitely.
	bids := ConsolidateSide([]BookLevel{{Price: 61, Size: 30}}, nil, BidSide)
	asks := ConsolidateSide(nil, []BookLevel{{Price: 45, Size: 30}}, AskSide)

	require.Equal(t, float64(61), bids[0].Price)
	require.Equal(t, float64(55), asks[0].Price)
	assert.True(t, Crossed(bids, asks))

	assert.False(t, Crossed(bids, nil))
	assert.False(t, Crossed(nil, asks))
	assert.False(t, Crossed(
		ConsolidateSide([]BookLevel{{Price: 45, Size: 10}}, nil, BidSide),
		ConsolidateSide(nil, []BookLevel{{Price: 44, Size: 20}}, AskSide),
	))
}

func TestBucketLaddersMergeWithoutMovingTheForecastInputs(t *testing.T) {
	// ConsolidatedBids/Asks now merge same-price levels. Everything downstream
	// sums price x size or walks in price order, so the ladder totals the
	// forecast reads must be unchanged.
	depth := BucketDepth{
		YesBids: []BookLevel{{Price: 40, Size: 10}, {Price: 38, Size: 5}},
		NoAsks:  []BookLevel{{Price: 60, Size: 7}}, // -> YES bid at 40
		YesAsks: []BookLevel{{Price: 44, Size: 3}},
		NoBids:  []BookLevel{{Price: 56, Size: 9}}, // -> YES ask at 44
	}

	bids := depth.ConsolidatedBids()
	assert.Equal(t, []BookLevel{{Price: 40, Size: 17}, {Price: 38, Size: 5}}, bids)
	assert.Equal(t, 40*17.0+38*5.0, centShares(bids))

	asks := depth.ConsolidatedAsks()
	assert.Equal(t, []BookLevel{{Price: 44, Size: 12}}, asks)
	assert.Equal(t, 44*12.0, centShares(asks))
}

func prices(levels []ConsolidatedLevel) []float64 {
	out := make([]float64, 0, len(levels))
	for _, l := range levels {
		out = append(out, l.Price)
	}
	return out
}

func centShares(levels []BookLevel) float64 {
	total := 0.0
	for _, l := range levels {
		total += l.Price * l.Size
	}
	return total
}

func TestReflectConsolidatedBook_SwapsSidesAndVolume(t *testing.T) {
	// A YES ask at 60 holding 10 YES sells and 4 NO buys (resting at 40) is, in
	// the NO frame, a NO bid at 40 holding those 4 NO buys natively and the 10
	// YES sells as its inverse.
	yes := ConsolidatedOrderBook{
		QueryID: 419,
		Outcome: true,
		Asks:    []ConsolidatedLevel{{Price: 60, Total: 14, Native: 10, Inverse: 4}},
		Bids:    []ConsolidatedLevel{{Price: 30, Total: 20, Native: 20}},
	}

	no := ReflectConsolidatedBook(yes)

	assert.Equal(t, 419, no.QueryID)
	assert.False(t, no.Outcome)
	assert.Equal(t, []ConsolidatedLevel{{Price: 40, Total: 14, Native: 4, Inverse: 10}}, no.Bids)
	assert.Equal(t, []ConsolidatedLevel{{Price: 70, Total: 20, Native: 0, Inverse: 20}}, no.Asks)
}

func TestReflectConsolidatedBook_RoundTripsToItself(t *testing.T) {
	yes := ConsolidatedOrderBook{
		QueryID: 419,
		Outcome: true,
		Asks: []ConsolidatedLevel{
			{Price: 16, Total: 349, Native: 320, Inverse: 29},
			{Price: 19, Total: 342, Native: 289, Inverse: 53},
		},
		Bids: []ConsolidatedLevel{
			{Price: 4, Total: 1082, Native: 1049, Inverse: 33},
			{Price: 1, Total: 4339, Native: 4283, Inverse: 56},
		},
	}
	yes.IsCrossed = Crossed(yes.Bids, yes.Asks)

	assert.Equal(t, yes, ReflectConsolidatedBook(ReflectConsolidatedBook(yes)))
}

func TestReflectConsolidatedBook_MatchesASecondConsolidation(t *testing.T) {
	// The reflection has to agree with reading the other outcome directly,
	// because that is the round trip it replaces.
	yesBids := []BookLevel{{Price: 40, Size: 20}}
	yesAsks := []BookLevel{{Price: 60, Size: 10}}
	noBids := []BookLevel{{Price: 30, Size: 7}}
	noAsks := []BookLevel{{Price: 55, Size: 3}}

	yes := ConsolidatedOrderBook{
		QueryID: 419,
		Outcome: true,
		Bids:    ConsolidateSide(yesBids, noAsks, BidSide),
		Asks:    ConsolidateSide(yesAsks, noBids, AskSide),
	}
	yes.IsCrossed = Crossed(yes.Bids, yes.Asks)

	direct := ConsolidatedOrderBook{
		QueryID: 419,
		Outcome: false,
		Bids:    ConsolidateSide(noBids, yesAsks, BidSide),
		Asks:    ConsolidateSide(noAsks, yesBids, AskSide),
	}
	direct.IsCrossed = Crossed(direct.Bids, direct.Asks)

	assert.Equal(t, direct, ReflectConsolidatedBook(yes))
}
