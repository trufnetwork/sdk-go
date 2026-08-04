package contractsapi

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/forecast"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ═══════════════════════════════════════════════════════════════
// MARKET FORECASTING
// ═══════════════════════════════════════════════════════════════

// GetMarketForecast collapses a market's bucket books into the single value they
// imply.
//
// A prediction market prices ranges: each bucket is its own query_id with its
// own binary book. This reads every bucket's FULL YES and NO ladders and its
// bounds, then returns the one number the books collectively imply plus the band
// around it. See the forecast package for the algorithm.
//
// The YES and NO books are both fetched because the forecast consolidates them:
// on this venue a resting BUY NO at p is hittable by a BUY YES at 100-p (mint
// match), so NO liquidity is executable YES liquidity and ignoring it would
// discard real quotes. That costs two order-book reads per bucket.
//
// The buckets are expected to tile the whole line, with the bottom one open
// below and the top one open above; a layout that does not is still estimated,
// with the problem reported in the forecast's Warnings rather than returned as
// an error.
//
// Returns a nil forecast, and no error, when no bucket has a usable quote.
func (o *OrderBook) GetMarketForecast(ctx context.Context, input types.GetMarketForecastInput) (
	*forecast.MarketForecast, error,
) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	books := make([]forecast.BucketDepth, 0, len(input.QueryIDs))
	identity := ""
	for i, queryID := range input.QueryIDs {
		info, err := o.GetMarketInfo(ctx, types.GetMarketInfoInput{QueryID: queryID})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if len(info.QueryComponents) == 0 {
			return nil, errors.Errorf(
				"market %d has no query_components, so its bucket bounds cannot be derived",
				queryID)
		}
		market, err := DecodeMarketData(info.QueryComponents)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode market %d", queryID)
		}

		// Buckets of one market differ only in their strike: they share a data
		// provider, a stream, a settlement time and the query time they observe.
		// Forecasting across two events would normalise unrelated probabilities
		// into one distribution and return a confident number about nothing, so
		// it is rejected rather than warned about.
		thisIdentity := marketIdentity(market, info.SettleTime)
		if i == 0 {
			identity = thisIdentity
		} else if thisIdentity != identity {
			return nil, errors.Errorf(
				"market %d belongs to a different event than the first bucket: "+
					"(data_provider, stream_id, settle_time, timestamp, frozen_at) is %s against %s. "+
					"One forecast covers the buckets of ONE market.",
				queryID, thisIdentity, identity)
		}

		lower, upper, err := BucketBoundsFromMarketData(market)
		if err != nil {
			return nil, errors.Wrapf(err, "market %d", queryID)
		}

		yesBids, yesAsks, err := o.orderBookLadders(ctx, queryID, true)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		noBids, noAsks, err := o.orderBookLadders(ctx, queryID, false)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		id := queryID
		books = append(books, forecast.BucketDepth{
			Lower:   lower,
			Upper:   upper,
			YesBids: yesBids,
			YesAsks: yesAsks,
			NoBids:  noBids,
			NoAsks:  noAsks,
			QueryID: &id,
		})
	}

	return forecastFromBooks(books), nil
}

// forecastFromBooks orders a market's bucket books and forecasts from them,
// prepending any problem with the bucket layout to the warnings.
//
// Split out from GetMarketForecast so the ordering and layout checks can be
// tested without a node: everything above this point is network I/O, everything
// below is a pure function of the books.
func forecastFromBooks(books []forecast.BucketDepth) *forecast.MarketForecast {
	if len(books) == 0 {
		return nil
	}

	// Open below sorts first, which is where that bucket belongs.
	sort.SliceStable(books, func(i, j int) bool {
		if books[i].Lower == nil {
			return books[j].Lower != nil
		}
		if books[j].Lower == nil {
			return false
		}
		return *books[i].Lower < *books[j].Lower
	})

	var layout []string
	if books[0].Lower != nil {
		layout = append(layout, "lowest bucket is not open below")
	}
	if books[len(books)-1].Upper != nil {
		layout = append(layout, "highest bucket is not open above")
	}
	breaks := 0
	for i := 0; i+1 < len(books); i++ {
		if !sameBound(books[i].Upper, books[i+1].Lower) {
			breaks++
		}
	}
	if breaks > 0 {
		layout = append(layout, fmt.Sprintf("%d gap(s) or overlap(s) between bucket bounds", breaks))
	}

	result := forecast.FromDepth(books)
	if result == nil {
		return nil
	}
	result.Warnings = append(layout, result.Warnings...)
	return result
}

// marketIdentity is the part of a market's definition that every bucket of one
// market shares. Buckets differ only in their strike, so anything outside the
// strike identifies the event itself.
//
// The query's own timestamp and frozen_at are included, not just the settlement
// time: two markets can settle at the same moment while observing the stream at
// different points, and merging those would be exactly the mistake this guards
// against. On mainnet today the settle time and the query timestamp happen to
// coincide, which is why this costs nothing in practice.
//
// Kept as a pure function so the comparison can be tested without a node.
func marketIdentity(market *MarketData, settleTime int64) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s",
		market.DataProvider, market.StreamID, settleTime,
		formatOptionalInt64(market.Timestamp), formatOptionalInt64(market.FrozenAt))
}

// formatOptionalInt64 renders a nullable INT8 for the identity string. "null"
// is a distinct value: frozen_at uses it to mean "latest".
func formatOptionalInt64(v *int64) string {
	if v == nil {
		return "null"
	}
	return strconv.FormatInt(*v, 10)
}

// sameBound reports whether two optional bounds describe the same edge. Two
// open ends match; an open end never matches a number.
func sameBound(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// orderBookLadders returns one outcome's resting book, split into bid and ask
// ladders.
//
// GetOrderBook marks bids with a NEGATIVE price and asks with a positive one;
// price 0 means shares held with no resting order, which is not part of the
// book. Prices come back to the forecast as the positive 1-99 cent convention it
// expects.
func (o *OrderBook) orderBookLadders(ctx context.Context, queryID int, outcome bool) (
	bids, asks []forecast.BookLevel, err error,
) {
	entries, err := o.GetOrderBook(ctx, types.GetOrderBookInput{QueryID: queryID, Outcome: outcome})
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	bids, asks = laddersFromEntries(entries)
	return bids, asks, nil
}

// laddersFromEntries splits raw order book rows into bid and ask ladders.
//
// The sign convention is the whole point: a NEGATIVE price is a bid, a positive
// one is an ask, and 0 is a holding rather than a resting order. Get this
// backwards and every one-sided bucket in the forecast inverts silently, so it
// is kept as a pure function that can be tested without a node.
func laddersFromEntries(entries []types.OrderBookEntry) (bids, asks []forecast.BookLevel) {
	for _, entry := range entries {
		switch {
		case entry.Price < 0:
			bids = append(bids, forecast.BookLevel{
				Price: float64(-entry.Price),
				Size:  float64(entry.Amount),
			})
		case entry.Price > 0:
			asks = append(asks, forecast.BookLevel{
				Price: float64(entry.Price),
				Size:  float64(entry.Amount),
			})
		}
	}
	return bids, asks
}
