package contractsapi

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	kwilClientType "github.com/trufnetwork/kwil-db/core/client/types"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ═══════════════════════════════════════════════════════════════
// MARKET OPERATIONS
// ═══════════════════════════════════════════════════════════════

// CreateMarket creates a new prediction market
// Maps to: create_market($bridge, $query_components, $settle_time, $max_spread, $min_order_size)
// Migration: 032-order-book-actions.sql:85-226
func (o *OrderBook) CreateMarket(ctx context.Context, input types.CreateMarketInput,
	opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	if err := input.Validate(); err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return o.execute(ctx, "create_market", [][]any{{
		input.Bridge,
		input.QueryComponents,
		input.SettleTime,
		input.MaxSpread,
		input.MinOrderSize,
	}}, opts...)
}

// GetMarketInfo retrieves market details by ID
// Maps to: get_market_info($query_id)
// Migration: 032-order-book-actions.sql:157-185
func (o *OrderBook) GetMarketInfo(ctx context.Context, input types.GetMarketInfoInput) (*types.MarketInfo, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.QueryID}
	result, err := o.call(ctx, "get_market_info", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(result.Values) == 0 {
		return nil, fmt.Errorf("market not found: query_id=%d", input.QueryID)
	}

	row := result.Values[0]
	market, err := parseMarketInfoRow(row, input.QueryID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return market, nil
}

// GetMarketByHash retrieves market details by query hash
// Maps to: get_market_by_hash($query_hash)
// Migration: 032-order-book-actions.sql:283-311
// Note: Returns fewer columns than get_market_info (no query_components or bridge)
func (o *OrderBook) GetMarketByHash(ctx context.Context, input types.GetMarketByHashInput) (*types.MarketInfo, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.QueryHash}
	result, err := o.call(ctx, "get_market_by_hash", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(result.Values) == 0 {
		return nil, fmt.Errorf("market not found for given hash")
	}

	row := result.Values[0]

	// get_market_by_hash returns 9 columns: id, settle_time, settled, winning_outcome, settled_at, max_spread, min_order_size, created_at, creator
	market, err := parseMarketByHashRow(row)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return market, nil
}

// ListMarkets returns paginated list of markets with optional filtering
// Maps to: list_markets($settled_filter, $limit_val, $offset_val)
// Migration: 032-order-book-actions.sql:242-284
func (o *OrderBook) ListMarkets(ctx context.Context, input types.ListMarketsInput) ([]types.MarketSummary, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	var settledFilterArg any
	if input.SettledFilter != nil {
		settledFilterArg = *input.SettledFilter
	}

	var limitArg any
	if input.Limit != nil {
		limitArg = *input.Limit
	}

	var offsetArg any
	if input.Offset != nil {
		offsetArg = *input.Offset
	}

	args := []any{settledFilterArg, limitArg, offsetArg}
	result, err := o.call(ctx, "list_markets", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var markets []types.MarketSummary
	for _, row := range result.Values {
		market, err := parseMarketSummaryRow(row)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		markets = append(markets, market)
	}

	return markets, nil
}

// MarketExists checks if market exists by hash (lightweight)
// Maps to: market_exists($query_hash)
// Migration: 032-order-book-actions.sql:296-307
func (o *OrderBook) MarketExists(ctx context.Context, input types.MarketExistsInput) (bool, error) {
	if err := input.Validate(); err != nil {
		return false, errors.WithStack(err)
	}

	args := []any{input.QueryHash}
	result, err := o.call(ctx, "market_exists", args)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if len(result.Values) == 0 {
		return false, nil
	}

	row := result.Values[0]
	if len(row) < 1 {
		return false, fmt.Errorf("invalid result: expected 1 column, got %d", len(row))
	}

	var exists bool
	if err := extractBoolColumn(row[0], &exists, 0, "market_exists"); err != nil {
		return false, errors.WithStack(err)
	}

	return exists, nil
}

// ValidateMarketCollateral checks binary token parity and vault balance
// Maps to: validate_market_collateral($query_id)
// Migration: 037-order-book-validation.sql:24-119
func (o *OrderBook) ValidateMarketCollateral(ctx context.Context, input types.ValidateMarketCollateralInput) (*types.MarketValidation, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.QueryID}
	result, err := o.call(ctx, "validate_market_collateral", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(result.Values) == 0 {
		return nil, fmt.Errorf("validation data not found for query_id=%d", input.QueryID)
	}

	row := result.Values[0]
	validation, err := parseMarketValidationRow(row)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return validation, nil
}

// ═══════════════════════════════════════════════════════════════
// PARSING HELPERS
// ═══════════════════════════════════════════════════════════════

// parseMarketInfoRow parses a row from get_market_info
// Row format: hash, query_components, bridge, settle_time, settled, winning_outcome, settled_at, max_spread, min_order_size, created_at, creator
func parseMarketInfoRow(row []any, marketID int) (*types.MarketInfo, error) {
	if len(row) < 11 {
		return nil, fmt.Errorf("invalid row: expected 11 columns, got %d", len(row))
	}

	market := &types.MarketInfo{
		ID: marketID,
	}

	// Column 0: hash (BYTEA)
	if err := extractBytesColumn(row[0], &market.Hash, 0, "hash"); err != nil {
		return nil, err
	}

	// Column 1: query_components (BYTEA)
	if err := extractBytesColumn(row[1], &market.QueryComponents, 1, "query_components"); err != nil {
		return nil, err
	}

	// Column 2: bridge (TEXT)
	if err := extractStringColumn(row[2], &market.Bridge, 2, "bridge"); err != nil {
		return nil, err
	}

	// Column 3: settle_time (INT8)
	if err := extractInt64Column(row[3], &market.SettleTime, 3, "settle_time"); err != nil {
		return nil, err
	}

	// Column 4: settled (BOOL)
	if err := extractBoolColumn(row[4], &market.Settled, 4, "settled"); err != nil {
		return nil, err
	}

	// Column 5: winning_outcome (BOOL, nullable)
	if row[5] != nil {
		var outcome bool
		if err := extractBoolColumn(row[5], &outcome, 5, "winning_outcome"); err != nil {
			return nil, err
		}
		market.WinningOutcome = &outcome
	}

	// Column 6: settled_at (INT8, nullable)
	if row[6] != nil {
		var settledAt int64
		if err := extractInt64Column(row[6], &settledAt, 6, "settled_at"); err != nil {
			return nil, err
		}
		market.SettledAt = &settledAt
	}

	// Column 7: max_spread (INT)
	if err := extractIntColumn(row[7], &market.MaxSpread, 7, "max_spread"); err != nil {
		return nil, err
	}

	// Column 8: min_order_size (INT8)
	if err := extractInt64Column(row[8], &market.MinOrderSize, 8, "min_order_size"); err != nil {
		return nil, err
	}

	// Column 9: created_at (INT8)
	if err := extractInt64Column(row[9], &market.CreatedAt, 9, "created_at"); err != nil {
		return nil, err
	}

	// Column 10: creator (BYTEA)
	if err := extractBytesColumn(row[10], &market.Creator, 10, "creator"); err != nil {
		return nil, err
	}

	return market, nil
}

// parseMarketByHashRow parses a row from get_market_by_hash
// Row format: id, settle_time, settled, winning_outcome, settled_at, max_spread, min_order_size, created_at, creator
// Note: get_market_by_hash does NOT return query_components or bridge
func parseMarketByHashRow(row []any) (*types.MarketInfo, error) {
	if len(row) < 9 {
		return nil, fmt.Errorf("invalid row: expected 9 columns, got %d", len(row))
	}

	market := &types.MarketInfo{}

	// Column 0: id (INT)
	if err := extractIntColumn(row[0], &market.ID, 0, "id"); err != nil {
		return nil, err
	}

	// Column 1: settle_time (INT8)
	if err := extractInt64Column(row[1], &market.SettleTime, 1, "settle_time"); err != nil {
		return nil, err
	}

	// Column 2: settled (BOOL)
	if err := extractBoolColumn(row[2], &market.Settled, 2, "settled"); err != nil {
		return nil, err
	}

	// Column 3: winning_outcome (BOOL, nullable)
	if row[3] != nil {
		var outcome bool
		if err := extractBoolColumn(row[3], &outcome, 3, "winning_outcome"); err != nil {
			return nil, err
		}
		market.WinningOutcome = &outcome
	}

	// Column 4: settled_at (INT8, nullable)
	if row[4] != nil {
		var settledAt int64
		if err := extractInt64Column(row[4], &settledAt, 4, "settled_at"); err != nil {
			return nil, err
		}
		market.SettledAt = &settledAt
	}

	// Column 5: max_spread (INT)
	if err := extractIntColumn(row[5], &market.MaxSpread, 5, "max_spread"); err != nil {
		return nil, err
	}

	// Column 6: min_order_size (INT8)
	if err := extractInt64Column(row[6], &market.MinOrderSize, 6, "min_order_size"); err != nil {
		return nil, err
	}

	// Column 7: created_at (INT8)
	if err := extractInt64Column(row[7], &market.CreatedAt, 7, "created_at"); err != nil {
		return nil, err
	}

	// Column 8: creator (BYTEA)
	if err := extractBytesColumn(row[8], &market.Creator, 8, "creator"); err != nil {
		return nil, err
	}

	return market, nil
}

// parseMarketSummaryRow parses a row from list_markets
// Row format: id, hash, settle_time, settled, winning_outcome, max_spread, min_order_size, created_at
func parseMarketSummaryRow(row []any) (types.MarketSummary, error) {
	if len(row) < 8 {
		return types.MarketSummary{}, fmt.Errorf("invalid row: expected 8 columns, got %d", len(row))
	}

	summary := types.MarketSummary{}

	// Column 0: id (INT)
	if err := extractIntColumn(row[0], &summary.ID, 0, "id"); err != nil {
		return summary, err
	}

	// Column 1: hash (BYTEA)
	if err := extractBytesColumn(row[1], &summary.Hash, 1, "hash"); err != nil {
		return summary, err
	}

	// Column 2: settle_time (INT8)
	if err := extractInt64Column(row[2], &summary.SettleTime, 2, "settle_time"); err != nil {
		return summary, err
	}

	// Column 3: settled (BOOL)
	if err := extractBoolColumn(row[3], &summary.Settled, 3, "settled"); err != nil {
		return summary, err
	}

	// Column 4: winning_outcome (BOOL, nullable)
	if row[4] != nil {
		var outcome bool
		if err := extractBoolColumn(row[4], &outcome, 4, "winning_outcome"); err != nil {
			return summary, err
		}
		summary.WinningOutcome = &outcome
	}

	// Column 5: max_spread (INT)
	if err := extractIntColumn(row[5], &summary.MaxSpread, 5, "max_spread"); err != nil {
		return summary, err
	}

	// Column 6: min_order_size (INT8)
	if err := extractInt64Column(row[6], &summary.MinOrderSize, 6, "min_order_size"); err != nil {
		return summary, err
	}

	// Column 7: created_at (INT8)
	if err := extractInt64Column(row[7], &summary.CreatedAt, 7, "created_at"); err != nil {
		return summary, err
	}

	return summary, nil
}

// parseMarketValidationRow parses a row from validate_market_collateral
// Row format: valid_token_binaries, valid_collateral, total_true, total_false, vault_balance, expected_collateral, open_buys_value
func parseMarketValidationRow(row []any) (*types.MarketValidation, error) {
	if len(row) < 7 {
		return nil, fmt.Errorf("invalid row: expected 7 columns, got %d", len(row))
	}

	validation := &types.MarketValidation{}

	// Column 0: valid_token_binaries (BOOL)
	if err := extractBoolColumn(row[0], &validation.ValidTokenBinaries, 0, "valid_token_binaries"); err != nil {
		return nil, err
	}

	// Column 1: valid_collateral (BOOL)
	if err := extractBoolColumn(row[1], &validation.ValidCollateral, 1, "valid_collateral"); err != nil {
		return nil, err
	}

	// Column 2: total_true (INT8)
	if err := extractInt64Column(row[2], &validation.TotalTrue, 2, "total_true"); err != nil {
		return nil, err
	}

	// Column 3: total_false (INT8)
	if err := extractInt64Column(row[3], &validation.TotalFalse, 3, "total_false"); err != nil {
		return nil, err
	}

	// Column 4: vault_balance (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[4], &validation.VaultBalance, 4, "vault_balance"); err != nil {
		return nil, err
	}

	// Column 5: expected_collateral (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[5], &validation.ExpectedCollateral, 5, "expected_collateral"); err != nil {
		return nil, err
	}

	// Column 6: open_buys_value (INT8)
	if err := extractInt64Column(row[6], &validation.OpenBuysValue, 6, "open_buys_value"); err != nil {
		return nil, err
	}

	return validation, nil
}
