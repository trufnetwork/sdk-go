package forecast

import (
	"math"
	"sort"
)

// Quoting a fill against a consolidated order book.
//
// A consolidated ladder is not sweepable. match_direct crosses through the
// order's limit, but match_mint and match_burn only fire when the two prices sum
// to exactly 100. So an order at limit P fills every native level past P plus
// exactly one inverse level, the one at P.
//
// One consequence is worth knowing before reading a quote: fillable size does
// not grow monotonically with the limit price. Raising the limit can lose the
// inverse level the fill was counting on, so the model evaluates every price
// rather than walking the ladder.
//
// The estimate assumes the order reaches the front of the queue at its price.
// Matching is FIFO within a level, so an older order resting at the same price
// takes the counterparty first and the real fill comes up short.
//
// Prices compare by equality here, which is safe for the levels ConsolidateSide
// produces: a whole-cent price survives float64 exactly, and so does its
// complement, since 100-p is exact for integral p.

// FillPath is how one leg of a fill reaches the chain.
type FillPath string

const (
	// FillDirect matches an order resting in the same outcome's book.
	FillDirect FillPath = "direct"
	// FillMint pairs two buys, minting a share pair. Their prices sum to 100.
	FillMint FillPath = "mint"
	// FillBurn pairs two sells, burning a share pair. Their prices sum to 100.
	FillBurn FillPath = "burn"
)

// ConsolidatedFill is one leg of a quoted fill.
type ConsolidatedFill struct {
	// Price is the leg's price in cents.
	Price float64
	// Shares is how much fills on this leg.
	Shares float64
	// Path is how the leg reaches the chain.
	Path FillPath
}

// ConsolidatedBuyQuote is what a buy of a given size can expect.
type ConsolidatedBuyQuote struct {
	// LimitPrice is the price to submit, in cents, chosen by the model. It is 0
	// when the book holds no tradable level, which no real price can be.
	LimitPrice float64
	// FilledShares is how much fills at LimitPrice.
	FilledShares float64
	// AvailableShares is the most one order can fill at any price. It is less
	// than the ladder's total whenever inverse volume rests at more than one
	// price.
	AvailableShares float64
	// EstimatedTotalCost is dollars paid: native legs at their own price and the
	// inverse leg at the limit.
	EstimatedTotalCost float64
	// AveragePrice is the blended price in cents across every leg, 0 when
	// nothing fills.
	AveragePrice float64
	// IsFullyFilled reports whether the whole requested size fills.
	IsFullyFilled bool
	// Fills is how the fill breaks down, in the order the engine executes it.
	Fills []ConsolidatedFill
}

// ConsolidatedSellQuote is what a sell of a given size can expect.
type ConsolidatedSellQuote struct {
	// LimitPrice is the price to submit, in cents, chosen by the model. It is 0
	// when the book holds no tradable level, which no real price can be.
	LimitPrice float64
	// FilledShares is how much fills at LimitPrice.
	FilledShares float64
	// AvailableShares is the most one order can fill at any price.
	AvailableShares float64
	// EstimatedProceeds is dollars received. Every share pays the submitted
	// limit.
	EstimatedProceeds float64
	// AveragePrice is the blended price in cents, which for a sell is always the
	// submitted limit. It is 0 when nothing fills.
	AveragePrice float64
	// IsFullyFilled reports whether the whole requested size fills.
	IsFullyFilled bool
	// Fills is how the fill breaks down, in the order the engine executes it.
	Fills []ConsolidatedFill
}

// tradableLevels drops the levels the engine cannot trade at.
//
// place_buy_order and place_sell_order both reject a price outside 1-99, so a
// level there can never be the limit and must not be picked as one.
func tradableLevels(levels []ConsolidatedLevel, side BookSide) []ConsolidatedLevel {
	out := make([]ConsolidatedLevel, 0, len(levels))
	for _, level := range levels {
		if level.Price >= 1 && level.Price <= 99 {
			out = append(out, level)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if side == BidSide {
			return out[i].Price > out[j].Price
		}
		return out[i].Price < out[j].Price
	})
	return out
}

// simulateBuy reports the shares filled and cents paid by a buy of shares
// submitted at limit. asks must be sorted best (lowest) first.
func simulateBuy(asks []ConsolidatedLevel, shares, limit float64) (filled, costCents float64, fills []ConsolidatedFill) {
	remaining := shares
	fills = []ConsolidatedFill{}

	for _, level := range asks {
		if level.Price > limit || remaining <= 0 {
			break
		}

		take := math.Min(level.Native, remaining)
		if take <= 0 {
			continue
		}

		filled += take
		costCents += take * level.Price
		remaining -= take
		fills = append(fills, ConsolidatedFill{Price: level.Price, Shares: take, Path: FillDirect})
	}

	if remaining > 0 {
		for _, level := range asks {
			if level.Price != limit {
				continue
			}
			if take := math.Min(level.Inverse, remaining); take > 0 {
				filled += take
				costCents += take * limit
				remaining -= take
				fills = append(fills, ConsolidatedFill{Price: limit, Shares: take, Path: FillMint})
			}
			break
		}
	}

	return filled, costCents, fills
}

// simulateSell reports the shares filled by a sell of shares submitted at limit.
// bids must be sorted best (highest) first.
//
// Proceeds are uniform: a direct match pays the seller the ask price and refunds
// the buyer the difference, and a burn pays each side its own price, so every
// share pays limit.
func simulateSell(bids []ConsolidatedLevel, shares, limit float64) (filled float64, fills []ConsolidatedFill) {
	remaining := shares
	fills = []ConsolidatedFill{}

	for _, level := range bids {
		if level.Price < limit || remaining <= 0 {
			break
		}

		take := math.Min(level.Native, remaining)
		if take <= 0 {
			continue
		}

		filled += take
		remaining -= take
		fills = append(fills, ConsolidatedFill{Price: limit, Shares: take, Path: FillDirect})
	}

	if remaining > 0 {
		for _, level := range bids {
			if level.Price != limit {
				continue
			}
			if take := math.Min(level.Inverse, remaining); take > 0 {
				filled += take
				remaining -= take
				fills = append(fills, ConsolidatedFill{Price: limit, Shares: take, Path: FillBurn})
			}
			break
		}
	}

	return filled, fills
}

// buyableShares is the most any single buy can take out of this ladder.
func buyableShares(asks []ConsolidatedLevel) float64 {
	most := 0.0
	for _, level := range asks {
		filled, _, _ := simulateBuy(asks, math.Inf(1), level.Price)
		most = math.Max(most, filled)
	}
	return most
}

// sellableShares is the most any single sell can place into this ladder.
func sellableShares(bids []ConsolidatedLevel) float64 {
	most := 0.0
	for _, level := range bids {
		filled, _ := simulateSell(bids, math.Inf(1), level.Price)
		most = math.Max(most, filled)
	}
	return most
}

// QuoteConsolidatedBuyAtPrice quotes a buy at a limit the caller has already
// chosen.
//
// Pass the consolidated asks. Use this when the routing policy is the caller's:
// QuoteConsolidatedBuy picks the cheapest limit that fills the whole order, and
// a caller wanting the largest fill, a price ceiling or the least market impact
// wants this instead.
func QuoteConsolidatedBuyAtPrice(levels []ConsolidatedLevel, shares, limit float64) ConsolidatedBuyQuote {
	asks := tradableLevels(levels, AskSide)
	filled, costCents, fills := simulateBuy(asks, shares, limit)

	average := 0.0
	if filled > 0 {
		average = costCents / filled
	}

	return ConsolidatedBuyQuote{
		LimitPrice:         limit,
		FilledShares:       filled,
		AvailableShares:    buyableShares(asks),
		EstimatedTotalCost: costCents / 100,
		AveragePrice:       average,
		IsFullyFilled:      shares > 0 && filled >= shares,
		Fills:              fills,
	}
}

// QuoteConsolidatedBuy quotes a buy of shares against the consolidated asks,
// choosing the cheapest limit that fills the most.
func QuoteConsolidatedBuy(levels []ConsolidatedLevel, shares float64) ConsolidatedBuyQuote {
	asks := tradableLevels(levels, AskSide)

	var best *ConsolidatedBuyQuote
	for _, candidate := range asks {
		filled, costCents, fills := simulateBuy(asks, shares, candidate.Price)

		if best == nil || filled > best.FilledShares {
			average := 0.0
			if filled > 0 {
				average = costCents / filled
			}
			best = &ConsolidatedBuyQuote{
				LimitPrice:         candidate.Price,
				FilledShares:       filled,
				EstimatedTotalCost: costCents / 100,
				AveragePrice:       average,
				IsFullyFilled:      shares > 0 && filled >= shares,
				Fills:              fills,
			}
		}

		if best.FilledShares >= shares {
			break
		}
	}

	if best == nil {
		return ConsolidatedBuyQuote{Fills: []ConsolidatedFill{}}
	}

	best.AvailableShares = buyableShares(asks)
	return *best
}

// QuoteConsolidatedSellAtPrice quotes a sell at a limit the caller has already
// chosen.
//
// Pass the consolidated bids. Needed when something downstream of the quote
// moves the price, such as a self-trade guard raising it clear of the seller's
// own resting buy order, and whenever the routing policy is the caller's rather
// than the one QuoteConsolidatedSell applies.
func QuoteConsolidatedSellAtPrice(levels []ConsolidatedLevel, shares, limit float64) ConsolidatedSellQuote {
	bids := tradableLevels(levels, BidSide)
	filled, fills := simulateSell(bids, shares, limit)

	average := 0.0
	if filled > 0 {
		average = limit
	}

	return ConsolidatedSellQuote{
		LimitPrice:        limit,
		FilledShares:      filled,
		AvailableShares:   sellableShares(bids),
		EstimatedProceeds: filled * limit / 100,
		AveragePrice:      average,
		IsFullyFilled:     shares > 0 && filled >= shares,
		Fills:             fills,
	}
}

// QuoteConsolidatedSell quotes a sell of shares against the consolidated bids,
// choosing the highest limit that fills the most.
func QuoteConsolidatedSell(levels []ConsolidatedLevel, shares float64) ConsolidatedSellQuote {
	bids := tradableLevels(levels, BidSide)

	var best *ConsolidatedSellQuote
	for _, candidate := range bids {
		filled, fills := simulateSell(bids, shares, candidate.Price)

		if best == nil || filled > best.FilledShares {
			average := 0.0
			if filled > 0 {
				average = candidate.Price
			}
			best = &ConsolidatedSellQuote{
				LimitPrice:        candidate.Price,
				FilledShares:      filled,
				EstimatedProceeds: filled * candidate.Price / 100,
				AveragePrice:      average,
				IsFullyFilled:     shares > 0 && filled >= shares,
				Fills:             fills,
			}
		}

		if best.FilledShares >= shares {
			break
		}
	}

	if best == nil {
		return ConsolidatedSellQuote{Fills: []ConsolidatedFill{}}
	}

	best.AvailableShares = sellableShares(bids)
	return *best
}
