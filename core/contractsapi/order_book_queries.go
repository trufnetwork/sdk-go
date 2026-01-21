package contractsapi

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
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
		return nil, fmt.Errorf("no price data found for query_id=%d outcome=%v", input.QueryID, input.Outcome)
	}

	row := result.Values[0]
	prices, err := parseBestPricesRow(row)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return prices, nil
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

// ═══════════════════════════════════════════════════════════════
// PARSING HELPERS
// ═══════════════════════════════════════════════════════════════

// parseOrderBookEntryRow parses a row from get_order_book
// Row format: wallet_address, price, amount, last_updated
func parseOrderBookEntryRow(row []any) (types.OrderBookEntry, error) {
	if len(row) < 4 {
		return types.OrderBookEntry{}, fmt.Errorf("invalid row: expected 4 columns, got %d", len(row))
	}

	entry := types.OrderBookEntry{}

	// Column 0: wallet_address (BYTEA)
	if err := extractBytesColumn(row[0], &entry.WalletAddress, 0, "wallet_address"); err != nil {
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

	return entry, nil
}

// parseUserPositionRow parses a row from get_user_positions
// Row format: query_id, outcome, price, amount, last_updated
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

	// Column 4: last_updated (INT8)
	if err := extractInt64Column(row[4], &position.LastUpdated, 4, "last_updated"); err != nil {
		return position, err
	}

	return position, nil
}

// parseDepthLevelRow parses a row from get_market_depth
// Row format: price, total_amount
func parseDepthLevelRow(row []any) (types.DepthLevel, error) {
	if len(row) < 2 {
		return types.DepthLevel{}, fmt.Errorf("invalid row: expected 2 columns, got %d", len(row))
	}

	level := types.DepthLevel{}

	// Column 0: price (INT)
	if err := extractIntColumn(row[0], &level.Price, 0, "price"); err != nil {
		return level, err
	}

	// Column 1: total_amount (INT8)
	if err := extractInt64Column(row[1], &level.TotalAmount, 1, "total_amount"); err != nil {
		return level, err
	}

	return level, nil
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
