// Tests for quoting a fill against a consolidated ladder.
//
// What matters here is that the ladder is not sweepable: an order at limit P
// takes every native level past P but exactly ONE inverse level, the one at P.
// So fillable size is not monotonic in the limit, the ladder's total is not
// reachable by any single order, and a sell pays its limit on every share.

package forecast

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuoteConsolidatedBuy_FillsWhereTheInverseLegIsReachable(t *testing.T) {
	// YES asks 100 @ 60, NO bids 200 @ 41 and 50 @ 45.
	asks := []ConsolidatedLevel{
		{Price: 55, Total: 50, Inverse: 50},
		{Price: 59, Total: 200, Inverse: 200},
		{Price: 60, Total: 100, Native: 100},
	}

	quote := QuoteConsolidatedBuy(asks, 100)

	assert.Equal(t, 59.0, quote.LimitPrice)
	assert.Equal(t, 100.0, quote.FilledShares)
	assert.Equal(t, 59.0, quote.EstimatedTotalCost)
}

func TestQuoteConsolidatedBuy_ReportsTheBestFillOneOrderCanGet(t *testing.T) {
	asks := []ConsolidatedLevel{
		{Price: 55, Total: 50, Inverse: 50},
		{Price: 59, Total: 200, Inverse: 200},
		{Price: 60, Total: 100, Native: 100},
	}

	ladder := 0.0
	for _, level := range asks {
		ladder += level.Total
	}
	require.Equal(t, 350.0, ladder)

	// Summing the ladder says 350. No single order can do better than 200.
	quote := QuoteConsolidatedBuy(asks, 350)

	assert.Equal(t, 200.0, quote.AvailableShares)
	assert.False(t, quote.IsFullyFilled)
}

func TestQuoteConsolidatedBuy_GainsNoFillByRaisingThePastTheInverseLeg(t *testing.T) {
	asks := []ConsolidatedLevel{
		{Price: 59, Total: 200, Inverse: 200},
		{Price: 60, Total: 100, Native: 100},
	}

	// A limit of 60 reaches 100 native shares; a limit of 59 reaches 200 inverse
	// ones. More fill sits at the lower price.
	assert.Equal(t, 59.0, QuoteConsolidatedBuy(asks, 150).LimitPrice)
	assert.Equal(t, 150.0, QuoteConsolidatedBuy(asks, 150).FilledShares)
	assert.Equal(t, 200.0, QuoteConsolidatedBuy(asks, 250).FilledShares)
	assert.Equal(t, 59.0, QuoteConsolidatedBuy(asks, 250).LimitPrice)
}

func TestQuoteConsolidatedBuy_PricesNativeLegsOwnAndInverseAtTheLimit(t *testing.T) {
	asks := []ConsolidatedLevel{
		{Price: 20, Total: 40, Native: 40},
		{Price: 30, Total: 60, Inverse: 60},
	}

	quote := QuoteConsolidatedBuy(asks, 100)

	assert.Equal(t, 30.0, quote.LimitPrice)
	assert.Equal(t, 26.0, quote.EstimatedTotalCost)
	assert.Equal(t, []ConsolidatedFill{
		{Price: 20, Shares: 40, Path: FillDirect},
		{Price: 30, Shares: 60, Path: FillMint},
	}, quote.Fills)
}

func TestQuoteConsolidatedBuy_AveragesThePriceActuallyPaid(t *testing.T) {
	asks := []ConsolidatedLevel{
		{Price: 20, Total: 40, Native: 40},
		{Price: 30, Total: 60, Inverse: 60},
	}

	assert.Equal(t, 26.0, QuoteConsolidatedBuy(asks, 100).AveragePrice)
}

func TestQuoteConsolidatedSell_PaysTheLimitOnEveryShare(t *testing.T) {
	bids := []ConsolidatedLevel{
		{Price: 80, Total: 50, Native: 50},
		{Price: 70, Total: 50, Native: 50},
	}

	// match_direct pays the seller at the ask price, so walking the ladder and
	// crediting 50 at 80 plus 50 at 70 overstates this by $5.
	quote := QuoteConsolidatedSell(bids, 100)

	assert.Equal(t, 70.0, quote.LimitPrice)
	assert.Equal(t, 100.0, quote.FilledShares)
	assert.Equal(t, 70.0, quote.EstimatedProceeds)
}

func TestQuoteConsolidatedSell_CombinesADirectLegAndABurnLeg(t *testing.T) {
	bids := []ConsolidatedLevel{
		{Price: 70, Total: 70, Native: 30, Inverse: 40},
		{Price: 60, Total: 100, Native: 100},
	}

	quote := QuoteConsolidatedSell(bids, 70)

	assert.Equal(t, 70.0, quote.LimitPrice)
	assert.Equal(t, 70.0, quote.FilledShares)
	assert.Equal(t, 49.0, quote.EstimatedProceeds)
	assert.Equal(t, []ConsolidatedFill{
		{Price: 70, Shares: 30, Path: FillDirect},
		{Price: 70, Shares: 40, Path: FillBurn},
	}, quote.Fills)
}

func TestQuoteConsolidatedSell_ReachesTheInverseLegOnlyAtTheExactLimit(t *testing.T) {
	bids := []ConsolidatedLevel{
		{Price: 65, Total: 80, Inverse: 80},
		{Price: 60, Total: 40, Native: 40},
	}

	quote := QuoteConsolidatedSell(bids, 80)

	assert.Equal(t, 65.0, quote.LimitPrice)
	assert.Equal(t, 52.0, quote.EstimatedProceeds)
	assert.Equal(t, []ConsolidatedFill{{Price: 65, Shares: 80, Path: FillBurn}}, quote.Fills)
}

func TestQuoteConsolidated_IgnoresLevelsOutsideTheTradableRange(t *testing.T) {
	asks := []ConsolidatedLevel{
		{Price: 0, Total: 500, Native: 500},
		{Price: 100, Total: 500, Native: 500},
		{Price: 40, Total: 60, Native: 60},
	}

	quote := QuoteConsolidatedBuy(asks, 60)

	assert.Equal(t, 40.0, quote.LimitPrice)
	assert.Equal(t, 60.0, quote.AvailableShares)
}

func TestQuoteConsolidatedSellAtPrice_QuotesACallerSuppliedLimit(t *testing.T) {
	bids := []ConsolidatedLevel{
		{Price: 80, Total: 50, Native: 50},
		{Price: 70, Total: 50, Native: 50},
	}

	quote := QuoteConsolidatedSellAtPrice(bids, 100, 80)

	// Only the 80 bid is reachable at a limit of 80, and it pays 80 a share.
	assert.Equal(t, 50.0, quote.FilledShares)
	assert.Equal(t, 40.0, quote.EstimatedProceeds)
	assert.False(t, quote.IsFullyFilled)
}

func TestQuoteConsolidatedBuyAtPrice_QuotesACallerSuppliedLimit(t *testing.T) {
	asks := []ConsolidatedLevel{
		{Price: 59, Total: 200, Inverse: 200},
		{Price: 60, Total: 100, Native: 100},
	}

	// QuoteConsolidatedBuy would pick 59 for the larger fill. A caller willing to
	// pay up for the native side gets to say so.
	quote := QuoteConsolidatedBuyAtPrice(asks, 250, 60)

	assert.Equal(t, 60.0, quote.LimitPrice)
	assert.Equal(t, 100.0, quote.FilledShares)
	assert.Equal(t, 60.0, quote.EstimatedTotalCost)
	assert.False(t, quote.IsFullyFilled)
}

func TestQuoteConsolidated_EmptyBookQuotesNothing(t *testing.T) {
	buy := QuoteConsolidatedBuy(nil, 100)
	assert.Zero(t, buy.LimitPrice)
	assert.Zero(t, buy.FilledShares)
	assert.Zero(t, buy.AvailableShares)
	assert.False(t, buy.IsFullyFilled)
	assert.Empty(t, buy.Fills)

	sell := QuoteConsolidatedSell(nil, 100)
	assert.Zero(t, sell.LimitPrice)
	assert.Zero(t, sell.FilledShares)
	assert.False(t, sell.IsFullyFilled)
	assert.Empty(t, sell.Fills)
}

func TestQuoteConsolidated_SortsAnUnorderedLadder(t *testing.T) {
	// ConsolidateSide sorts, but a caller can hand-build a ladder and the model
	// must not quote a worse price off the input order.
	asks := []ConsolidatedLevel{
		{Price: 60, Total: 100, Native: 100},
		{Price: 20, Total: 40, Native: 40},
	}

	assert.Equal(t, 20.0, QuoteConsolidatedBuy(asks, 40).LimitPrice)
}

// A frozen snapshot of mainnet market 419, read 2026-08-12 through
// get_full_market_depth.
//
//	YES bids  1c x4283, 3c x1428, 4c x1049
//	YES asks 16c x320, 17c x324, 19c x289
//	NO  bids 81c x53,  83c x51,  84c x29
//	NO  asks 96c x33,  97c x57,  99c x56
//
// The NO orders fold into the YES frame at 100 - price, which lands each of them
// on a price the YES book already quotes.
var (
	market419Asks = []ConsolidatedLevel{
		{Price: 16, Total: 349, Native: 320, Inverse: 29},
		{Price: 17, Total: 375, Native: 324, Inverse: 51},
		{Price: 19, Total: 342, Native: 289, Inverse: 53},
	}
	market419Bids = []ConsolidatedLevel{
		{Price: 4, Total: 1082, Native: 1049, Inverse: 33},
		{Price: 3, Total: 1485, Native: 1428, Inverse: 57},
		{Price: 1, Total: 4339, Native: 4283, Inverse: 56},
	}
)

func TestMarket419_OneBuyCannotTakeMoreThanTheBestSinglePriceAllows(t *testing.T) {
	assert.Equal(t, 986.0, QuoteConsolidatedBuy(market419Asks, 99_999).AvailableShares)
}

func TestMarket419_ABuyInsideNativeDepthNeverReachesAnInverseLeg(t *testing.T) {
	quote := QuoteConsolidatedBuy(market419Asks, 700)

	assert.Equal(t, 19.0, quote.LimitPrice)
	assert.InDelta(t, 116.92, quote.EstimatedTotalCost, 1e-6)
	for _, fill := range quote.Fills {
		assert.Equal(t, FillDirect, fill.Path)
	}
}

func TestMarket419_ASellPastTheTopBidPaysItsLimitOnEveryShare(t *testing.T) {
	quote := QuoteConsolidatedSell(market419Bids, 2000)

	// 1049 rest at 4c, but the order only fills in full at a limit of 3c, and
	// every share then pays 3c. Walking the ladder would have quoted 70.49.
	assert.Equal(t, 3.0, quote.LimitPrice)
	assert.InDelta(t, 60.0, quote.EstimatedProceeds, 1e-6)
}

func TestQuoteConsolidated_AvailableSharesToleratesAnUnboundedRequest(t *testing.T) {
	// The available-shares scan runs an unbounded fill, so the arithmetic has to
	// survive an infinite remaining size rather than looping or going NaN.
	filled, cost, fills := simulateBuy(market419Asks, math.Inf(1), 19)

	assert.Equal(t, 986.0, filled)
	assert.False(t, math.IsNaN(cost))
	assert.Len(t, fills, 4)
}
