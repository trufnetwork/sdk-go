package forecast

import "sort"

// Folding a market's two outcome books into the one ladder a trader can hit.
//
// A binary market's YES and NO books are two views of one position, and the
// matching engine treats them that way. A resting SELL NO at p burn-matches an
// incoming SELL YES at 100-p, and a resting BUY NO at p mint-matches an incoming
// BUY YES at 100-p. So, in the YES frame:
//
//	consolidated bids = YesBids + (100 - p for every NO ask)
//	consolidated asks = YesAsks + (100 - p for every NO bid)
//
// The sides swap. A NO ask is a YES bid, because hitting it means both parties
// sell and the pair burns. Verified on testnet (2026-08-03).
//
// Mint and burn both require the two prices to sum to EXACTLY 100: match_mint
// and match_burn in the node's 032-order-book-actions.sql return early
// otherwise, and select the resting order with price = rather than a range. Only
// a direct same-outcome match crosses prices. That is why every level here
// carries its native and inverse volume separately instead of only the sum: one
// order sweeps every native level past its limit but exactly ONE inverse level,
// so anything quoting a fill needs the split to get the answer right.

// BookSide is which way a consolidated side sorts: bids best-highest, asks
// best-lowest.
type BookSide int

const (
	// BidSide sorts highest price first.
	BidSide BookSide = iota
	// AskSide sorts lowest price first.
	AskSide
)

// complement is what a YES price and its NO price always sum to.
const complement = 100

// ConsolidatedLevel is one price level of a consolidated order book, in the
// requested outcome's frame.
//
// Native rests in this outcome's own book and fills as a direct match. Inverse
// rests in the opposite outcome's book at complement-Price, and fills by minting
// a share pair on the ask side or burning one on the bid side.
type ConsolidatedLevel struct {
	// Price is the level's price in cents, 1-99.
	Price float64
	// Total is the shares available here: Native + Inverse.
	Total float64
	// Native is the shares resting in this outcome's own book.
	Native float64
	// Inverse is the shares resting in the opposite outcome's book at 100-Price.
	Inverse float64
}

// ConsolidatedOrderBook is one outcome's order book with the opposite outcome's
// quotes folded in.
type ConsolidatedOrderBook struct {
	// QueryID is the market this book belongs to.
	QueryID int
	// Outcome is which side the prices are framed in: true=YES, false=NO.
	Outcome bool
	// Bids are the executable bids, best (highest) first.
	Bids []ConsolidatedLevel
	// Asks are the executable asks, best (lowest) first.
	Asks []ConsolidatedLevel
	// IsCrossed reports whether the best bid sits at or above the best ask.
	//
	// A consolidated book can be crossed and stay that way. A YES bid at 61 and
	// a NO bid at 45 read as a bid at 61 over an ask at 55, and the engine will
	// never match them because 61 + 45 is not 100. Render it rather than
	// treating it as bad data.
	IsCrossed bool
}

// InversePrice returns the price in the opposite outcome's frame that this price
// is hittable at.
func InversePrice(price float64) float64 { return round4(complement - price) }

// ConsolidateSide returns one side of a consolidated ladder: this outcome's own
// levels plus the opposite outcome's levels inverted into this frame.
//
// Pass the opposite side of the opposite book. Consolidated bids take the
// opposite outcome's asks; consolidated asks take its bids.
//
// Levels landing on the same price merge into one entry that keeps the
// native/inverse split. Zero and negative sizes are dropped.
func ConsolidateSide(native, opposite []BookLevel, side BookSide) []ConsolidatedLevel {
	byPrice := make(map[float64]*ConsolidatedLevel, len(native)+len(opposite))
	order := make([]float64, 0, len(native)+len(opposite))

	add := func(price, size float64, inverse bool) {
		if size <= 0 {
			return
		}
		level, ok := byPrice[price]
		if !ok {
			level = &ConsolidatedLevel{Price: price}
			byPrice[price] = level
			order = append(order, price)
		}
		if inverse {
			level.Inverse += size
		} else {
			level.Native += size
		}
		level.Total += size
	}

	for _, l := range native {
		add(l.Price, l.Size, false)
	}
	for _, l := range opposite {
		add(InversePrice(l.Price), l.Size, true)
	}

	levels := make([]ConsolidatedLevel, 0, len(order))
	for _, price := range order {
		levels = append(levels, *byPrice[price])
	}
	sort.SliceStable(levels, func(i, j int) bool {
		if side == BidSide {
			return levels[i].Price > levels[j].Price
		}
		return levels[i].Price < levels[j].Price
	})
	return levels
}

// reflectLevel moves one level into the opposite outcome's frame.
//
// Native and inverse swap because the resting order belongs to the other
// outcome once the frame flips: this outcome's own book is the opposite
// outcome's inverse, and the other way round.
func reflectLevel(level ConsolidatedLevel) ConsolidatedLevel {
	return ConsolidatedLevel{
		Price:   InversePrice(level.Price),
		Total:   level.Total,
		Native:  level.Inverse,
		Inverse: level.Native,
	}
}

// ReflectConsolidatedBook returns the same market in the opposite outcome's
// frame, without reading the chain again.
//
// Both outcome views come from one get_full_market_depth response, so the
// opposite view is an exact reflection rather than new information: prices
// complement to 100, asks and bids swap, and native and inverse volume swap
// with them. Reading the second outcome separately costs another round trip and
// risks stitching two different moments together.
func ReflectConsolidatedBook(book ConsolidatedOrderBook) ConsolidatedOrderBook {
	bids := make([]ConsolidatedLevel, 0, len(book.Asks))
	for _, level := range book.Asks {
		bids = append(bids, reflectLevel(level))
	}
	asks := make([]ConsolidatedLevel, 0, len(book.Bids))
	for _, level := range book.Bids {
		asks = append(asks, reflectLevel(level))
	}

	sort.SliceStable(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price })
	sort.SliceStable(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })

	return ConsolidatedOrderBook{
		QueryID:   book.QueryID,
		Outcome:   !book.Outcome,
		Bids:      bids,
		Asks:      asks,
		IsCrossed: Crossed(bids, asks),
	}
}

// Crossed reports whether the best bid sits at or above the best ask.
func Crossed(bids, asks []ConsolidatedLevel) bool {
	return len(bids) > 0 && len(asks) > 0 && bids[0].Price >= asks[0].Price
}

// flatten drops the native/inverse split, for callers that only need the ladder.
func flatten(levels []ConsolidatedLevel) []BookLevel {
	out := make([]BookLevel, 0, len(levels))
	for _, l := range levels {
		out = append(out, BookLevel{Price: l.Price, Size: l.Total})
	}
	return out
}
