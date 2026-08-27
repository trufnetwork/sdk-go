package contractsapi

import (
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// QueryComponentsABI defines the ABI type for encoding query_components tuple
// Format: (address data_provider, bytes32 stream_id, string action_id, bytes args)
var QueryComponentsABI abi.Arguments

func init() {
	addressType, err := abi.NewType("address", "", nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create address ABI type: %v", err))
	}
	bytes32Type, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create bytes32 ABI type: %v", err))
	}
	stringType, err := abi.NewType("string", "", nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create string ABI type: %v", err))
	}
	bytesType, err := abi.NewType("bytes", "", nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create bytes ABI type: %v", err))
	}

	QueryComponentsABI = abi.Arguments{
		{Type: addressType, Name: "data_provider"},
		{Type: bytes32Type, Name: "stream_id"},
		{Type: stringType, Name: "action_id"},
		{Type: bytesType, Name: "args"},
	}
}

// EncodeQueryComponents ABI-encodes the query components tuple.
//
// This function creates the query_components parameter required by the node's
// create_market action. The node will compute the attestation hash internally
// using tn_utils.compute_attestation_hash($query_components).
//
// Parameters:
//   - dataProvider: 0x-prefixed Ethereum address (42 chars, e.g., "0x1234...abcd")
//   - streamID: 32-character stream ID (e.g., "stbtcusd00000000000000000000000")
//   - actionID: Action name (e.g., "price_above_threshold", "get_record")
//   - args: Pre-encoded action arguments (from EncodeActionArgs)
//
// Returns the ABI-encoded query_components bytes
func EncodeQueryComponents(dataProvider, streamID, actionID string, args []byte) ([]byte, error) {
	// Validate data provider
	if len(dataProvider) != 42 {
		return nil, fmt.Errorf("data_provider must be 42 characters (0x + 40 hex), got %d", len(dataProvider))
	}
	if dataProvider[:2] != "0x" {
		return nil, fmt.Errorf("data_provider must be 0x-prefixed, got %s", dataProvider[:2])
	}

	// Validate stream ID
	if len(streamID) != 32 {
		return nil, fmt.Errorf("stream_id must be exactly 32 characters, got %d", len(streamID))
	}

	// Validate action ID
	if actionID == "" {
		return nil, fmt.Errorf("action_id cannot be empty")
	}

	// Convert data provider to address
	dpAddr := common.HexToAddress(dataProvider)

	// Convert stream ID to bytes32 (right-padded with zeros)
	var sidBytes32 [32]byte
	copy(sidBytes32[:], []byte(streamID))

	// Encode the tuple
	encoded, err := QueryComponentsABI.Pack(dpAddr, sidBytes32, actionID, args)
	if err != nil {
		return nil, fmt.Errorf("failed to ABI-encode query_components: %w", err)
	}

	return encoded, nil
}

// DecodeQueryComponents decodes ABI-encoded query_components back to its parts
//
// Returns:
//   - dataProvider: 0x-prefixed Ethereum address
//   - streamID: 32-character stream ID
//   - actionID: Action name
//   - args: Encoded action arguments
func DecodeQueryComponents(encoded []byte) (dataProvider, streamID, actionID string, args []byte, err error) {
	unpacked, err := QueryComponentsABI.Unpack(encoded)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to ABI-decode query_components: %w", err)
	}

	if len(unpacked) != 4 {
		return "", "", "", nil, fmt.Errorf("expected 4 values, got %d", len(unpacked))
	}

	// Extract data provider
	addr, ok := unpacked[0].(common.Address)
	if !ok {
		return "", "", "", nil, fmt.Errorf("expected address for data_provider, got %T", unpacked[0])
	}
	dataProvider = addr.Hex()

	// Extract stream ID (bytes32 -> string, trimming trailing zeros)
	sidBytes, ok := unpacked[1].([32]byte)
	if !ok {
		return "", "", "", nil, fmt.Errorf("expected [32]byte for stream_id, got %T", unpacked[1])
	}
	// Find the actual length by looking for trailing zeros
	// Default to 0 so all-zero bytes yields empty string
	sidLen := 0
	for i := 31; i >= 0; i-- {
		if sidBytes[i] != 0 {
			sidLen = i + 1
			break
		}
	}
	streamID = string(sidBytes[:sidLen])

	// Extract action ID
	actionID, ok = unpacked[2].(string)
	if !ok {
		return "", "", "", nil, fmt.Errorf("expected string for action_id, got %T", unpacked[2])
	}

	// Extract args
	args, ok = unpacked[3].([]byte)
	if !ok {
		return "", "", "", nil, fmt.Errorf("expected []byte for args, got %T", unpacked[3])
	}

	return dataProvider, streamID, actionID, args, nil
}

// MarketData represents the structured content of a prediction market's query components
type MarketData struct {
	DataProvider string `json:"data_provider"`
	StreamID     string `json:"stream_id"`
	ActionID     string `json:"action_id"`
	Type         string `json:"type"` // "above", "below", "between", "equals", "change_between"
	// Thresholds holds the market's strike values in the order the action
	// declares them, one entry per slot. A "change_between" market may strike an
	// open tail, which reads back as an empty string in place rather than a
	// shorter slice -- dropping it would slide the remaining bound into the wrong
	// position.
	Thresholds []string `json:"thresholds"`
	// Timestamp is the point in the stream the query observes, in unix seconds.
	// Every bucket of one market shares it; nil only when the arguments could
	// not be read.
	Timestamp *int64 `json:"timestamp"`
	// FrozenAt is the block height the data is pinned to. It is encoded as NULL
	// to mean "latest", so nil is a real value rather than a decode failure.
	FrozenAt *int64 `json:"frozen_at"`
	// BaseTime is the index base date the query measures against, in unix
	// seconds, or nil for the stream's own default.
	//
	// Only a "change_between" market carries one; it is nil for every other
	// type, which has no such argument.
	BaseTime *int64 `json:"base_time"`
	// TimeInterval is how far back the query looks for its comparison value, in
	// seconds -- e.g. 31536000 for year-over-year.
	//
	// Only a "change_between" market carries one. Two markets over the same
	// stream and the same observation time but different intervals are asking
	// different questions, so this is part of a market's identity rather than
	// presentation.
	TimeInterval *int64 `json:"time_interval"`
}

// readQueryTime fills in a market's query time, but only when the whole
// argument list is present.
//
// Both slots have to exist for either to mean anything. A truncated argument
// list would otherwise leave FrozenAt nil, which is indistinguishable from the
// explicit ABI NULL that every well-formed market carries to mean "latest" --
// so a malformed market would match a healthy one on that component of its
// identity. Leaving Timestamp nil instead hands the whole market to the
// caller's readability check.
func readQueryTime(market *MarketData, args []any, frozenAtIndex int) {
	if len(args) <= frozenAtIndex {
		return
	}
	market.Timestamp = argInt64(args, 2)
	market.FrozenAt = argInt64(args, frozenAtIndex)
}

// argInt64 pulls an INT8 argument out of the decoded args.
//
// Kwil hands these back as *int64, and a nil argument is meaningful rather than
// an error: frozen_at is encoded as NULL to mean "latest".
func argInt64(args []any, index int) *int64 {
	if index < 0 || index >= len(args) || args[index] == nil {
		return nil
	}
	switch v := args[index].(type) {
	case *int64:
		if v == nil {
			return nil
		}
		n := *v
		return &n
	case int64:
		return &v
	case int:
		n := int64(v)
		return &n
	}
	return nil
}

// DecodeMarketData decodes ABI-encoded query_components into high-level MarketData
func DecodeMarketData(encoded []byte) (*MarketData, error) {
	dataProvider, streamID, actionID, argsBytes, err := DecodeQueryComponents(encoded)
	if err != nil {
		return nil, err
	}

	args, err := DecodeActionArgs(argsBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode action args: %w", err)
	}

	market := &MarketData{
		DataProvider: dataProvider,
		StreamID:     streamID,
		ActionID:     actionID,
		Thresholds:   []string{},
	}

	// Helper to format arguments (handling Decimal and pointer types)
	formatArg := func(arg any) string {
		if arg == nil {
			return ""
		}

		// Handle *string directly (common in decoded results)
		if s, ok := arg.(*string); ok {
			if s == nil {
				return ""
			}
			return *s
		}

		// Use reflection to find String() method (handles other pointer types like *Decimal)
		v := reflect.ValueOf(arg)
		method := v.MethodByName("String")
		if method.IsValid() && method.Type().NumIn() == 0 && method.Type().NumOut() == 1 {
			results := method.Call(nil)
			if s, ok := results[0].Interface().(string); ok {
				return s
			}
		}

		return fmt.Sprint(arg)
	}

	// Map action_id to market type and thresholds
	// Based on 040-binary-attestation-actions.sql and
	// 055-index-change-attestation-action.sql
	//
	// Every binary action takes ($data_provider, $stream_id, $timestamp, ...,
	// $frozen_at), so the timestamp is always argument 2 and frozen_at is always
	// last. Only the arguments in between change shape, and they are not all
	// thresholds: index_change_in_range carries $base_time and $time_interval
	// ahead of its two bounds.
	switch actionID {
	case "price_above_threshold":
		market.Type = "above"
		if len(args) >= 4 {
			market.Thresholds = append(market.Thresholds, formatArg(args[3]))
		}
		readQueryTime(market, args, 4)
	case "price_below_threshold":
		market.Type = "below"
		if len(args) >= 4 {
			market.Thresholds = append(market.Thresholds, formatArg(args[3]))
		}
		readQueryTime(market, args, 4)
	case "value_in_range":
		market.Type = "between"
		if len(args) >= 5 {
			market.Thresholds = append(market.Thresholds, formatArg(args[3]), formatArg(args[4]))
		}
		readQueryTime(market, args, 5)
	case "value_equals":
		market.Type = "equals"
		if len(args) >= 5 {
			market.Thresholds = append(market.Thresholds, formatArg(args[3]), formatArg(args[4]))
		}
		readQueryTime(market, args, 5)
	case "index_change_in_range":
		// Its own type rather than "between": the bounds here are half-open and
		// either may be NULL for an open tail, which "between" consumers parse
		// as a number and would reject.
		market.Type = "change_between"
		if len(args) >= 7 {
			market.Thresholds = append(market.Thresholds, formatArg(args[5]), formatArg(args[6]))
			// The two arguments the bounds displaced. They are not strikes, so
			// they do not belong in Thresholds, but they do change the question
			// the market asks and so cannot be dropped either.
			market.BaseTime = argInt64(args, 3)
			market.TimeInterval = argInt64(args, 4)
		}
		readQueryTime(market, args, 7)
	default:
		market.Type = "unknown"
	}

	return market, nil
}
