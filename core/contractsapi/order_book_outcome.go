package contractsapi

import (
	"context"
	"fmt"
	"strings"

	kwilClientType "github.com/trufnetwork/kwil-db/core/client/types"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ═══════════════════════════════════════════════════════════════
// OUTCOME MARKET HELPER TYPES (ids 13-16, node migration 063)
// A market on an outcome action always settles: YES or NO once the print
// arrives, or INVALID at block time Timestamp + Window when it has not, which
// pays every YES and NO share $0.50 and refunds every bid.
// ═══════════════════════════════════════════════════════════════

// CreatePriceAboveThresholdV2MarketInput contains all parameters for creating a
// "Will X be above Y?" market on price_above_threshold_v2
type CreatePriceAboveThresholdV2MarketInput struct {
	// Query parameters (for attestation)
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // T: the point the record is read at, unix seconds
	Window       int64  // W: seconds after T the market waits for the print
	Tolerance    int64  // seconds before T a record may be stamped
	Threshold    string // decimal string, YES if value > Threshold

	// Market parameters
	Bridge       string // Bridge namespace, e.g. eth_usdc
	SettleTime   int64  // Unix timestamp when market can be settled
	MaxSpread    int    // Maximum spread for LP rewards (1-50 cents)
	MinOrderSize int64  // Minimum order size for LP rewards
}

// CreatePriceBelowThresholdV2MarketInput contains all parameters for creating a
// "Will X be below Y?" market on price_below_threshold_v2
type CreatePriceBelowThresholdV2MarketInput struct {
	// Query parameters (for attestation)
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // T: the point the record is read at, unix seconds
	Window       int64  // W: seconds after T the market waits for the print
	Tolerance    int64  // seconds before T a record may be stamped
	Threshold    string // decimal string, YES if value < Threshold

	// Market parameters
	Bridge       string // Bridge namespace, e.g. eth_usdc
	SettleTime   int64  // Unix timestamp when market can be settled
	MaxSpread    int    // Maximum spread for LP rewards (1-50 cents)
	MinOrderSize int64  // Minimum order size for LP rewards
}

// CreateValueInRangeV2MarketInput contains all parameters for creating a
// "Will X land in [Y, Z)?" market on value_in_range_v2
type CreateValueInRangeV2MarketInput struct {
	// Query parameters (for attestation)
	DataProvider string  // 0x-prefixed Ethereum address of the data provider
	StreamID     string  // 32-character stream ID
	Timestamp    int64   // T: the point the record is read at, unix seconds
	Window       int64   // W: seconds after T the market waits for the print
	Tolerance    int64   // seconds before T a record may be stamped
	MinValue     *string // lower bound, inclusive; nil for an open tail
	MaxValue     *string // upper bound, exclusive; nil for an open tail

	// Market parameters
	Bridge       string // Bridge namespace, e.g. eth_usdc
	SettleTime   int64  // Unix timestamp when market can be settled
	MaxSpread    int    // Maximum spread for LP rewards (1-50 cents)
	MinOrderSize int64  // Minimum order size for LP rewards
}

// CreateIndexChangeInRangeV2MarketInput contains all parameters for creating a
// "Will X's change land in [Y%, Z%)?" market on index_change_in_range_v2
type CreateIndexChangeInRangeV2MarketInput struct {
	// Query parameters (for attestation)
	DataProvider string  // 0x-prefixed Ethereum address of the data provider
	StreamID     string  // 32-character stream ID
	Timestamp    int64   // T: the earliest stamp the current record may carry, unix seconds
	Window       int64   // W: seconds after T the market waits for the print
	Tolerance    int64   // seconds after T the current record may be stamped
	TimeInterval int64   // seconds between the prior and the current record (e.g. 31536000 for YoY)
	MinChange    *string // lower bound in percent, inclusive; nil for an open tail
	MaxChange    *string // upper bound in percent, exclusive; nil for an open tail

	// Market parameters
	Bridge       string // Bridge namespace, e.g. eth_usdc
	SettleTime   int64  // Unix timestamp when market can be settled
	MaxSpread    int    // Maximum spread for LP rewards (1-50 cents)
	MinOrderSize int64  // Minimum order size for LP rewards
}

// ═══════════════════════════════════════════════════════════════
// OUTCOME MARKET HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════

// CreatePriceAboveThresholdV2Market creates a market that settles YES if the
// latest record stamped in [Timestamp - Tolerance, Timestamp] is above the
// threshold, NO if not, and INVALID if no such record exists by block time
// Timestamp + Window.
//
// Example: "Will BTC close above $100,000 on 31 Dec 2025?"
//   - Timestamp: 1767139200 (the reference time)
//   - Tolerance: 86400, Window: 259200
//   - Threshold: "100000"
func (o *OrderBook) CreatePriceAboveThresholdV2Market(
	ctx context.Context,
	input CreatePriceAboveThresholdV2MarketInput,
	opts ...kwilClientType.TxOpt,
) (kwiltypes.Hash, error) {
	queryComponents, err := BuildPriceAboveThresholdV2QueryComponents(types.PriceAboveThresholdV2Input{
		DataProvider: input.DataProvider,
		StreamID:     input.StreamID,
		Timestamp:    input.Timestamp,
		Window:       input.Window,
		Tolerance:    input.Tolerance,
		Threshold:    input.Threshold,
	})
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

// CreatePriceBelowThresholdV2Market creates a market that settles YES if the
// latest record stamped in [Timestamp - Tolerance, Timestamp] is below the
// threshold, NO if not, and INVALID if no such record exists by block time
// Timestamp + Window.
func (o *OrderBook) CreatePriceBelowThresholdV2Market(
	ctx context.Context,
	input CreatePriceBelowThresholdV2MarketInput,
	opts ...kwilClientType.TxOpt,
) (kwiltypes.Hash, error) {
	queryComponents, err := BuildPriceBelowThresholdV2QueryComponents(types.PriceBelowThresholdV2Input{
		DataProvider: input.DataProvider,
		StreamID:     input.StreamID,
		Timestamp:    input.Timestamp,
		Window:       input.Window,
		Tolerance:    input.Tolerance,
		Threshold:    input.Threshold,
	})
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

// CreateValueInRangeV2Market creates a market that settles YES if the latest
// record stamped in [Timestamp - Tolerance, Timestamp] is in [MinValue,
// MaxValue), NO if not, and INVALID if no such record exists by block time
// Timestamp + Window.
//
// Leave MinValue or MaxValue nil for an open tail. A ladder strikes every
// bucket this way, with open outer tails, so exactly one bucket settles YES.
func (o *OrderBook) CreateValueInRangeV2Market(
	ctx context.Context,
	input CreateValueInRangeV2MarketInput,
	opts ...kwilClientType.TxOpt,
) (kwiltypes.Hash, error) {
	queryComponents, err := BuildValueInRangeV2QueryComponents(types.ValueInRangeV2Input{
		DataProvider: input.DataProvider,
		StreamID:     input.StreamID,
		Timestamp:    input.Timestamp,
		Window:       input.Window,
		Tolerance:    input.Tolerance,
		MinValue:     input.MinValue,
		MaxValue:     input.MaxValue,
	})
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

// CreateIndexChangeInRangeV2Market creates a market that settles YES if the
// stream's percent change over TimeInterval lands in [MinChange, MaxChange),
// NO if not, and INVALID if a record is missing by block time Timestamp +
// Window.
//
// Example: "Will year-over-year CPI land between 2% and 3%?"
//   - Timestamp: the data month start, e.g. 1759276800 (1 Oct 2025)
//   - Tolerance: 172800, Window: 7776000, TimeInterval: 31536000
//   - MinChange: "2", MaxChange: "3"
func (o *OrderBook) CreateIndexChangeInRangeV2Market(
	ctx context.Context,
	input CreateIndexChangeInRangeV2MarketInput,
	opts ...kwilClientType.TxOpt,
) (kwiltypes.Hash, error) {
	queryComponents, err := BuildIndexChangeInRangeV2QueryComponents(types.IndexChangeInRangeV2Input{
		DataProvider: input.DataProvider,
		StreamID:     input.StreamID,
		Timestamp:    input.Timestamp,
		Window:       input.Window,
		Tolerance:    input.Tolerance,
		TimeInterval: input.TimeInterval,
		MinChange:    input.MinChange,
		MaxChange:    input.MaxChange,
	})
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

// BuildPriceAboveThresholdV2QueryComponents builds query_components for a
// price_above_threshold_v2 query
func BuildPriceAboveThresholdV2QueryComponents(input types.PriceAboveThresholdV2Input) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	threshold, err := outcomeNumericArg("threshold", &input.Threshold)
	if err != nil {
		return nil, err
	}
	// price_above_threshold_v2($data_provider, $stream_id, $timestamp, $window, $tolerance, $threshold)
	return encodeOutcomeQuery(input.ActionName(), input.DataProvider, input.StreamID,
		input.Timestamp, input.Window, input.Tolerance, threshold)
}

// BuildPriceBelowThresholdV2QueryComponents builds query_components for a
// price_below_threshold_v2 query
func BuildPriceBelowThresholdV2QueryComponents(input types.PriceBelowThresholdV2Input) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	threshold, err := outcomeNumericArg("threshold", &input.Threshold)
	if err != nil {
		return nil, err
	}
	// price_below_threshold_v2($data_provider, $stream_id, $timestamp, $window, $tolerance, $threshold)
	return encodeOutcomeQuery(input.ActionName(), input.DataProvider, input.StreamID,
		input.Timestamp, input.Window, input.Tolerance, threshold)
}

// BuildValueInRangeV2QueryComponents builds query_components for a
// value_in_range_v2 query
func BuildValueInRangeV2QueryComponents(input types.ValueInRangeV2Input) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	minValue, err := outcomeNumericArg("min_value", input.MinValue)
	if err != nil {
		return nil, err
	}
	maxValue, err := outcomeNumericArg("max_value", input.MaxValue)
	if err != nil {
		return nil, err
	}
	// value_in_range_v2($data_provider, $stream_id, $timestamp, $window, $tolerance,
	//                   $min_value, $max_value)
	return encodeOutcomeQuery(input.ActionName(), input.DataProvider, input.StreamID,
		input.Timestamp, input.Window, input.Tolerance, minValue, maxValue)
}

// BuildIndexChangeInRangeV2QueryComponents builds query_components for an
// index_change_in_range_v2 query
func BuildIndexChangeInRangeV2QueryComponents(input types.IndexChangeInRangeV2Input) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	minChange, err := outcomeNumericArg("min_change", input.MinChange)
	if err != nil {
		return nil, err
	}
	maxChange, err := outcomeNumericArg("max_change", input.MaxChange)
	if err != nil {
		return nil, err
	}
	// index_change_in_range_v2($data_provider, $stream_id, $timestamp, $window, $tolerance,
	//                          $time_interval, $min_change, $max_change)
	return encodeOutcomeQuery(input.ActionName(), input.DataProvider, input.StreamID,
		input.Timestamp, input.Window, input.Tolerance, input.TimeInterval, minChange, maxChange)
}

// outcomeNumericArg parses a threshold or bound as NUMERIC(36,18). A nil value
// stays an untyped nil, which encodes as NULL: an open tail.
func outcomeNumericArg(param string, value *string) (any, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := kwiltypes.ParseDecimalExplicit(*value, 36, 18)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", param, err)
	}
	if parsed.NaN() {
		return nil, fmt.Errorf("%s must be a number, got NaN", param)
	}
	return parsed, nil
}

// encodeOutcomeQuery encodes an outcome action's args and its query_components.
// The args are positional: data_provider, stream_id, timestamp, window,
// tolerance, then the action's own. The data provider is lowercased, as every
// SDK does for these actions, so one question always has one market hash.
func encodeOutcomeQuery(
	actionName, dataProvider, streamID string,
	timestamp, window, tolerance int64,
	rest ...any,
) ([]byte, error) {
	dataProvider = strings.ToLower(dataProvider)

	args := append([]any{dataProvider, streamID, timestamp, window, tolerance}, rest...)
	argsBytes, err := EncodeActionArgs(args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode action args: %w", err)
	}
	return EncodeQueryComponents(dataProvider, streamID, actionName, argsBytes)
}
