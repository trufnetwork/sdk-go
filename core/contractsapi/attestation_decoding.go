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
