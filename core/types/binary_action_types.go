package types

import "fmt"

// ═══════════════════════════════════════════════════════════════
// BINARY ACTION INPUT TYPES
// These types represent the parameters for binary attestation actions
// that return TRUE/FALSE results for prediction market settlement.
// ═══════════════════════════════════════════════════════════════

// PriceAboveThresholdInput contains parameters for "price_above_threshold" action
// Use case: "Will BTC exceed $100,000?"
// Returns TRUE if value > threshold at the specified timestamp
type PriceAboveThresholdInput struct {
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // Unix timestamp to check the value at
	Threshold    string // Threshold value as decimal string (e.g., "100000.00")
	FrozenAt     *int64 // Optional: Unix timestamp to freeze the value lookup
}

// Validate checks if the input is valid
func (p *PriceAboveThresholdInput) Validate() error {
	if len(p.DataProvider) != 42 {
		return fmt.Errorf("data_provider must be 42 characters (0x + 40 hex), got %d", len(p.DataProvider))
	}
	if p.DataProvider[:2] != "0x" {
		return fmt.Errorf("data_provider must be 0x-prefixed")
	}
	if len(p.StreamID) != 32 {
		return fmt.Errorf("stream_id must be exactly 32 characters, got %d", len(p.StreamID))
	}
	if p.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}
	if p.Threshold == "" {
		return fmt.Errorf("threshold is required")
	}
	return nil
}

// ActionName returns the action name for this input type
func (p *PriceAboveThresholdInput) ActionName() string {
	return "price_above_threshold"
}

// PriceBelowThresholdInput contains parameters for "price_below_threshold" action
// Use case: "Will unemployment drop below 4%?"
// Returns TRUE if value < threshold at the specified timestamp
type PriceBelowThresholdInput struct {
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // Unix timestamp to check the value at
	Threshold    string // Threshold value as decimal string (e.g., "4.0")
	FrozenAt     *int64 // Optional: Unix timestamp to freeze the value lookup
}

// Validate checks if the input is valid
func (p *PriceBelowThresholdInput) Validate() error {
	if len(p.DataProvider) != 42 {
		return fmt.Errorf("data_provider must be 42 characters (0x + 40 hex), got %d", len(p.DataProvider))
	}
	if p.DataProvider[:2] != "0x" {
		return fmt.Errorf("data_provider must be 0x-prefixed")
	}
	if len(p.StreamID) != 32 {
		return fmt.Errorf("stream_id must be exactly 32 characters, got %d", len(p.StreamID))
	}
	if p.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}
	if p.Threshold == "" {
		return fmt.Errorf("threshold is required")
	}
	return nil
}

// ActionName returns the action name for this input type
func (p *PriceBelowThresholdInput) ActionName() string {
	return "price_below_threshold"
}

// ValueInRangeInput contains parameters for "value_in_range" action
// Use case: "Will BTC stay between $90k-$110k?"
// Returns TRUE if min <= value <= max at the specified timestamp
type ValueInRangeInput struct {
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // Unix timestamp to check the value at
	MinValue     string // Minimum value (inclusive) as decimal string
	MaxValue     string // Maximum value (inclusive) as decimal string
	FrozenAt     *int64 // Optional: Unix timestamp to freeze the value lookup
}

// Validate checks if the input is valid
func (v *ValueInRangeInput) Validate() error {
	if len(v.DataProvider) != 42 {
		return fmt.Errorf("data_provider must be 42 characters (0x + 40 hex), got %d", len(v.DataProvider))
	}
	if v.DataProvider[:2] != "0x" {
		return fmt.Errorf("data_provider must be 0x-prefixed")
	}
	if len(v.StreamID) != 32 {
		return fmt.Errorf("stream_id must be exactly 32 characters, got %d", len(v.StreamID))
	}
	if v.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}
	if v.MinValue == "" {
		return fmt.Errorf("min_value is required")
	}
	if v.MaxValue == "" {
		return fmt.Errorf("max_value is required")
	}
	return nil
}

// ActionName returns the action name for this input type
func (v *ValueInRangeInput) ActionName() string {
	return "value_in_range"
}

// ValueEqualsInput contains parameters for "value_equals" action
// Use case: "Will Fed rate be exactly 5.25%?"
// Returns TRUE if value = target ± tolerance at the specified timestamp
type ValueEqualsInput struct {
	DataProvider string // 0x-prefixed Ethereum address of the data provider
	StreamID     string // 32-character stream ID
	Timestamp    int64  // Unix timestamp to check the value at
	TargetValue  string // Target value as decimal string (e.g., "5.25")
	Tolerance    string // Tolerance for equality check (e.g., "0.01" means ±0.01)
	FrozenAt     *int64 // Optional: Unix timestamp to freeze the value lookup
}

// Validate checks if the input is valid
func (v *ValueEqualsInput) Validate() error {
	if len(v.DataProvider) != 42 {
		return fmt.Errorf("data_provider must be 42 characters (0x + 40 hex), got %d", len(v.DataProvider))
	}
	if v.DataProvider[:2] != "0x" {
		return fmt.Errorf("data_provider must be 0x-prefixed")
	}
	if len(v.StreamID) != 32 {
		return fmt.Errorf("stream_id must be exactly 32 characters, got %d", len(v.StreamID))
	}
	if v.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}
	if v.TargetValue == "" {
		return fmt.Errorf("target_value is required")
	}
	if v.Tolerance == "" {
		return fmt.Errorf("tolerance is required")
	}
	return nil
}

// ActionName returns the action name for this input type
func (v *ValueEqualsInput) ActionName() string {
	return "value_equals"
}

// ═══════════════════════════════════════════════════════════════
// BINARY ACTION INTERFACE
// ═══════════════════════════════════════════════════════════════

// BinaryActionInput is an interface for all binary action input types
type BinaryActionInput interface {
	Validate() error
	ActionName() string
}

// Ensure all input types implement the interface
var (
	_ BinaryActionInput = (*PriceAboveThresholdInput)(nil)
	_ BinaryActionInput = (*PriceBelowThresholdInput)(nil)
	_ BinaryActionInput = (*ValueInRangeInput)(nil)
	_ BinaryActionInput = (*ValueEqualsInput)(nil)
)
