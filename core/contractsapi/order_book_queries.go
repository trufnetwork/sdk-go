package contractsapi

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/forecast"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ═══════════════════════════════════════════════════════════════
// QUERY OPERATIONS
// ═══════════════════════════════════════════════════════════════

// GetOrderBook retrieves all buy/sell orders for a market outcome
// Maps to: get_order_book($query_id, $outcome)
// Migration: 038-order-book-queries.sql:18-73
//
// Returns: All buy and sell orders (excludes holdings with price=0)
// Ordering: By price (best first), then by last_updated (FIFO within price level)
func (o *OrderBook) GetOrderBook(ctx context.Context, input types.GetOrderBookInput) ([]types.OrderBookEntry, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.QueryID, input.Outcome}
	result, err := o.call(ctx, "get_order_book", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var entries []types.OrderBookEntry
	for _, row := range result.Values {
		entry, err := parseOrderBookEntryRow(row)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetUserPositions retrieves caller's portfolio across all markets
// Maps to: get_user_positions()
// Migration: 038-order-book-queries.sql:84-138
//
// Returns: All positions (holdings + orders) across all markets for caller
// Uses @caller from client signer
func (o *OrderBook) GetUserPositions(ctx context.Context) ([]types.UserPosition, error) {
	args := []any{} // No arguments, uses @caller
	result, err := o.call(ctx, "get_user_positions", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var positions []types.UserPosition
	for _, row := range result.Values {
		position, err := parseUserPositionRow(row)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		positions = append(positions, position)
	}

	return positions, nil
}

// GetMarketDepth returns aggregated volume per price level
// Maps to: get_market_depth($query_id, $outcome)
// Migration: 038-order-book-queries.sql:149-208
//
// Returns: Aggregated volume per price level (combines all orders at same price)
func (o *OrderBook) GetMarketDepth(ctx context.Context, input types.GetMarketDepthInput) ([]types.DepthLevel, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.QueryID, input.Outcome}
	result, err := o.call(ctx, "get_market_depth", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var levels []types.DepthLevel
	for _, row := range result.Values {
		level, err := parseDepthLevelRow(row)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		levels = append(levels, level)
	}

	return levels, nil
}

// GetFullMarketDepth returns aggregated volume per price level for both outcomes
// Maps to: get_full_market_depth($query_id)
// Migration: 038-order-book-queries.sql:215-282
//
// Same aggregation as GetMarketDepth, for the whole market instead of one
// outcome, with each level tagged by the outcome it rests on. Rows arrive YES
// first then NO, price ascending within each.
//
// One statement means one snapshot. Anything that compares the two outcomes to
// each other wants this rather than two GetMarketDepth calls, because between
// two calls an order can land on one side and not the other.
func (o *OrderBook) GetFullMarketDepth(
	ctx context.Context, input types.GetFullMarketDepthInput,
) ([]types.FullDepthLevel, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.QueryID}
	result, err := o.call(ctx, "get_full_market_depth", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var levels []types.FullDepthLevel
	for _, row := range result.Values {
		level, err := parseFullDepthLevelRow(row)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		levels = append(levels, level)
	}

	return levels, nil
}

// GetBestPrices returns current bid/ask spread
// Maps to: get_best_prices($query_id, $outcome)
// Migration: 038-order-book-queries.sql:219-268
//
// Returns: Current bid/ask spread
// - BestBid: Highest buy price, nil if no bids
// - BestAsk: Lowest sell price, nil if no asks
// - Spread: BestAsk - BestBid, nil if either side empty
func (o *OrderBook) GetBestPrices(ctx context.Context, input types.GetBestPricesInput) (*types.BestPrices, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.QueryID, input.Outcome}
	result, err := o.call(ctx, "get_best_prices", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(result.Values) == 0 {
		// Empty order book - return zero-value BestPrices (all fields nil)
		return &types.BestPrices{}, nil
	}

	row := result.Values[0]
	prices, err := parseBestPricesRow(row)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return prices, nil
}

// GetMarketActivity returns filled volume and participation for one order book
// over a time window, both ends inclusive.
// Maps to: get_market_activity($query_id, $from_ts, $to_ts)
// Migration: 058-order-book-market-activity.sql
//
// The node action holds the one definition of volume, so every SDK reads the
// same figures. Volume is in cents of the market's own collateral, named by
// Bridge. Never sum it across bridges: base units differ between collateral
// tokens by orders of magnitude, and nothing here converts between them. Unique
// traders are comparable across bridges.
//
// The node trims ob_order_events once the events are indexed, so it holds a
// rolling window. CoverageFromBlock is the earliest block it still holds, and
// CoverageComplete is false when the market was created before that block. A
// zero with CoverageComplete false is truncation, not inactivity.
//
// A market that does not exist is an error. Requires a node carrying
// get_market_activity.
func (o *OrderBook) GetMarketActivity(
	ctx context.Context, input types.GetMarketActivityInput,
) (*types.MarketActivity, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	result, err := o.call(ctx, "get_market_activity", marketActivityArgs(input))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return marketActivityFromResult(input.QueryID, result)
}

// marketActivityArgs orders the input as get_market_activity takes it.
func marketActivityArgs(input types.GetMarketActivityInput) []any {
	return []any{input.QueryID, input.FromTs, input.ToTs}
}

// marketActivityFromResult reads get_market_activity's result. A market that
// exists returns one row even when nothing traded in the window; a market that
// does not exist returns none.
func marketActivityFromResult(queryID int, result *kwiltypes.QueryResult) (*types.MarketActivity, error) {
	if result == nil {
		return nil, fmt.Errorf("get_market_activity for market %d returned no result", queryID)
	}

	switch len(result.Values) {
	case 0:
		return nil, fmt.Errorf("market %d not found", queryID)
	case 1:
		return parseMarketActivityRow(result.Values[0])
	default:
		return nil, fmt.Errorf("get_market_activity for market %d returned %d rows, expected 1",
			queryID, len(result.Values))
	}
}

// GetConsolidatedOrderBook returns one outcome's book with the opposite
// outcome's quotes folded in.
//
// Reads the whole market with one get_full_market_depth call and folds the two
// sides together here. GetMarketDepth returns a single outcome's ladder, but a
// binary market's two books are two views of one position and the matching
// engine fills across them. A resting SELL NO at 93c is a standing BID for YES
// at 7c: a trader hits it by SELLING YES, both sides sell, and the chain burns
// the share pair. Reading only the YES book makes that quote invisible and the
// market look thinner than it is.
//
// So, in the YES frame:
//
//	consolidated bids = YES bids + (100 - p for every NO ask)
//	consolidated asks = YES asks + (100 - p for every NO bid)
//
// The sides swap: a NO ask arrives as a YES bid. Hitting it means both parties
// sell and the share pair burns; hitting a consolidated ask means both buy and a
// pair mints.
//
// The result is NOT a sweepable ladder. Mint and burn matches fire only when the
// two prices sum to exactly 100, while a direct same-outcome match crosses. So
// one order fills every native level past its limit plus exactly ONE inverse
// level, and walking these levels the way you would walk a regular ladder quotes
// fills the chain will not produce. Each level keeps Native and Inverse
// separately so a caller can price that correctly.
//
// Input.Outcome frames the prices. The NO-framed book is the YES-framed book
// reflected, so one call answers either tab.
//
// Both sides come from one read, so they are one snapshot of the chain and
// IsCrossed describes a state the book was really in. This used to take two
// get_market_depth calls, where an order landing between them could make the
// stitched ladder read as crossed when neither height was.
//
// Requires a node carrying get_full_market_depth.
func (o *OrderBook) GetConsolidatedOrderBook(
	ctx context.Context, input types.GetConsolidatedOrderBookInput,
) (*forecast.ConsolidatedOrderBook, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	depth, err := o.GetFullMarketDepth(ctx, types.GetFullMarketDepthInput{QueryID: input.QueryID})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	native, opposite := splitFullDepth(depth, input.Outcome)

	bids := forecast.ConsolidateSide(depthBids(native), depthAsks(opposite), forecast.BidSide)
	asks := forecast.ConsolidateSide(depthAsks(native), depthBids(opposite), forecast.AskSide)

	return &forecast.ConsolidatedOrderBook{
		QueryID:   input.QueryID,
		Outcome:   input.Outcome,
		Bids:      bids,
		Asks:      asks,
		IsCrossed: forecast.Crossed(bids, asks),
	}, nil
}

// splitFullDepth separates a whole-market depth read into the ladder for the
// requested outcome and the ladder for the other one.
//
// The outcome tag is the only thing that distinguishes the two here: prices are
// still in each outcome's own frame, and inverting the opposite side into this
// frame is ConsolidateSide's job.
func splitFullDepth(depth []types.FullDepthLevel, outcome bool) (native, opposite []types.DepthLevel) {
	for _, full := range depth {
		level := types.DepthLevel{
			Price: full.Price, BuyVolume: full.BuyVolume, SellVolume: full.SellVolume,
		}
		if full.Outcome == outcome {
			native = append(native, level)
		} else {
			opposite = append(opposite, level)
		}
	}
	return native, opposite
}

// depthBids returns the buy orders in a depth ladder, as levels.
func depthBids(depth []types.DepthLevel) []forecast.BookLevel {
	levels := make([]forecast.BookLevel, 0, len(depth))
	for _, l := range depth {
		if l.BuyVolume > 0 {
			levels = append(levels, forecast.BookLevel{
				Price: float64(l.Price), Size: float64(l.BuyVolume),
			})
		}
	}
	return levels
}

// depthAsks returns the sell orders in a depth ladder, as levels.
func depthAsks(depth []types.DepthLevel) []forecast.BookLevel {
	levels := make([]forecast.BookLevel, 0, len(depth))
	for _, l := range depth {
		if l.SellVolume > 0 {
			levels = append(levels, forecast.BookLevel{
				Price: float64(l.Price), Size: float64(l.SellVolume),
			})
		}
	}
	return levels
}

// GetUserCollateral returns caller's total locked collateral value
// Maps to: get_user_collateral()
// Migration: 038-order-book-queries.sql:279-359
//
// Returns: Total locked collateral broken down by type
// - TotalLocked: total locked collateral in wei (NUMERIC(78,0) as string)
// - BuyOrdersLocked: collateral locked in buy orders (NUMERIC(78,0) as string)
// - SharesValue: value of shares at $1.00 per share (NUMERIC(78,0) as string)
//
// Uses @caller from client signer
func (o *OrderBook) GetUserCollateral(ctx context.Context) (*types.UserCollateral, error) {
	args := []any{} // No arguments, uses @caller
	result, err := o.call(ctx, "get_user_collateral", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(result.Values) == 0 {
		// No positions, return zeros
		return &types.UserCollateral{
			TotalLocked:     "0",
			BuyOrdersLocked: "0",
			SharesValue:     "0",
		}, nil
	}

	row := result.Values[0]
	collateral, err := parseUserCollateralRow(row)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return collateral, nil
}

// GetPositionsByWallet retrieves a wallet's portfolio by address
// Maps to: get_positions_by_wallet($wallet_address)
// Migration: 051-order-book-portfolio-by-wallet.sql
//
// Unlike GetUserPositions (which reads @caller), this reads the wallet passed in,
// so an owner can read an agent wallet's (MAA) positions without holding its key.
func (o *OrderBook) GetPositionsByWallet(ctx context.Context, input types.GetPositionsByWalletInput) ([]types.UserPosition, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.WalletHex}
	result, err := o.call(ctx, "get_positions_by_wallet", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var positions []types.UserPosition
	for _, row := range result.Values {
		position, err := parseUserPositionRow(row)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		positions = append(positions, position)
	}

	return positions, nil
}

// GetCollateralByWallet returns a wallet's total locked collateral by address
// Maps to: get_collateral_by_wallet($wallet_address, $bridge)
// Migration: 051-order-book-portfolio-by-wallet.sql
//
// Unlike GetUserCollateral (which reads @caller), this reads the wallet passed in.
// bridge is required (per-bridge decimals).
func (o *OrderBook) GetCollateralByWallet(ctx context.Context, input types.GetCollateralByWalletInput) (*types.UserCollateral, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.WalletHex, input.Bridge}
	result, err := o.call(ctx, "get_collateral_by_wallet", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(result.Values) == 0 {
		// No positions, return zeros
		return &types.UserCollateral{
			TotalLocked:     "0",
			BuyOrdersLocked: "0",
			SharesValue:     "0",
		}, nil
	}

	row := result.Values[0]
	collateral, err := parseUserCollateralRow(row)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return collateral, nil
}

// ═══════════════════════════════════════════════════════════════
// PARSING HELPERS
// ═══════════════════════════════════════════════════════════════

// parseOrderBookEntryRow parses a row from get_order_book
// Row format: participant_id, price, amount, last_updated, wallet_address
func parseOrderBookEntryRow(row []any) (types.OrderBookEntry, error) {
	if len(row) < 5 {
		return types.OrderBookEntry{}, fmt.Errorf("invalid row: expected 5 columns, got %d", len(row))
	}

	entry := types.OrderBookEntry{}

	// Column 0: participant_id (INT)
	if err := extractIntColumn(row[0], &entry.ParticipantID, 0, "participant_id"); err != nil {
		return entry, err
	}

	// Column 1: price (INT)
	if err := extractIntColumn(row[1], &entry.Price, 1, "price"); err != nil {
		return entry, err
	}

	// Column 2: amount (INT8)
	if err := extractInt64Column(row[2], &entry.Amount, 2, "amount"); err != nil {
		return entry, err
	}

	// Column 3: last_updated (INT8)
	if err := extractInt64Column(row[3], &entry.LastUpdated, 3, "last_updated"); err != nil {
		return entry, err
	}

	// Column 4: wallet_address (TEXT/BYTEA)
	if err := extractBytesColumn(row[4], &entry.WalletAddress, 4, "wallet_address"); err != nil {
		return entry, err
	}

	return entry, nil
}

// parseUserPositionRow parses a row from get_user_positions
// Row format: query_id, outcome, price, amount, position_type
func parseUserPositionRow(row []any) (types.UserPosition, error) {
	if len(row) < 5 {
		return types.UserPosition{}, fmt.Errorf("invalid row: expected 5 columns, got %d", len(row))
	}

	position := types.UserPosition{}

	// Column 0: query_id (INT)
	if err := extractIntColumn(row[0], &position.QueryID, 0, "query_id"); err != nil {
		return position, err
	}

	// Column 1: outcome (BOOL)
	if err := extractBoolColumn(row[1], &position.Outcome, 1, "outcome"); err != nil {
		return position, err
	}

	// Column 2: price (INT)
	if err := extractIntColumn(row[2], &position.Price, 2, "price"); err != nil {
		return position, err
	}

	// Column 3: amount (INT8)
	if err := extractInt64Column(row[3], &position.Amount, 3, "amount"); err != nil {
		return position, err
	}

	// Column 4: position_type (TEXT)
	if err := extractStringColumn(row[4], &position.PositionType, 4, "position_type"); err != nil {
		return position, err
	}

	return position, nil
}

// parseDepthLevelRow parses a row from get_market_depth
// Row format: price, buy_volume, sell_volume
func parseDepthLevelRow(row []any) (types.DepthLevel, error) {
	if len(row) < 3 {
		return types.DepthLevel{}, fmt.Errorf("invalid row: expected 3 columns, got %d", len(row))
	}

	level := types.DepthLevel{}

	// Column 0: price (INT)
	if err := extractIntColumn(row[0], &level.Price, 0, "price"); err != nil {
		return level, err
	}

	// Column 1: buy_volume (INT8)
	if err := extractInt64Column(row[1], &level.BuyVolume, 1, "buy_volume"); err != nil {
		return level, err
	}

	// Column 2: sell_volume (INT8)
	if err := extractInt64Column(row[2], &level.SellVolume, 2, "sell_volume"); err != nil {
		return level, err
	}

	return level, nil
}

// parseFullDepthLevelRow parses a row from get_full_market_depth
// Row format: outcome, price, buy_volume, sell_volume
func parseFullDepthLevelRow(row []any) (types.FullDepthLevel, error) {
	if len(row) < 4 {
		return types.FullDepthLevel{}, fmt.Errorf("invalid row: expected 4 columns, got %d", len(row))
	}

	level := types.FullDepthLevel{}

	// Column 0: outcome (BOOL)
	if err := extractBoolColumn(row[0], &level.Outcome, 0, "outcome"); err != nil {
		return level, err
	}

	// Column 1: price (INT)
	if err := extractIntColumn(row[1], &level.Price, 1, "price"); err != nil {
		return level, err
	}

	// Column 2: buy_volume (INT8)
	if err := extractInt64Column(row[2], &level.BuyVolume, 2, "buy_volume"); err != nil {
		return level, err
	}

	// Column 3: sell_volume (INT8)
	if err := extractInt64Column(row[3], &level.SellVolume, 3, "sell_volume"); err != nil {
		return level, err
	}

	return level, nil
}

// parseMarketActivityRow parses a row from get_market_activity
// Row format: bridge, volume_cents, direct_cents, mint_burn_cents,
// unique_traders, fill_count, direct_fill_count, shares_traded,
// first_event_ts, last_event_ts, coverage_from_block, coverage_complete
func parseMarketActivityRow(row []any) (*types.MarketActivity, error) {
	if len(row) < 12 {
		return nil, fmt.Errorf("invalid row: expected 12 columns, got %d", len(row))
	}

	activity := &types.MarketActivity{}

	// Column 0: bridge (TEXT)
	if err := extractStringColumn(row[0], &activity.Bridge, 0, "bridge"); err != nil {
		return nil, err
	}

	// Column 1: volume_cents (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[1], &activity.VolumeCents, 1, "volume_cents"); err != nil {
		return nil, err
	}

	// Column 2: direct_cents (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[2], &activity.DirectCents, 2, "direct_cents"); err != nil {
		return nil, err
	}

	// Column 3: mint_burn_cents (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[3], &activity.MintBurnCents, 3, "mint_burn_cents"); err != nil {
		return nil, err
	}

	// Column 4: unique_traders (INT)
	if err := extractIntColumn(row[4], &activity.UniqueTraders, 4, "unique_traders"); err != nil {
		return nil, err
	}

	// Column 5: fill_count (INT)
	if err := extractIntColumn(row[5], &activity.FillCount, 5, "fill_count"); err != nil {
		return nil, err
	}

	// Column 6: direct_fill_count (INT)
	if err := extractIntColumn(row[6], &activity.DirectFillCount, 6, "direct_fill_count"); err != nil {
		return nil, err
	}

	// Column 7: shares_traded (INT8)
	if err := extractInt64Column(row[7], &activity.SharesTraded, 7, "shares_traded"); err != nil {
		return nil, err
	}

	// Column 8: first_event_ts (INT8, nullable)
	if row[8] != nil {
		var ts int64
		if err := extractInt64Column(row[8], &ts, 8, "first_event_ts"); err != nil {
			return nil, err
		}
		activity.FirstEventTs = &ts
	}

	// Column 9: last_event_ts (INT8, nullable)
	if row[9] != nil {
		var ts int64
		if err := extractInt64Column(row[9], &ts, 9, "last_event_ts"); err != nil {
			return nil, err
		}
		activity.LastEventTs = &ts
	}

	// Column 10: coverage_from_block (INT8). The action reports 0 when the node
	// holds no order events.
	if err := extractInt64Column(row[10], &activity.CoverageFromBlock, 10, "coverage_from_block"); err != nil {
		return nil, err
	}

	// Column 11: coverage_complete (BOOL)
	if err := extractBoolColumn(row[11], &activity.CoverageComplete, 11, "coverage_complete"); err != nil {
		return nil, err
	}

	return activity, nil
}

// parseBestPricesRow parses a row from get_best_prices
// Row format: best_bid, best_ask, spread
func parseBestPricesRow(row []any) (*types.BestPrices, error) {
	if len(row) < 3 {
		return nil, fmt.Errorf("invalid row: expected 3 columns, got %d", len(row))
	}

	prices := &types.BestPrices{}

	// Column 0: best_bid (INT, nullable)
	if row[0] != nil {
		var bid int
		if err := extractIntColumn(row[0], &bid, 0, "best_bid"); err != nil {
			return nil, err
		}
		prices.BestBid = &bid
	}

	// Column 1: best_ask (INT, nullable)
	if row[1] != nil {
		var ask int
		if err := extractIntColumn(row[1], &ask, 1, "best_ask"); err != nil {
			return nil, err
		}
		prices.BestAsk = &ask
	}

	// Column 2: spread (INT, nullable)
	if row[2] != nil {
		var spread int
		if err := extractIntColumn(row[2], &spread, 2, "spread"); err != nil {
			return nil, err
		}
		prices.Spread = &spread
	}

	return prices, nil
}

// parseUserCollateralRow parses a row from get_user_collateral
// Row format: total_locked, buy_orders_locked, shares_value
func parseUserCollateralRow(row []any) (*types.UserCollateral, error) {
	if len(row) < 3 {
		return nil, fmt.Errorf("invalid row: expected 3 columns, got %d", len(row))
	}

	collateral := &types.UserCollateral{}

	// All three columns are NUMERIC(78,0) returned as string

	// Column 0: total_locked (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[0], &collateral.TotalLocked, 0, "total_locked"); err != nil {
		return nil, err
	}

	// Column 1: buy_orders_locked (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[1], &collateral.BuyOrdersLocked, 1, "buy_orders_locked"); err != nil {
		return nil, err
	}

	// Column 2: shares_value (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[2], &collateral.SharesValue, 2, "shares_value"); err != nil {
		return nil, err
	}

	return collateral, nil
}
