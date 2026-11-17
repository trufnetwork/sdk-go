package types

import (
	"encoding/hex"
	"fmt"

	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
)

// ═══════════════════════════════════════════════════════════════
// OUTCOME ACTION INPUT TYPES (ids 13-16, node migration 063)
// Each action answers 1 (YES), 0 (NO) or -1 (INVALID). A market on one
// always settles: a capture at or after block time Timestamp + Window that
// finds no qualifying record answers INVALID and the market refunds every
// position. A record written after that deadline but before the first
// capture still decides the market.
// ═══════════════════════════════════════════════════════════════

// Node bounds on the outcome action arguments, in seconds.
const (
	// OutcomeMaxSpanSeconds (10 years of 365 days) caps Window and TimeInterval.
	OutcomeMaxSpanSeconds int64 = 315360000
	// OutcomeMaxToleranceSeconds (45 days) caps Tolerance.
	OutcomeMaxToleranceSeconds int64 = 3888000
)

// PriceAboveThresholdV2Input contains parameters for "price_above_threshold_v2".
// YES if the latest record stamped in [Timestamp - Tolerance, Timestamp] is
// above Threshold, NO if not, INVALID if no such record exists by block time
// Timestamp + Window.
type PriceAboveThresholdV2Input struct {
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // T: the point the record is read at, unix seconds
	Window       int64  // W: seconds after T the market waits for the print
	Tolerance    int64  // seconds before T a record may be stamped
	Threshold    string // decimal string, YES if value > Threshold
}

// Validate checks the input against the node's bounds.
func (p *PriceAboveThresholdV2Input) Validate() error {
	if err := validateOutcomeQuery(p.DataProvider, p.StreamID, p.Timestamp, p.Window, p.Tolerance); err != nil {
		return err
	}
	return validateOutcomeThreshold(p.Threshold)
}

// ActionName returns the action name for this input type
func (p *PriceAboveThresholdV2Input) ActionName() string {
	return "price_above_threshold_v2"
}

// PriceBelowThresholdV2Input contains parameters for "price_below_threshold_v2".
// YES if the latest record stamped in [Timestamp - Tolerance, Timestamp] is
// below Threshold, NO if not, INVALID if no such record exists by block time
// Timestamp + Window.
type PriceBelowThresholdV2Input struct {
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // T: the point the record is read at, unix seconds
	Window       int64  // W: seconds after T the market waits for the print
	Tolerance    int64  // seconds before T a record may be stamped
	Threshold    string // decimal string, YES if value < Threshold
}

// Validate checks the input against the node's bounds.
func (p *PriceBelowThresholdV2Input) Validate() error {
	if err := validateOutcomeQuery(p.DataProvider, p.StreamID, p.Timestamp, p.Window, p.Tolerance); err != nil {
		return err
	}
	return validateOutcomeThreshold(p.Threshold)
}

// ActionName returns the action name for this input type
func (p *PriceBelowThresholdV2Input) ActionName() string {
	return "price_below_threshold_v2"
}

// ValueInRangeV2Input contains parameters for "value_in_range_v2".
// YES if the latest record stamped in [Timestamp - Tolerance, Timestamp] is in
// [MinValue, MaxValue), NO if not, INVALID if no such record exists by block
// time Timestamp + Window.
//
// Unlike value_in_range, the range is half-open and either bound may be nil for
// an open tail, so one set of buckets tiles the line and exactly one settles YES.
type ValueInRangeV2Input struct {
	DataProvider string  // 0x-prefixed Ethereum address of the data provider
	StreamID     string  // 32-character stream ID
	Timestamp    int64   // T: the point the record is read at, unix seconds
	Window       int64   // W: seconds after T the market waits for the print
	Tolerance    int64   // seconds before T a record may be stamped
	MinValue     *string // lower bound, inclusive; nil for an open tail
	MaxValue     *string // upper bound, exclusive; nil for an open tail
}

// Validate checks the input against the node's bounds.
func (v *ValueInRangeV2Input) Validate() error {
	if err := validateOutcomeQuery(v.DataProvider, v.StreamID, v.Timestamp, v.Window, v.Tolerance); err != nil {
		return err
	}
	return validateOutcomeBounds("min_value", "max_value", v.MinValue, v.MaxValue)
}

// ActionName returns the action name for this input type
func (v *ValueInRangeV2Input) ActionName() string {
	return "value_in_range_v2"
}

// IndexChangeInRangeV2Input contains parameters for "index_change_in_range_v2".
// YES if the percent change from the prior record to the current one is in
// [MinChange, MaxChange). The current record is the first stamped in
// [Timestamp, Timestamp + Tolerance]; the prior is the one nearest
// current - TimeInterval, at most Tolerance away. INVALID if either is missing
// by block time Timestamp + Window, or the prior is zero.
//
// Either bound may be nil for an open tail. There is no base_time: the change
// is read from the stream's raw records.
type IndexChangeInRangeV2Input struct {
	DataProvider string  // 0x-prefixed Ethereum address of the data provider
	StreamID     string  // 32-character stream ID
	Timestamp    int64   // T: the earliest stamp the current record may carry, unix seconds
	Window       int64   // W: seconds after T the market waits for the print
	Tolerance    int64   // seconds after T the current record may be stamped
	TimeInterval int64   // seconds between the prior and the current record (e.g. 31536000 for YoY)
	MinChange    *string // lower bound in percent, inclusive; nil for an open tail
	MaxChange    *string // upper bound in percent, exclusive; nil for an open tail
}

// Validate checks the input against the node's bounds.
func (v *IndexChangeInRangeV2Input) Validate() error {
	if err := validateOutcomeQuery(v.DataProvider, v.StreamID, v.Timestamp, v.Window, v.Tolerance); err != nil {
		return err
	}
	if v.TimeInterval <= 0 || v.TimeInterval > OutcomeMaxSpanSeconds {
		return fmt.Errorf("time_interval must be in (0, %d], got %d", OutcomeMaxSpanSeconds, v.TimeInterval)
	}
	// The prior is read within tolerance of current - interval, so the two
	// reads must not overlap.
	if 2*v.Tolerance >= v.TimeInterval {
		return fmt.Errorf("time_interval (%d) must be more than twice tolerance (%d)", v.TimeInterval, v.Tolerance)
	}
	return validateOutcomeBounds("min_change", "max_change", v.MinChange, v.MaxChange)
}

// ActionName returns the action name for this input type
func (v *IndexChangeInRangeV2Input) ActionName() string {
	return "index_change_in_range_v2"
}

// OutcomeActionInput is implemented by the input types of the outcome actions
// (ids 13-16).
type OutcomeActionInput interface {
	Validate() error
	ActionName() string
}

var (
	_ OutcomeActionInput = (*PriceAboveThresholdV2Input)(nil)
	_ OutcomeActionInput = (*PriceBelowThresholdV2Input)(nil)
	_ OutcomeActionInput = (*ValueInRangeV2Input)(nil)
	_ OutcomeActionInput = (*IndexChangeInRangeV2Input)(nil)
)

// parseOutcomeNumeric parses a threshold or bound the way the outcome actions
// take it, as NUMERIC(36,18). NaN parses at that type, so it is refused here,
// as the node does.
func parseOutcomeNumeric(param, value string) (*kwiltypes.Decimal, error) {
	parsed, err := kwiltypes.ParseDecimalExplicit(value, 36, 18)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", param, err)
	}
	if parsed.NaN() {
		return nil, fmt.Errorf("%s must be a number, got NaN", param)
	}
	return parsed, nil
}

// validateOutcomeQuery checks the arguments every outcome action shares. It
// mirrors the node and is never stricter. The node's one other rule, that T is
// at most 10 years past the block time, needs a block time and is left to it.
func validateOutcomeQuery(dataProvider, streamID string, timestamp, window, tolerance int64) error {
	if len(dataProvider) != 42 {
		return fmt.Errorf("data_provider must be 42 characters (0x + 40 hex), got %d", len(dataProvider))
	}
	if dataProvider[:2] != "0x" && dataProvider[:2] != "0X" {
		return fmt.Errorf("data_provider must be 0x-prefixed")
	}
	if _, err := hex.DecodeString(dataProvider[2:]); err != nil {
		return fmt.Errorf("data_provider must be 40 hex characters after 0x")
	}
	if len(streamID) != 32 {
		return fmt.Errorf("stream_id must be exactly 32 characters, got %d", len(streamID))
	}
	if timestamp < 0 {
		return fmt.Errorf("timestamp must not be negative, got %d", timestamp)
	}
	if window <= 0 || window > OutcomeMaxSpanSeconds {
		return fmt.Errorf("window must be in (0, %d], got %d", OutcomeMaxSpanSeconds, window)
	}
	if tolerance < 0 || tolerance > OutcomeMaxToleranceSeconds {
		return fmt.Errorf("tolerance must be in [0, %d], got %d", OutcomeMaxToleranceSeconds, tolerance)
	}
	// A print stamped anywhere in the tolerance must be able to arrive before
	// the deadline.
	if window < tolerance {
		return fmt.Errorf("window (%d) must not be less than tolerance (%d)", window, tolerance)
	}
	return nil
}

// validateOutcomeThreshold checks the threshold of ids 14 and 15, which may
// not be NULL.
func validateOutcomeThreshold(threshold string) error {
	if threshold == "" {
		return fmt.Errorf("threshold is required")
	}
	_, err := parseOutcomeNumeric("threshold", threshold)
	return err
}

// validateOutcomeBounds checks a [min, max) pair. Either may be nil for an
// open tail, not both, and min must be below max at NUMERIC(36,18), where
// "2.0" and "2" are the same number.
func validateOutcomeBounds(minParam, maxParam string, minValue, maxValue *string) error {
	if minValue == nil && maxValue == nil {
		return fmt.Errorf("at least one of %s or %s is required", minParam, maxParam)
	}
	var lo, hi *kwiltypes.Decimal
	if minValue != nil {
		if *minValue == "" {
			return fmt.Errorf("%s is set but empty; use nil for an open tail", minParam)
		}
		parsed, err := parseOutcomeNumeric(minParam, *minValue)
		if err != nil {
			return err
		}
		lo = parsed
	}
	if maxValue != nil {
		if *maxValue == "" {
			return fmt.Errorf("%s is set but empty; use nil for an open tail", maxParam)
		}
		parsed, err := parseOutcomeNumeric(maxParam, *maxValue)
		if err != nil {
			return err
		}
		hi = parsed
	}
	if lo == nil || hi == nil {
		return nil
	}
	order, err := lo.Cmp(hi)
	if err != nil {
		return fmt.Errorf("failed to compare %s and %s: %w", minParam, maxParam, err)
	}
	if order >= 0 {
		return fmt.Errorf("%s must be less than %s, got [%s, %s)", minParam, maxParam, *minValue, *maxValue)
	}
	return nil
}
