package contractsapi

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/trufnetwork/kwil-db/core/types"
	sdktypes "github.com/trufnetwork/sdk-go/core/types"
)

// BinaryActionResultIDs defines the action IDs that return boolean results
// These are the binary attestation actions (IDs 6-9)
var BinaryActionResultIDs = map[uint16]bool{
	6: true, // price_above_threshold
	7: true, // price_below_threshold
	8: true, // value_in_range
	9: true, // value_equals
}

// Security limits to prevent memory exhaustion attacks
const (
	maxRows    = 100000 // Maximum rows in query result
	maxColumns = 2      // Maximum columns per row (all public actions return 1-2 columns)
	maxArgs    = 10     // Maximum arguments (current max is 8 for get_index_change)
)

// Binary reading helpers

// readUint32LE reads a uint32 value in little-endian format
func readUint32LE(buf []byte, offset int) uint32 {
	if offset+4 > len(buf) {
		return 0
	}
	return binary.LittleEndian.Uint32(buf[offset:])
}

// readUint16LE reads a uint16 value in little-endian format
func readUint16LE(buf []byte, offset int) uint16 {
	if offset+2 > len(buf) {
		return 0
	}
	return binary.LittleEndian.Uint16(buf[offset:])
}

// readUint32BE reads a uint32 value in big-endian format
func readUint32BE(buf []byte, offset int) uint32 {
	if offset+4 > len(buf) {
		return 0
	}
	return binary.BigEndian.Uint32(buf[offset:])
}

// readUint16BE reads a uint16 value in big-endian format
func readUint16BE(buf []byte, offset int) uint16 {
	if offset+2 > len(buf) {
		return 0
	}
	return binary.BigEndian.Uint16(buf[offset:])
}

// readUint64BE reads a uint64 value in big-endian format
func readUint64BE(buf []byte, offset int) uint64 {
	if offset+8 > len(buf) {
		return 0
	}
	return binary.BigEndian.Uint64(buf[offset:])
}

// decodeEncodedValue decodes a kwil-db EncodedValue from bytes
func decodeEncodedValue(buf []byte) (any, error) {
	var encodedVal types.EncodedValue
	if err := encodedVal.UnmarshalBinary(buf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal EncodedValue: %w", err)
	}

	// Decode to Go native type
	value, err := encodedVal.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode EncodedValue: %w", err)
	}

	// Handle pointer types - dereference to get actual values
	if value == nil {
		return nil, nil
	}

	// Convert pointers to values for common types
	switch v := value.(type) {
	case *string:
		if v == nil {
			return nil, nil
		}
		return *v, nil
	case *int64:
		if v == nil {
			return nil, nil
		}
		return *v, nil
	case *bool:
		if v == nil {
			return nil, nil
		}
		return *v, nil
	case *[]byte:
		if v == nil {
			return nil, nil
		}
		return *v, nil
	default:
		// Return as-is for other types
		return value, nil
	}
}

// decodeCanonicalQueryResult decodes a canonical query result (rows/columns format)
//
// Format: [row_count: uint32 LE]
//         [row1_col_count: uint32 LE]
//           [col1_len: uint32 LE][encoded_col1]
//           [col2_len: uint32 LE][encoded_col2]
//           ...
//         [row2_col_count: uint32 LE]
//           ...
func decodeCanonicalQueryResult(data []byte) ([]sdktypes.DecodedRow, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("data too short for row count")
	}

	offset := 0

	// Read row count
	rowCount := readUint32LE(data, offset)
	if rowCount > maxRows {
		return nil, fmt.Errorf("row count %d exceeds maximum %d", rowCount, maxRows)
	}
	offset += 4

	rows := make([]sdktypes.DecodedRow, 0, rowCount)

	for i := uint32(0); i < rowCount; i++ {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("data too short for column count at row %d", i)
		}

		// Read column count for this row
		colCount := readUint32LE(data, offset)
		if colCount > maxColumns {
			return nil, fmt.Errorf("column count %d exceeds maximum %d at row %d", colCount, maxColumns, i)
		}
		offset += 4

		values := make([]any, 0, colCount)

		for j := uint32(0); j < colCount; j++ {
			if offset+4 > len(data) {
				return nil, fmt.Errorf("data too short for column %d length at row %d", j, i)
			}

			// Read column length
			colLen := readUint32LE(data, offset)
			offset += 4

			if offset+int(colLen) > len(data) {
				return nil, fmt.Errorf("data too short for column %d bytes at row %d", j, i)
			}

			// Extract column bytes
			colBytes := data[offset : offset+int(colLen)]

			// Decode the EncodedValue
			jsValue, err := decodeEncodedValue(colBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to decode column %d at row %d: %w", j, i, err)
			}

			values = append(values, jsValue)
			offset += int(colLen)
		}

		rows = append(rows, sdktypes.DecodedRow{Values: values})
	}

	return rows, nil
}

// formatFixedPoint formats a fixed-point integer value to decimal string
func formatFixedPoint(value *big.Int, decimals int) string {
	isNegative := value.Sign() < 0
	absValue := new(big.Int).Abs(value)

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	integerPart := new(big.Int).Div(absValue, divisor)
	fractionalPart := new(big.Int).Mod(absValue, divisor)

	// Pad fractional part with leading zeros
	fractionalStr := fractionalPart.String()
	if len(fractionalStr) < decimals {
		fractionalStr = strings.Repeat("0", decimals-len(fractionalStr)) + fractionalStr
	}

	// Remove trailing zeros from fractional part
	fractionalStr = strings.TrimRight(fractionalStr, "0")

	if fractionalStr == "" {
		if isNegative {
			return "-" + integerPart.String()
		}
		return integerPart.String()
	}

	if isNegative {
		return "-" + integerPart.String() + "." + fractionalStr
	}
	return integerPart.String() + "." + fractionalStr
}

// decodeABIDatapoints decodes ABI-encoded datapoints result (timestamps and values)
//
// Format: abi.encode(uint256[] timestamps, int256[] values)
//
// Returns an array of decoded rows with [timestamp, value] pairs
func decodeABIDatapoints(data []byte) ([]sdktypes.DecodedRow, error) {
	// Handle empty data
	if len(data) == 0 {
		return []sdktypes.DecodedRow{}, nil
	}

	// Define the ABI types
	uint256ArrayType, err := abi.NewType("uint256[]", "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create uint256[] type: %w", err)
	}

	int256ArrayType, err := abi.NewType("int256[]", "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create int256[] type: %w", err)
	}

	// Create arguments
	arguments := abi.Arguments{
		{Type: uint256ArrayType},
		{Type: int256ArrayType},
	}

	// Unpack the data
	unpacked, err := arguments.Unpack(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack ABI data: %w", err)
	}

	if len(unpacked) != 2 {
		return nil, fmt.Errorf("expected 2 arrays, got %d", len(unpacked))
	}

	// Extract timestamps and values
	timestamps, ok := unpacked[0].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("expected []*big.Int for timestamps, got %T", unpacked[0])
	}

	values, ok := unpacked[1].([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("expected []*big.Int for values, got %T", unpacked[1])
	}

	if len(timestamps) != len(values) {
		return nil, fmt.Errorf("timestamp/value array length mismatch: %d vs %d", len(timestamps), len(values))
	}

	// Build result rows
	rows := make([]sdktypes.DecodedRow, 0, len(timestamps))
	for i := 0; i < len(timestamps); i++ {
		row := sdktypes.DecodedRow{
			Values: []any{
				timestamps[i].String(),
				formatFixedPoint(values[i], 18), // 18-decimal fixed point
			},
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// ParseAttestationPayload parses a canonical attestation payload (without signature)
//
// Payload format:
// 1. Version (1 byte)
// 2. Algorithm (1 byte, 0 = secp256k1)
// 3. Block height (8 bytes, uint64 big-endian)
// 4. Data provider (length-prefixed with 4 bytes big-endian)
// 5. Stream ID (length-prefixed with 4 bytes big-endian)
// 6. Action ID (2 bytes, uint16 big-endian)
// 7. Arguments (length-prefixed with 4 bytes big-endian)
// 8. Result (length-prefixed with 4 bytes big-endian)
//
// Returns the parsed payload structure
func ParseAttestationPayload(payload []byte) (*sdktypes.ParsedAttestationPayload, error) {
	offset := 0

	// 1. Version (1 byte)
	if len(payload) < 1 {
		return nil, fmt.Errorf("payload too short for version")
	}
	version := payload[offset]
	offset++

	// 2. Algorithm (1 byte)
	if offset >= len(payload) {
		return nil, fmt.Errorf("payload too short for algorithm")
	}
	algorithm := payload[offset]
	offset++

	// 3. Block height (8 bytes, uint64 big-endian)
	if offset+8 > len(payload) {
		return nil, fmt.Errorf("payload too short for block height")
	}
	blockHeight := readUint64BE(payload, offset)
	offset += 8

	// 4. Data provider (length-prefixed, 4 bytes big-endian)
	if offset+4 > len(payload) {
		return nil, fmt.Errorf("payload too short for data provider length")
	}
	dataProviderLen := readUint32BE(payload, offset)
	offset += 4

	if offset+int(dataProviderLen) > len(payload) {
		return nil, fmt.Errorf("payload too short for data provider")
	}
	dataProviderBytes := payload[offset : offset+int(dataProviderLen)]

	// Data provider is typically a hex address (20 bytes for Ethereum address)
	// or a UTF-8 string
	var dataProvider string
	if dataProviderLen == 20 {
		// Likely an Ethereum address (20 bytes)
		dataProvider = fmt.Sprintf("0x%x", dataProviderBytes)
	} else {
		// Try UTF-8 decoding
		decoded := string(dataProviderBytes)
		// Check if it looks like a hex address string (starts with "0x")
		if len(decoded) > 2 && decoded[:2] == "0x" {
			dataProvider = decoded
		} else {
			// Assume it's a valid UTF-8 string
			dataProvider = decoded
		}
	}
	offset += int(dataProviderLen)

	// 5. Stream ID (length-prefixed, 4 bytes big-endian)
	if offset+4 > len(payload) {
		return nil, fmt.Errorf("payload too short for stream ID length")
	}
	streamIDLen := readUint32BE(payload, offset)
	offset += 4

	if offset+int(streamIDLen) > len(payload) {
		return nil, fmt.Errorf("payload too short for stream ID")
	}
	streamIDBytes := payload[offset : offset+int(streamIDLen)]
	streamID := string(streamIDBytes)
	offset += int(streamIDLen)

	// 6. Action ID (2 bytes, uint16 big-endian)
	if offset+2 > len(payload) {
		return nil, fmt.Errorf("payload too short for action ID")
	}
	actionID := readUint16BE(payload, offset)
	offset += 2

	// 7. Arguments (length-prefixed, 4 bytes big-endian)
	if offset+4 > len(payload) {
		return nil, fmt.Errorf("payload too short for arguments length")
	}
	argsLen := readUint32BE(payload, offset)
	offset += 4

	if offset+int(argsLen) > len(payload) {
		return nil, fmt.Errorf("payload too short for arguments")
	}
	argsBytes := payload[offset : offset+int(argsLen)]
	offset += int(argsLen)

	// Decode arguments
	args := []any{}
	if argsLen > 0 {
		argsOffset := 0

		// Arguments format: [arg_count: uint32 LE][length: uint32 LE][encoded_arg]...
		if len(argsBytes) < 4 {
			return nil, fmt.Errorf("arguments data too short for arg count")
		}
		argCount := readUint32LE(argsBytes, argsOffset)
		if argCount > maxArgs {
			return nil, fmt.Errorf("argument count %d exceeds maximum %d", argCount, maxArgs)
		}
		argsOffset += 4

		for i := uint32(0); i < argCount; i++ {
			if argsOffset+4 > len(argsBytes) {
				return nil, fmt.Errorf("arguments data too short for arg %d length", i)
			}
			argLen := readUint32LE(argsBytes, argsOffset)
			argsOffset += 4

			if argsOffset+int(argLen) > len(argsBytes) {
				return nil, fmt.Errorf("arguments data too short for arg %d bytes", i)
			}
			argBytes := argsBytes[argsOffset : argsOffset+int(argLen)]

			decodedArg, err := decodeEncodedValue(argBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to decode arg %d: %w", i, err)
			}

			args = append(args, decodedArg)
			argsOffset += int(argLen)
		}
	}

	// 8. Result (length-prefixed, 4 bytes big-endian)
	if offset+4 > len(payload) {
		return nil, fmt.Errorf("payload too short for result length")
	}
	resultLen := readUint32BE(payload, offset)
	offset += 4

	if offset+int(resultLen) > len(payload) {
		return nil, fmt.Errorf("payload too short for result")
	}
	resultBytes := payload[offset : offset+int(resultLen)]

	// Decode result (ABI-encoded as uint256[], int256[])
	result, err := decodeABIDatapoints(resultBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode result: %w", err)
	}

	return &sdktypes.ParsedAttestationPayload{
		Version:      version,
		Algorithm:    algorithm,
		BlockHeight:  blockHeight,
		DataProvider: dataProvider,
		StreamID:     streamID,
		ActionID:     actionID,
		Arguments:    args,
		Result:       result,
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// BINARY ACTION RESULT PARSING
// ═══════════════════════════════════════════════════════════════

// decodeABIBoolean decodes ABI-encoded boolean result
//
// Format: abi.encode(bool)
// This is a single bool packed into 32 bytes (left-padded with zeros)
func decodeABIBoolean(data []byte) (bool, error) {
	if len(data) == 0 {
		return false, fmt.Errorf("empty data for boolean result")
	}

	// ABI-encoded bool is 32 bytes (1 byte value, 31 bytes padding)
	if len(data) != 32 {
		return false, fmt.Errorf("expected 32 bytes for ABI-encoded bool, got %d", len(data))
	}

	// Define the ABI type
	boolType, err := abi.NewType("bool", "", nil)
	if err != nil {
		return false, fmt.Errorf("failed to create bool type: %w", err)
	}

	arguments := abi.Arguments{
		{Type: boolType},
	}

	// Unpack the data
	unpacked, err := arguments.Unpack(data)
	if err != nil {
		return false, fmt.Errorf("failed to unpack ABI bool: %w", err)
	}

	if len(unpacked) != 1 {
		return false, fmt.Errorf("expected 1 value, got %d", len(unpacked))
	}

	result, ok := unpacked[0].(bool)
	if !ok {
		return false, fmt.Errorf("expected bool, got %T", unpacked[0])
	}

	return result, nil
}

// ParseBooleanResult extracts a boolean result from a binary action attestation payload.
//
// This function is specifically for binary attestation actions (IDs 6-9):
//   - price_above_threshold (6)
//   - price_below_threshold (7)
//   - value_in_range (8)
//   - value_equals (9)
//
// These actions return abi.encode(bool) instead of abi.encode(uint256[], int256[]).
//
// Parameters:
//   - payload: The canonical attestation payload (without signature)
//
// Returns:
//   - result: The boolean outcome (TRUE/FALSE)
//   - actionID: The action ID from the payload (should be 6-9)
//   - err: Error if parsing fails or action is not a binary action
func ParseBooleanResult(payload []byte) (result bool, actionID uint16, err error) {
	// First, parse enough to get the action ID and result bytes
	// We'll do a partial parse to avoid fully decoding numeric results

	offset := 0

	// 1. Skip version (1 byte)
	if len(payload) < 1 {
		return false, 0, fmt.Errorf("payload too short for version")
	}
	offset++

	// 2. Skip algorithm (1 byte)
	if offset >= len(payload) {
		return false, 0, fmt.Errorf("payload too short for algorithm")
	}
	offset++

	// 3. Skip block height (8 bytes)
	if offset+8 > len(payload) {
		return false, 0, fmt.Errorf("payload too short for block height")
	}
	offset += 8

	// 4. Skip data provider (length-prefixed)
	if offset+4 > len(payload) {
		return false, 0, fmt.Errorf("payload too short for data provider length")
	}
	dataProviderLen := readUint32BE(payload, offset)
	if offset+4+int(dataProviderLen) > len(payload) {
		return false, 0, fmt.Errorf("payload too short for data provider content")
	}
	offset += 4 + int(dataProviderLen)

	// 5. Skip stream ID (length-prefixed)
	if offset+4 > len(payload) {
		return false, 0, fmt.Errorf("payload too short for stream ID length")
	}
	streamIDLen := readUint32BE(payload, offset)
	if offset+4+int(streamIDLen) > len(payload) {
		return false, 0, fmt.Errorf("payload too short for stream ID content")
	}
	offset += 4 + int(streamIDLen)

	// 6. Read action ID (2 bytes)
	if offset+2 > len(payload) {
		return false, 0, fmt.Errorf("payload too short for action ID")
	}
	actionID = readUint16BE(payload, offset)
	offset += 2

	// Validate this is a binary action
	if !BinaryActionResultIDs[actionID] {
		return false, actionID, fmt.Errorf("action ID %d is not a binary action (expected 6-9)", actionID)
	}

	// 7. Skip arguments (length-prefixed)
	if offset+4 > len(payload) {
		return false, actionID, fmt.Errorf("payload too short for arguments length")
	}
	argsLen := readUint32BE(payload, offset)
	if offset+4+int(argsLen) > len(payload) {
		return false, actionID, fmt.Errorf("payload too short for arguments content")
	}
	offset += 4 + int(argsLen)

	// 8. Read result (length-prefixed)
	if offset+4 > len(payload) {
		return false, actionID, fmt.Errorf("payload too short for result length")
	}
	resultLen := readUint32BE(payload, offset)
	offset += 4

	if offset+int(resultLen) > len(payload) {
		return false, actionID, fmt.Errorf("payload too short for result")
	}
	resultBytes := payload[offset : offset+int(resultLen)]

	// Decode the boolean result
	result, err = decodeABIBoolean(resultBytes)
	if err != nil {
		return false, actionID, fmt.Errorf("failed to decode boolean result: %w", err)
	}

	return result, actionID, nil
}

// ParseBooleanResultFromParsed extracts a boolean result from an already-parsed attestation.
// This is useful when you've already called ParseAttestationPayload and want to interpret
// the result as a boolean.
//
// Note: This function attempts to re-interpret the Result field. For binary actions,
// the Result will be empty (decodeABIDatapoints can't parse abi.encode(bool)).
// Use ParseBooleanResult with raw payload instead for binary actions.
func ParseBooleanResultFromParsed(parsed *sdktypes.ParsedAttestationPayload) (bool, error) {
	if parsed == nil {
		return false, fmt.Errorf("parsed payload is nil")
	}

	// Validate this is a binary action
	if !BinaryActionResultIDs[parsed.ActionID] {
		return false, fmt.Errorf("action ID %d is not a binary action (expected 6-9)", parsed.ActionID)
	}

	// For binary actions, the Result field may be empty or incorrectly parsed
	// because decodeABIDatapoints expects uint256[]/int256[] format
	if len(parsed.Result) == 0 {
		return false, fmt.Errorf("no result in parsed payload (use ParseBooleanResult with raw payload for binary actions)")
	}

	// If result was somehow parsed, try to extract boolean
	if len(parsed.Result[0].Values) == 0 {
		return false, fmt.Errorf("no values in first result row")
	}

	// Try to interpret the value as boolean
	switch v := parsed.Result[0].Values[0].(type) {
	case bool:
		return v, nil
	case string:
		// Sometimes boolean might be encoded as "true"/"false" string
		if v == "true" || v == "1" {
			return true, nil
		}
		if v == "false" || v == "0" {
			return false, nil
		}
		return false, fmt.Errorf("cannot interpret string %q as boolean", v)
	default:
		return false, fmt.Errorf("unexpected result type: %T", v)
	}
}

// IsBinaryActionResult returns true if the action ID corresponds to a binary action
func IsBinaryActionResult(actionID uint16) bool {
	return BinaryActionResultIDs[actionID]
}
