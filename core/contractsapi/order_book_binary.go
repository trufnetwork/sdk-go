package contractsapi

import (
	"context"
	"fmt"

	kwilClientType "github.com/trufnetwork/kwil-db/core/client/types"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ═══════════════════════════════════════════════════════════════
// BINARY MARKET HELPER TYPES
// These input types combine query parameters with market parameters
// for convenient creation of binary prediction markets.
// ═══════════════════════════════════════════════════════════════

// CreatePriceAboveThresholdMarketInput contains all parameters for creating a
// "Will X exceed Y?" prediction market
type CreatePriceAboveThresholdMarketInput struct {
	// Query parameters (for attestation)
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // Unix timestamp to check the value at
	Threshold    string // Threshold value as decimal string (e.g., "100000.00")
	FrozenAt     *int64 // Optional: Unix timestamp to freeze the value lookup

	// Market parameters
	Bridge       string // Bridge namespace: hoodi_tt2, sepolia_bridge, or ethereum_bridge
	SettleTime   int64  // Unix timestamp when market can be settled
	MaxSpread    int    // Maximum spread for LP rewards (1-50 cents)
	MinOrderSize int64  // Minimum order size for LP rewards
}

// CreatePriceBelowThresholdMarketInput contains all parameters for creating a
// "Will X drop below Y?" prediction market
type CreatePriceBelowThresholdMarketInput struct {
	// Query parameters (for attestation)
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // Unix timestamp to check the value at
	Threshold    string // Threshold value as decimal string (e.g., "4.0")
	FrozenAt     *int64 // Optional: Unix timestamp to freeze the value lookup

	// Market parameters
	Bridge       string // Bridge namespace: hoodi_tt2, sepolia_bridge, or ethereum_bridge
	SettleTime   int64  // Unix timestamp when market can be settled
	MaxSpread    int    // Maximum spread for LP rewards (1-50 cents)
	MinOrderSize int64  // Minimum order size for LP rewards
}

// CreateValueInRangeMarketInput contains all parameters for creating a
// "Will X stay between Y and Z?" prediction market
type CreateValueInRangeMarketInput struct {
	// Query parameters (for attestation)
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // Unix timestamp to check the value at
	MinValue     string // Minimum value (inclusive) as decimal string
	MaxValue     string // Maximum value (inclusive) as decimal string
	FrozenAt     *int64 // Optional: Unix timestamp to freeze the value lookup

	// Market parameters
	Bridge       string // Bridge namespace: hoodi_tt2, sepolia_bridge, or ethereum_bridge
	SettleTime   int64  // Unix timestamp when market can be settled
	MaxSpread    int    // Maximum spread for LP rewards (1-50 cents)
	MinOrderSize int64  // Minimum order size for LP rewards
}

// CreateValueEqualsMarketInput contains all parameters for creating a
// "Will X be exactly Y?" prediction market
type CreateValueEqualsMarketInput struct {
	// Query parameters (for attestation)
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // Unix timestamp to check the value at
	TargetValue  string // Target value as decimal string (e.g., "5.25")
	Tolerance    string // Tolerance for equality check (e.g., "0.01")
	FrozenAt     *int64 // Optional: Unix timestamp to freeze the value lookup

	// Market parameters
	Bridge       string // Bridge namespace: hoodi_tt2, sepolia_bridge, or ethereum_bridge
	SettleTime   int64  // Unix timestamp when market can be settled
	MaxSpread    int    // Maximum spread for LP rewards (1-50 cents)
	MinOrderSize int64  // Minimum order size for LP rewards
}

// CreateIndexChangeInRangeMarketInput contains all parameters for creating a
// "Will X's rate of change land between Y% and Z%?" prediction market
type CreateIndexChangeInRangeMarketInput struct {
	// Query parameters (for attestation)
	DataProvider string  // 0x-prefixed Ethereum address of the data provider
	StreamID     string  // 32-character stream ID
	Timestamp    int64   // Unix timestamp to measure the change at
	BaseTime     *int64  // Optional: index base date, nil for the stream's default
	TimeInterval int64   // Seconds to look back for the comparison value (e.g. 31536000 for YoY)
	MinChange    *string // Lower bound in percent, inclusive; nil for an open tail
	MaxChange    *string // Upper bound in percent, exclusive; nil for an open tail
	FrozenAt     *int64  // Optional: Unix timestamp to freeze the value lookup

	// Market parameters
	Bridge       string // Bridge namespace, e.g. eth_usdc
	SettleTime   int64  // Unix timestamp when market can be settled
	MaxSpread    int    // Maximum spread for LP rewards (1-50 cents)
	MinOrderSize int64  // Minimum order size for LP rewards
}

// ═══════════════════════════════════════════════════════════════
// BINARY MARKET HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════

// CreatePriceAboveThresholdMarket creates a binary prediction market that settles
// TRUE if the stream value exceeds the threshold at the specified timestamp.
//
// Example: "Will BTC exceed $100,000 by December 31, 2025?"
//   - DataProvider: "0x1234...abcd" (BTC price oracle)
//   - StreamID: "stbtcusd00000000000000000000000"
//   - Timestamp: 1735689600 (Dec 31, 2025 00:00:00 UTC)
//   - Threshold: "100000"
func (o *OrderBook) CreatePriceAboveThresholdMarket(
	ctx context.Context,
	input CreatePriceAboveThresholdMarketInput,
	opts ...kwilClientType.TxOpt,
) (kwiltypes.Hash, error) {
	// Parse threshold as Decimal (NUMERIC(36,18))
	thresholdDecimal, err := kwiltypes.ParseDecimalExplicit(input.Threshold, 36, 18)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("invalid threshold: %w", err)
	}

	// Build action arguments in the order expected by the node
	// price_above_threshold($data_provider, $stream_id, $timestamp, $threshold, $frozen_at)
	args := []any{
		input.DataProvider,
		input.StreamID,
		input.Timestamp,
		thresholdDecimal,
		input.FrozenAt, // Can be nil
	}

	// Encode action arguments
	argsBytes, err := EncodeActionArgs(args)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("failed to encode action args: %w", err)
	}

	// Encode query components
	queryComponents, err := EncodeQueryComponents(
		input.DataProvider,
		input.StreamID,
		"price_above_threshold",
		argsBytes,
	)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("failed to encode query_components: %w", err)
	}

	// Create the market
	return o.CreateMarket(ctx, types.CreateMarketInput{
		Bridge:          input.Bridge,
		QueryComponents: queryComponents,
		SettleTime:      input.SettleTime,
		MaxSpread:       input.MaxSpread,
		MinOrderSize:    input.MinOrderSize,
	}, opts...)
}

// CreatePriceBelowThresholdMarket creates a binary prediction market that settles
// TRUE if the stream value is below the threshold at the specified timestamp.
//
// Example: "Will unemployment drop below 4% by Q2 2025?"
//   - DataProvider: "0x5678...efgh" (unemployment rate oracle)
//   - StreamID: "stunemprate0000000000000000000"
//   - Timestamp: 1719792000 (Jul 1, 2025 00:00:00 UTC)
//   - Threshold: "4.0"
func (o *OrderBook) CreatePriceBelowThresholdMarket(
	ctx context.Context,
	input CreatePriceBelowThresholdMarketInput,
	opts ...kwilClientType.TxOpt,
) (kwiltypes.Hash, error) {
	// Parse threshold as Decimal (NUMERIC(36,18))
	thresholdDecimal, err := kwiltypes.ParseDecimalExplicit(input.Threshold, 36, 18)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("invalid threshold: %w", err)
	}

	// Build action arguments
	// price_below_threshold($data_provider, $stream_id, $timestamp, $threshold, $frozen_at)
	args := []any{
		input.DataProvider,
		input.StreamID,
		input.Timestamp,
		thresholdDecimal,
		input.FrozenAt,
	}

	argsBytes, err := EncodeActionArgs(args)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("failed to encode action args: %w", err)
	}

	queryComponents, err := EncodeQueryComponents(
		input.DataProvider,
		input.StreamID,
		"price_below_threshold",
		argsBytes,
	)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("failed to encode query_components: %w", err)
	}

	return o.CreateMarket(ctx, types.CreateMarketInput{
		Bridge:          input.Bridge,
		QueryComponents: queryComponents,
		SettleTime:      input.SettleTime,
		MaxSpread:       input.MaxSpread,
		MinOrderSize:    input.MinOrderSize,
	}, opts...)
}

// CreateValueInRangeMarket creates a binary prediction market that settles
// TRUE if the stream value is within the specified range (inclusive) at the timestamp.
//
// Example: "Will BTC stay between $90k-$110k on settlement date?"
//   - DataProvider: "0x1234...abcd"
//   - StreamID: "stbtcusd00000000000000000000000"
//   - Timestamp: 1735689600
//   - MinValue: "90000"
//   - MaxValue: "110000"
func (o *OrderBook) CreateValueInRangeMarket(
	ctx context.Context,
	input CreateValueInRangeMarketInput,
	opts ...kwilClientType.TxOpt,
) (kwiltypes.Hash, error) {
	// Parse min/max values as Decimal (NUMERIC(36,18))
	minValueDecimal, err := kwiltypes.ParseDecimalExplicit(input.MinValue, 36, 18)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("invalid min_value: %w", err)
	}
	maxValueDecimal, err := kwiltypes.ParseDecimalExplicit(input.MaxValue, 36, 18)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("invalid max_value: %w", err)
	}

	// Build action arguments
	// value_in_range($data_provider, $stream_id, $timestamp, $min_value, $max_value, $frozen_at)
	args := []any{
		input.DataProvider,
		input.StreamID,
		input.Timestamp,
		minValueDecimal,
		maxValueDecimal,
		input.FrozenAt,
	}

	argsBytes, err := EncodeActionArgs(args)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("failed to encode action args: %w", err)
	}

	queryComponents, err := EncodeQueryComponents(
		input.DataProvider,
		input.StreamID,
		"value_in_range",
		argsBytes,
	)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("failed to encode query_components: %w", err)
	}

	return o.CreateMarket(ctx, types.CreateMarketInput{
		Bridge:          input.Bridge,
		QueryComponents: queryComponents,
		SettleTime:      input.SettleTime,
		MaxSpread:       input.MaxSpread,
		MinOrderSize:    input.MinOrderSize,
	}, opts...)
}

// CreateValueEqualsMarket creates a binary prediction market that settles
// TRUE if the stream value equals the target (within tolerance) at the timestamp.
//
// Example: "Will the Fed rate be exactly 5.25%?"
//   - DataProvider: "0xabcd...1234"
//   - StreamID: "stfedrate0000000000000000000000"
//   - Timestamp: 1735689600
//   - TargetValue: "5.25"
//   - Tolerance: "0.0" (exact match) or "0.01" (±0.01)
func (o *OrderBook) CreateValueEqualsMarket(
	ctx context.Context,
	input CreateValueEqualsMarketInput,
	opts ...kwilClientType.TxOpt,
) (kwiltypes.Hash, error) {
	// Parse target/tolerance values as Decimal (NUMERIC(36,18))
	targetValueDecimal, err := kwiltypes.ParseDecimalExplicit(input.TargetValue, 36, 18)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("invalid target_value: %w", err)
	}
	toleranceDecimal, err := kwiltypes.ParseDecimalExplicit(input.Tolerance, 36, 18)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("invalid tolerance: %w", err)
	}

	// Build action arguments
	// value_equals($data_provider, $stream_id, $timestamp, $target_value, $tolerance, $frozen_at)
	args := []any{
		input.DataProvider,
		input.StreamID,
		input.Timestamp,
		targetValueDecimal,
		toleranceDecimal,
		input.FrozenAt,
	}

	argsBytes, err := EncodeActionArgs(args)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("failed to encode action args: %w", err)
	}

	queryComponents, err := EncodeQueryComponents(
		input.DataProvider,
		input.StreamID,
		"value_equals",
		argsBytes,
	)
	if err != nil {
		return kwiltypes.Hash{}, fmt.Errorf("failed to encode query_components: %w", err)
	}

	return o.CreateMarket(ctx, types.CreateMarketInput{
		Bridge:          input.Bridge,
		QueryComponents: queryComponents,
		SettleTime:      input.SettleTime,
		MaxSpread:       input.MaxSpread,
		MinOrderSize:    input.MinOrderSize,
	}, opts...)
}

// ═══════════════════════════════════════════════════════════════
// HELPER FUNCTIONS FOR BUILDING QUERY COMPONENTS
// ═══════════════════════════════════════════════════════════════

// BuildPriceAboveThresholdQueryComponents builds query_components for a price_above_threshold query
func BuildPriceAboveThresholdQueryComponents(input types.PriceAboveThresholdInput) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Parse threshold as Decimal (NUMERIC(36,18))
	thresholdDecimal, err := kwiltypes.ParseDecimalExplicit(input.Threshold, 36, 18)
	if err != nil {
		return nil, fmt.Errorf("invalid threshold: %w", err)
	}

	args := []any{
		input.DataProvider,
		input.StreamID,
		input.Timestamp,
		thresholdDecimal,
		input.FrozenAt,
	}

	argsBytes, err := EncodeActionArgs(args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode action args: %w", err)
	}

	return EncodeQueryComponents(
		input.DataProvider,
		input.StreamID,
		"price_above_threshold",
		argsBytes,
	)
}

// BuildPriceBelowThresholdQueryComponents builds query_components for a price_below_threshold query
func BuildPriceBelowThresholdQueryComponents(input types.PriceBelowThresholdInput) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Parse threshold as Decimal (NUMERIC(36,18))
	thresholdDecimal, err := kwiltypes.ParseDecimalExplicit(input.Threshold, 36, 18)
	if err != nil {
		return nil, fmt.Errorf("invalid threshold: %w", err)
	}

	args := []any{
		input.DataProvider,
		input.StreamID,
		input.Timestamp,
		thresholdDecimal,
		input.FrozenAt,
	}

	argsBytes, err := EncodeActionArgs(args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode action args: %w", err)
	}

	return EncodeQueryComponents(
		input.DataProvider,
		input.StreamID,
		"price_below_threshold",
		argsBytes,
	)
}

// BuildValueInRangeQueryComponents builds query_components for a value_in_range query
func BuildValueInRangeQueryComponents(input types.ValueInRangeInput) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Parse min/max values as Decimal (NUMERIC(36,18))
	minValueDecimal, err := kwiltypes.ParseDecimalExplicit(input.MinValue, 36, 18)
	if err != nil {
		return nil, fmt.Errorf("invalid min_value: %w", err)
	}
	maxValueDecimal, err := kwiltypes.ParseDecimalExplicit(input.MaxValue, 36, 18)
	if err != nil {
		return nil, fmt.Errorf("invalid max_value: %w", err)
	}

	args := []any{
		input.DataProvider,
		input.StreamID,
		input.Timestamp,
		minValueDecimal,
		maxValueDecimal,
		input.FrozenAt,
	}

	argsBytes, err := EncodeActionArgs(args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode action args: %w", err)
	}

	return EncodeQueryComponents(
		input.DataProvider,
		input.StreamID,
		"value_in_range",
		argsBytes,
	)
}

// BuildValueEqualsQueryComponents builds query_components for a value_equals query
func BuildValueEqualsQueryComponents(input types.ValueEqualsInput) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Parse target/tolerance values as Decimal (NUMERIC(36,18))
	targetValueDecimal, err := kwiltypes.ParseDecimalExplicit(input.TargetValue, 36, 18)
	if err != nil {
		return nil, fmt.Errorf("invalid target_value: %w", err)
	}
	toleranceDecimal, err := kwiltypes.ParseDecimalExplicit(input.Tolerance, 36, 18)
	if err != nil {
		return nil, fmt.Errorf("invalid tolerance: %w", err)
	}

	args := []any{
		input.DataProvider,
		input.StreamID,
		input.Timestamp,
		targetValueDecimal,
		toleranceDecimal,
		input.FrozenAt,
	}

	argsBytes, err := EncodeActionArgs(args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode action args: %w", err)
	}

	return EncodeQueryComponents(
		input.DataProvider,
		input.StreamID,
		"value_equals",
		argsBytes,
	)
}

// indexChangeInRangeArgs builds the positional argument list for
// index_change_in_range, parsing the bounds that are present.
//
// A nil bound stays an untyped nil so it encodes as SQL NULL, which is how the
// action reads an open tail. Handing it a typed (*Decimal)(nil) instead would
// not.
//
// The order below IS the contract with the node action; EncodeActionArgs is
// purely positional.
func indexChangeInRangeArgs(
	dataProvider, streamID string,
	timestamp int64,
	baseTime *int64,
	timeInterval int64,
	minChange, maxChange *string,
	frozenAt *int64,
) ([]any, error) {
	var minChangeArg, maxChangeArg any

	if minChange != nil {
		parsed, err := kwiltypes.ParseDecimalExplicit(*minChange, 36, 18)
		if err != nil {
			return nil, fmt.Errorf("invalid min_change: %w", err)
		}
		minChangeArg = parsed
	}
	if maxChange != nil {
		parsed, err := kwiltypes.ParseDecimalExplicit(*maxChange, 36, 18)
		if err != nil {
			return nil, fmt.Errorf("invalid max_change: %w", err)
		}
		maxChangeArg = parsed
	}

	// The node refuses min >= max, and it compares at NUMERIC(36,18). Checking
	// here rather than in Validate() is what lets this comparison happen at the
	// same precision instead of on the decimal strings, where "2.0" and "2" and
	// "1e-19" do not order the way they read.
	if minChangeArg != nil && maxChangeArg != nil {
		order, err := minChangeArg.(*kwiltypes.Decimal).Cmp(maxChangeArg.(*kwiltypes.Decimal))
		if err != nil {
			return nil, fmt.Errorf("failed to compare min_change and max_change: %w", err)
		}
		if order >= 0 {
			return nil, fmt.Errorf(
				"min_change must be less than max_change, got [%s, %s)", *minChange, *maxChange)
		}
	}

	// index_change_in_range($data_provider, $stream_id, $timestamp, $base_time,
	//                       $time_interval, $min_change, $max_change, $frozen_at)
	return []any{
		dataProvider,
		streamID,
		timestamp,
		baseTime,
		timeInterval,
		minChangeArg,
		maxChangeArg,
		frozenAt,
	}, nil
}

// CreateIndexChangeInRangeMarket creates a binary prediction market that settles
// TRUE if the stream's percentage change over TimeInterval falls in
// [MinChange, MaxChange) at the timestamp.
//
// Example: "Will year-over-year inflation land between 2% and 3%?"
//   - DataProvider: "0x1234...abcd"
//   - StreamID: "stcpiyoy00000000000000000000000"
//   - Timestamp: 1735689600
//   - TimeInterval: 31536000 (365 days)
//   - MinChange: "2", MaxChange: "3"
//
// Leave MinChange or MaxChange nil to strike an open tail, which is how the outer
// two buckets of a set are struck. Both nil is rejected.
//
// TimeInterval and BaseTime go into the market hash and cannot be changed once the
// market exists.
func (o *OrderBook) CreateIndexChangeInRangeMarket(
	ctx context.Context,
	input CreateIndexChangeInRangeMarketInput,
	opts ...kwilClientType.TxOpt,
) (kwiltypes.Hash, error) {
	query := types.IndexChangeInRangeInput{
		DataProvider: input.DataProvider,
		StreamID:     input.StreamID,
		Timestamp:    input.Timestamp,
		BaseTime:     input.BaseTime,
		TimeInterval: input.TimeInterval,
		MinChange:    input.MinChange,
		MaxChange:    input.MaxChange,
		FrozenAt:     input.FrozenAt,
	}

	queryComponents, err := BuildIndexChangeInRangeQueryComponents(query)
	if err != nil {
		return kwiltypes.Hash{}, err
	}

	return o.CreateMarket(ctx, types.CreateMarketInput{
		Bridge:          input.Bridge,
		QueryComponents: queryComponents,
		SettleTime:      input.SettleTime,
		MaxSpread:       input.MaxSpread,
		MinOrderSize:    input.MinOrderSize,
	}, opts...)
}

// BuildIndexChangeInRangeQueryComponents builds query_components for an
// index_change_in_range query
func BuildIndexChangeInRangeQueryComponents(input types.IndexChangeInRangeInput) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	args, err := indexChangeInRangeArgs(
		input.DataProvider,
		input.StreamID,
		input.Timestamp,
		input.BaseTime,
		input.TimeInterval,
		input.MinChange,
		input.MaxChange,
		input.FrozenAt,
	)
	if err != nil {
		return nil, err
	}

	argsBytes, err := EncodeActionArgs(args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode action args: %w", err)
	}

	return EncodeQueryComponents(
		input.DataProvider,
		input.StreamID,
		"index_change_in_range",
		argsBytes,
	)
}
