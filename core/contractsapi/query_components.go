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
	DataProvider string   `json:"data_provider"`
	StreamID     string   `json:"stream_id"`
	ActionID     string   `json:"action_id"`
	Type         string   `json:"type"`       // "above", "below", "between", "equals"
	Thresholds   []string `json:"thresholds"` // Formatted numeric values
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
	// Based on 040-binary-attestation-actions.sql
	switch actionID {
	case "price_above_threshold":
		market.Type = "above"
		if len(args) >= 4 {
			market.Thresholds = append(market.Thresholds, formatArg(args[3]))
		}
	case "price_below_threshold":
		market.Type = "below"
		if len(args) >= 4 {
			market.Thresholds = append(market.Thresholds, formatArg(args[3]))
		}
	case "value_in_range":
		market.Type = "between"
		if len(args) >= 5 {
			market.Thresholds = append(market.Thresholds, formatArg(args[3]), formatArg(args[4]))
		}
	case "value_equals":
		market.Type = "equals"
		if len(args) >= 5 {
			market.Thresholds = append(market.Thresholds, formatArg(args[3]), formatArg(args[4]))
		}
	default:
		market.Type = "unknown"
	}

	return market, nil
}
