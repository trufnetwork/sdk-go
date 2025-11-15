package contractsapi

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/kwil-db/core/types"
)

// Test binary reading helpers
func TestReadUint32LE(t *testing.T) {
	testCases := []struct {
		name     string
		input    []byte
		offset   int
		expected uint32
	}{
		{"basic", []byte{0x78, 0x56, 0x34, 0x12}, 0, 0x12345678},
		{"zero", []byte{0x00, 0x00, 0x00, 0x00}, 0, 0},
		{"max", []byte{0xFF, 0xFF, 0xFF, 0xFF}, 0, 0xFFFFFFFF},
		{"with offset", []byte{0xAA, 0xBB, 0x78, 0x56, 0x34, 0x12}, 2, 0x12345678},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := readUint32LE(tc.input, tc.offset)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestReadUint32BE(t *testing.T) {
	testCases := []struct {
		name     string
		input    []byte
		offset   int
		expected uint32
	}{
		{"basic", []byte{0x12, 0x34, 0x56, 0x78}, 0, 0x12345678},
		{"zero", []byte{0x00, 0x00, 0x00, 0x00}, 0, 0},
		{"max", []byte{0xFF, 0xFF, 0xFF, 0xFF}, 0, 0xFFFFFFFF},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := readUint32BE(tc.input, tc.offset)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestReadUint16LE(t *testing.T) {
	testCases := []struct {
		name     string
		input    []byte
		offset   int
		expected uint16
	}{
		{"basic", []byte{0x78, 0x56}, 0, 0x5678},
		{"zero", []byte{0x00, 0x00}, 0, 0},
		{"max", []byte{0xFF, 0xFF}, 0, 0xFFFF},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := readUint16LE(tc.input, tc.offset)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestReadUint16BE(t *testing.T) {
	testCases := []struct {
		name     string
		input    []byte
		offset   int
		expected uint16
	}{
		{"basic", []byte{0x56, 0x78}, 0, 0x5678},
		{"zero", []byte{0x00, 0x00}, 0, 0},
		{"max", []byte{0xFF, 0xFF}, 0, 0xFFFF},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := readUint16BE(tc.input, tc.offset)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestReadUint64BE(t *testing.T) {
	testCases := []struct {
		name     string
		input    []byte
		offset   int
		expected uint64
	}{
		{"basic", []byte{0x00, 0x00, 0x00, 0x00, 0x12, 0x34, 0x56, 0x78}, 0, 0x12345678},
		{"zero", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0, 0},
		{"max", []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, 0, 0xFFFFFFFFFFFFFFFF},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := readUint64BE(tc.input, tc.offset)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test EncodedValue decoding
func TestDecodeEncodedValue(t *testing.T) {
	t.Run("string value", func(t *testing.T) {
		// Create an EncodedValue for a string
		encodedVal, err := types.EncodeValue("test string")
		require.NoError(t, err)

		bytes, err := encodedVal.MarshalBinary()
		require.NoError(t, err)

		// Decode it
		decoded, err := decodeEncodedValue(bytes)
		require.NoError(t, err)

		assert.Equal(t, "test string", decoded)
	})

	t.Run("int64 value", func(t *testing.T) {
		encodedVal, err := types.EncodeValue(int64(42))
		require.NoError(t, err)

		bytes, err := encodedVal.MarshalBinary()
		require.NoError(t, err)

		decoded, err := decodeEncodedValue(bytes)
		require.NoError(t, err)

		assert.Equal(t, int64(42), decoded)
	})

	t.Run("boolean value", func(t *testing.T) {
		encodedVal, err := types.EncodeValue(true)
		require.NoError(t, err)

		bytes, err := encodedVal.MarshalBinary()
		require.NoError(t, err)

		decoded, err := decodeEncodedValue(bytes)
		require.NoError(t, err)

		assert.Equal(t, true, decoded)
	})

	t.Run("nil value", func(t *testing.T) {
		encodedVal, err := types.EncodeValue(nil)
		require.NoError(t, err)

		bytes, err := encodedVal.MarshalBinary()
		require.NoError(t, err)

		decoded, err := decodeEncodedValue(bytes)
		require.NoError(t, err)

		assert.Nil(t, decoded)
	})
}

// Test formatFixedPoint
func TestFormatFixedPoint(t *testing.T) {
	testCases := []struct {
		name     string
		value    string // Use string to create big.Int
		decimals int
		expected string
	}{
		{"positive integer", "1000000000000000000", 18, "1"},
		{"positive decimal", "1500000000000000000", 18, "1.5"},
		{"negative integer", "-1000000000000000000", 18, "-1"},
		{"negative decimal", "-1500000000000000000", 18, "-1.5"},
		{"zero", "0", 18, "0"},
		{"small value", "123456789012345678", 18, "0.123456789012345678"},
		{"trailing zeros removed", "1230000000000000000", 18, "1.23"},
		{"large value", "77051806494788211665", 18, "77.051806494788211665"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := new(big.Int).SetString(tc.value, 10)
			require.True(t, ok, "failed to parse test value")

			result := formatFixedPoint(value, tc.decimals)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test decodeABIDatapoints
func TestDecodeABIDatapoints(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		result, err := decodeABIDatapoints([]byte{})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("single datapoint", func(t *testing.T) {
		// Create ABI types
		uint256ArrayType, err := abi.NewType("uint256[]", "", nil)
		require.NoError(t, err)
		int256ArrayType, err := abi.NewType("int256[]", "", nil)
		require.NoError(t, err)

		arguments := abi.Arguments{
			{Type: uint256ArrayType},
			{Type: int256ArrayType},
		}

		// Create test data: single timestamp and value
		timestamp := big.NewInt(1704067200) // Jan 1, 2024
		value := new(big.Int)
		value.SetString("77051806494788211665", 10) // 77.051806494788211665 with 18 decimals

		timestamps := []*big.Int{timestamp}
		values := []*big.Int{value}

		// Pack the data
		packed, err := arguments.Pack(timestamps, values)
		require.NoError(t, err)

		// Decode it
		result, err := decodeABIDatapoints(packed)
		require.NoError(t, err)

		assert.Len(t, result, 1)
		assert.Len(t, result[0].Values, 2)
		assert.Equal(t, "1704067200", result[0].Values[0])
		assert.Equal(t, "77.051806494788211665", result[0].Values[1])
	})

	t.Run("multiple datapoints", func(t *testing.T) {
		uint256ArrayType, err := abi.NewType("uint256[]", "", nil)
		require.NoError(t, err)
		int256ArrayType, err := abi.NewType("int256[]", "", nil)
		require.NoError(t, err)

		arguments := abi.Arguments{
			{Type: uint256ArrayType},
			{Type: int256ArrayType},
		}

		timestamps := []*big.Int{
			big.NewInt(1704067200),
			big.NewInt(1704153600),
			big.NewInt(1704240000),
		}

		value1, _ := new(big.Int).SetString("77051806494788211665", 10)
		value2, _ := new(big.Int).SetString("80000000000000000000", 10)
		value3, _ := new(big.Int).SetString("75500000000000000000", 10)

		values := []*big.Int{value1, value2, value3}

		packed, err := arguments.Pack(timestamps, values)
		require.NoError(t, err)

		result, err := decodeABIDatapoints(packed)
		require.NoError(t, err)

		assert.Len(t, result, 3)
		assert.Equal(t, "1704067200", result[0].Values[0])
		assert.Equal(t, "77.051806494788211665", result[0].Values[1])
		assert.Equal(t, "1704153600", result[1].Values[0])
		assert.Equal(t, "80", result[1].Values[1])
	})

	t.Run("negative value", func(t *testing.T) {
		uint256ArrayType, err := abi.NewType("uint256[]", "", nil)
		require.NoError(t, err)
		int256ArrayType, err := abi.NewType("int256[]", "", nil)
		require.NoError(t, err)

		arguments := abi.Arguments{
			{Type: uint256ArrayType},
			{Type: int256ArrayType},
		}

		timestamps := []*big.Int{big.NewInt(1704067200)}
		values := []*big.Int{big.NewInt(-1000000000000000000)} // -1.0

		packed, err := arguments.Pack(timestamps, values)
		require.NoError(t, err)

		result, err := decodeABIDatapoints(packed)
		require.NoError(t, err)

		assert.Len(t, result, 1)
		assert.Equal(t, "1704067200", result[0].Values[0])
		assert.Equal(t, "-1", result[0].Values[1])
	})
}

// Test ParseAttestationPayload
func TestParseAttestationPayload(t *testing.T) {
	t.Run("minimal valid payload", func(t *testing.T) {
		// Build a minimal valid payload manually
		payload := []byte{}

		// Version (1 byte)
		payload = append(payload, 0x01)

		// Algorithm (1 byte) - secp256k1
		payload = append(payload, 0x00)

		// Block height (8 bytes BE)
		blockHeight := uint64(1234567)
		heightBytes := make([]byte, 8)
		for i := 7; i >= 0; i-- {
			heightBytes[i] = byte(blockHeight & 0xFF)
			blockHeight >>= 8
		}
		payload = append(payload, heightBytes...)

		// Data provider (20 bytes with length prefix)
		dataProvider := make([]byte, 20)
		for i := range dataProvider {
			dataProvider[i] = byte(i + 1)
		}
		payload = append(payload, 0x00, 0x00, 0x00, 0x14) // length = 20
		payload = append(payload, dataProvider...)

		// Stream ID (32 bytes with length prefix)
		streamID := []byte("stai0000000000000000000000000000")
		payload = append(payload, 0x00, 0x00, 0x00, 0x20) // length = 32
		payload = append(payload, streamID...)

		// Action ID (2 bytes BE)
		payload = append(payload, 0x00, 0x01) // action_id = 1

		// Arguments (empty, with length prefix)
		payload = append(payload, 0x00, 0x00, 0x00, 0x00) // length = 0

		// Result (empty ABI-encoded arrays)
		uint256ArrayType, _ := abi.NewType("uint256[]", "", nil)
		int256ArrayType, _ := abi.NewType("int256[]", "", nil)
		arguments := abi.Arguments{
			{Type: uint256ArrayType},
			{Type: int256ArrayType},
		}
		emptyResult, _ := arguments.Pack([]*big.Int{}, []*big.Int{})
		resultLen := uint32(len(emptyResult))
		payload = append(payload, byte(resultLen>>24), byte(resultLen>>16), byte(resultLen>>8), byte(resultLen))
		payload = append(payload, emptyResult...)

		// Parse it
		parsed, err := ParseAttestationPayload(payload)
		require.NoError(t, err)

		assert.Equal(t, uint8(1), parsed.Version)
		assert.Equal(t, uint8(0), parsed.Algorithm)
		assert.Equal(t, uint64(1234567), parsed.BlockHeight)
		assert.Equal(t, "0x0102030405060708090a0b0c0d0e0f1011121314", parsed.DataProvider)
		assert.Equal(t, "stai0000000000000000000000000000", parsed.StreamID)
		assert.Equal(t, uint16(1), parsed.ActionID)
		assert.Empty(t, parsed.Arguments)
		assert.Empty(t, parsed.Result)
	})

	t.Run("payload too short", func(t *testing.T) {
		payload := []byte{0x01} // Only version
		_, err := ParseAttestationPayload(payload)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too short")
	})

	t.Run("with arguments", func(t *testing.T) {
		// Build payload with arguments
		payload := []byte{}
		payload = append(payload, 0x01) // Version
		payload = append(payload, 0x00) // Algorithm

		// Block height
		payload = append(payload, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12, 0xD6, 0x87) // 1234567

		// Data provider (20 bytes)
		dataProvider := make([]byte, 20)
		payload = append(payload, 0x00, 0x00, 0x00, 0x14)
		payload = append(payload, dataProvider...)

		// Stream ID
		streamID := []byte("stai0000000000000000000000000000")
		payload = append(payload, 0x00, 0x00, 0x00, 0x20)
		payload = append(payload, streamID...)

		// Action ID
		payload = append(payload, 0x00, 0x01)

		// Arguments: encode a single string "test"
		encodedArg, err := types.EncodeValue("test")
		require.NoError(t, err)
		argBytes, err := encodedArg.MarshalBinary()
		require.NoError(t, err)

		// Arguments format: [arg_count: uint32 LE][length: uint32 LE][encoded_arg]
		argsData := []byte{}
		// arg_count = 1 (little-endian)
		argsData = append(argsData, 0x01, 0x00, 0x00, 0x00)
		// arg length (little-endian)
		argLen := uint32(len(argBytes))
		argsData = append(argsData, byte(argLen), byte(argLen>>8), byte(argLen>>16), byte(argLen>>24))
		// arg bytes
		argsData = append(argsData, argBytes...)

		// Add arguments to payload with big-endian length prefix
		argsLen := uint32(len(argsData))
		payload = append(payload, byte(argsLen>>24), byte(argsLen>>16), byte(argsLen>>8), byte(argsLen))
		payload = append(payload, argsData...)

		// Result (empty)
		uint256ArrayType, _ := abi.NewType("uint256[]", "", nil)
		int256ArrayType, _ := abi.NewType("int256[]", "", nil)
		arguments := abi.Arguments{
			{Type: uint256ArrayType},
			{Type: int256ArrayType},
		}
		emptyResult, _ := arguments.Pack([]*big.Int{}, []*big.Int{})
		resultLen := uint32(len(emptyResult))
		payload = append(payload, byte(resultLen>>24), byte(resultLen>>16), byte(resultLen>>8), byte(resultLen))
		payload = append(payload, emptyResult...)

		// Parse it
		parsed, err := ParseAttestationPayload(payload)
		require.NoError(t, err)

		assert.Len(t, parsed.Arguments, 1)
		assert.Equal(t, "test", parsed.Arguments[0])
	})
}

// Test with real-world hex data (if available from examples)
func TestParseAttestationPayload_RealData(t *testing.T) {
	t.Run("decode hex data if provided", func(t *testing.T) {
		// This test can be populated with real hex data from the network
		// For now, skip if no data is available
		t.Skip("Populate with real attestation payload hex data for integration testing")

		// Example usage:
		// hexData := "01000000000000..." // Real hex from network
		// payloadBytes, err := hex.DecodeString(hexData)
		// require.NoError(t, err)
		//
		// parsed, err := ParseAttestationPayload(payloadBytes)
		// require.NoError(t, err)
		// assert.NotNil(t, parsed)
	})
}

// Benchmark tests
func BenchmarkParseAttestationPayload(b *testing.B) {
	// Build a realistic payload for benchmarking
	payload := []byte{}
	payload = append(payload, 0x01, 0x00) // version, algorithm

	// Block height
	payload = append(payload, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12, 0xD6, 0x87)

	// Data provider
	dataProvider := make([]byte, 20)
	payload = append(payload, 0x00, 0x00, 0x00, 0x14)
	payload = append(payload, dataProvider...)

	// Stream ID
	streamID := []byte("stai0000000000000000000000000000")
	payload = append(payload, 0x00, 0x00, 0x00, 0x20)
	payload = append(payload, streamID...)

	// Action ID
	payload = append(payload, 0x00, 0x01)

	// Empty arguments
	payload = append(payload, 0x00, 0x00, 0x00, 0x00)

	// Result with 10 datapoints
	uint256ArrayType, _ := abi.NewType("uint256[]", "", nil)
	int256ArrayType, _ := abi.NewType("int256[]", "", nil)
	arguments := abi.Arguments{
		{Type: uint256ArrayType},
		{Type: int256ArrayType},
	}

	timestamps := make([]*big.Int, 10)
	values := make([]*big.Int, 10)
	for i := 0; i < 10; i++ {
		timestamps[i] = big.NewInt(int64(1704067200 + i*86400))
		values[i], _ = new(big.Int).SetString("77051806494788211665", 10)
	}

	result, _ := arguments.Pack(timestamps, values)
	resultLen := uint32(len(result))
	payload = append(payload, byte(resultLen>>24), byte(resultLen>>16), byte(resultLen>>8), byte(resultLen))
	payload = append(payload, result...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseAttestationPayload(payload)
	}
}

func BenchmarkDecodeABIDatapoints(b *testing.B) {
	uint256ArrayType, _ := abi.NewType("uint256[]", "", nil)
	int256ArrayType, _ := abi.NewType("int256[]", "", nil)
	arguments := abi.Arguments{
		{Type: uint256ArrayType},
		{Type: int256ArrayType},
	}

	timestamps := make([]*big.Int, 100)
	values := make([]*big.Int, 100)
	for i := 0; i < 100; i++ {
		timestamps[i] = big.NewInt(int64(1704067200 + i*86400))
		values[i], _ = new(big.Int).SetString("77051806494788211665", 10)
	}

	packed, _ := arguments.Pack(timestamps, values)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = decodeABIDatapoints(packed)
	}
}
