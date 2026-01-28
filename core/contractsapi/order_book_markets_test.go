package contractsapi

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════
// ENCODE QUERY COMPONENTS TESTS
// ═══════════════════════════════════════════════════════════════

func TestEncodeQueryComponents(t *testing.T) {
	dataProvider := "0x1111111111111111111111111111111111111111"
	streamID := "stbtcusd000000000000000000000000" // Exactly 32 chars
	actionID := "get_record"
	args := []byte{0x00, 0x00, 0x00, 0x20} // Simple ABI-encoded bytes

	encoded, err := EncodeQueryComponents(dataProvider, streamID, actionID, args)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// Verify the encoded data can be decoded back
	addr := common.HexToAddress(dataProvider)
	var streamBytes [32]byte
	copy(streamBytes[:], []byte(streamID))

	// Define ABI type for decoding
	addressType, err := abi.NewType("address", "", nil)
	require.NoError(t, err)
	bytes32Type, err := abi.NewType("bytes32", "", nil)
	require.NoError(t, err)
	stringType, err := abi.NewType("string", "", nil)
	require.NoError(t, err)
	bytesType, err := abi.NewType("bytes", "", nil)
	require.NoError(t, err)

	abiArgs := abi.Arguments{
		{Type: addressType},
		{Type: bytes32Type},
		{Type: stringType},
		{Type: bytesType},
	}

	// Unpack and verify
	unpacked, err := abiArgs.Unpack(encoded)
	require.NoError(t, err)
	require.Len(t, unpacked, 4)

	// Verify address
	unpackedAddr, ok := unpacked[0].(common.Address)
	require.True(t, ok)
	require.Equal(t, addr, unpackedAddr)

	// Verify stream ID (bytes32)
	unpackedStreamBytes, ok := unpacked[1].([32]byte)
	require.True(t, ok)
	require.Equal(t, streamBytes, unpackedStreamBytes)

	// Verify action ID
	unpackedActionID, ok := unpacked[2].(string)
	require.True(t, ok)
	require.Equal(t, actionID, unpackedActionID)

	// Verify args
	unpackedArgs, ok := unpacked[3].([]byte)
	require.True(t, ok)
	require.Equal(t, args, unpackedArgs)
}

func TestEncodeQueryComponents_InvalidAddress(t *testing.T) {
	tests := []struct {
		name         string
		dataProvider string
		expectError  string
	}{
		{
			name:         "Not hex address - too short",
			dataProvider: "invalid",
			expectError:  "data_provider must be 42 characters",
		},
		{
			name:         "Too short",
			dataProvider: "0x1111",
			expectError:  "data_provider must be 42 characters",
		},
		{
			name:         "Too long",
			dataProvider: "0x11111111111111111111111111111111111111111",
			expectError:  "data_provider must be 42 characters",
		},
		{
			name:         "Missing 0x prefix",
			dataProvider: "1111111111111111111111111111111111111111",
			expectError:  "data_provider must be 42 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeQueryComponents(tt.dataProvider, "stream00000000000000000000000000", "get_record", []byte{})
			require.Error(t, err)
			require.Nil(t, encoded)
			require.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestEncodeQueryComponents_InvalidStreamID(t *testing.T) {
	tests := []struct {
		name        string
		streamID    string
		expectError string
	}{
		{
			name:        "Too short",
			streamID:    "btc",
			expectError: "stream_id must be exactly 32 characters",
		},
		{
			name:        "Too long",
			streamID:    "stbtcusd0000000000000000000000000", // 33 chars
			expectError: "stream_id must be exactly 32 characters",
		},
		{
			name:        "Empty",
			streamID:    "",
			expectError: "stream_id must be exactly 32 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeQueryComponents("0x1111111111111111111111111111111111111111", tt.streamID, "get_record", []byte{})
			require.Error(t, err)
			require.Nil(t, encoded)
			require.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestEncodeQueryComponents_InvalidActionID(t *testing.T) {
	encoded, err := EncodeQueryComponents("0x1111111111111111111111111111111111111111", "stbtcusd000000000000000000000000", "", []byte{})
	require.Error(t, err)
	require.Nil(t, encoded)
	require.Contains(t, err.Error(), "action_id cannot be empty")
}

func TestEncodeQueryComponents_DifferentActionIDs(t *testing.T) {
	dataProvider := "0x1111111111111111111111111111111111111111"
	streamID := "stbtcusd000000000000000000000000"
	args := []byte{0x00}

	actionIDs := []string{
		"get_record",
		"get_index",
		"get_change_over_time",
		"get_last_record",
		"get_first_record",
		"price_above_threshold",
		"price_below_threshold",
		"value_in_range",
		"value_equals",
	}

	for _, actionID := range actionIDs {
		t.Run(actionID, func(t *testing.T) {
			encoded, err := EncodeQueryComponents(dataProvider, streamID, actionID, args)
			require.NoError(t, err)
			require.NotEmpty(t, encoded)

			// Verify action ID is correctly encoded
			addressType, _ := abi.NewType("address", "", nil)
			bytes32Type, _ := abi.NewType("bytes32", "", nil)
			stringType, _ := abi.NewType("string", "", nil)
			bytesType, _ := abi.NewType("bytes", "", nil)

			abiArgs := abi.Arguments{
				{Type: addressType},
				{Type: bytes32Type},
				{Type: stringType},
				{Type: bytesType},
			}

			unpacked, err := abiArgs.Unpack(encoded)
			require.NoError(t, err)

			unpackedActionID, ok := unpacked[2].(string)
			require.True(t, ok)
			require.Equal(t, actionID, unpackedActionID)
		})
	}
}

func TestEncodeQueryComponents_ComplexArgs(t *testing.T) {
	dataProvider := "0x1111111111111111111111111111111111111111"
	streamID := "stbtcusd000000000000000000000000"
	actionID := "get_record"

	tests := []struct {
		name string
		args []byte
	}{
		{
			name: "Empty args",
			args: []byte{},
		},
		{
			name: "Single byte",
			args: []byte{0xFF},
		},
		{
			name: "Multiple bytes",
			args: []byte{0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x00, 0x40},
		},
		{
			name: "Large args",
			args: make([]byte, 1024), // 1KB of zeros
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeQueryComponents(dataProvider, streamID, actionID, tt.args)
			require.NoError(t, err)
			require.NotEmpty(t, encoded)

			// Verify args are correctly encoded
			addressType, _ := abi.NewType("address", "", nil)
			bytes32Type, _ := abi.NewType("bytes32", "", nil)
			stringType, _ := abi.NewType("string", "", nil)
			bytesType, _ := abi.NewType("bytes", "", nil)

			abiArgs := abi.Arguments{
				{Type: addressType},
				{Type: bytes32Type},
				{Type: stringType},
				{Type: bytesType},
			}

			unpacked, err := abiArgs.Unpack(encoded)
			require.NoError(t, err)

			unpackedArgs, ok := unpacked[3].([]byte)
			require.True(t, ok)
			require.Equal(t, tt.args, unpackedArgs)
		})
	}
}

func TestEncodeQueryComponents_Deterministic(t *testing.T) {
	dataProvider := "0x1111111111111111111111111111111111111111"
	streamID := "stbtcusd000000000000000000000000"
	actionID := "get_record"
	args := []byte{0x00, 0x00, 0x00, 0x20}

	// Encode multiple times and verify same result
	encoded1, err := EncodeQueryComponents(dataProvider, streamID, actionID, args)
	require.NoError(t, err)

	encoded2, err := EncodeQueryComponents(dataProvider, streamID, actionID, args)
	require.NoError(t, err)

	encoded3, err := EncodeQueryComponents(dataProvider, streamID, actionID, args)
	require.NoError(t, err)

	require.Equal(t, encoded1, encoded2)
	require.Equal(t, encoded2, encoded3)
}

func TestEncodeQueryComponents_DifferentInputsDifferentOutput(t *testing.T) {
	baseProvider := "0x1111111111111111111111111111111111111111"
	baseStreamID := "stbtcusd000000000000000000000000"
	baseActionID := "get_record"
	baseArgs := []byte{0x00}

	base, err := EncodeQueryComponents(baseProvider, baseStreamID, baseActionID, baseArgs)
	require.NoError(t, err)

	// Different provider
	diffProvider, err := EncodeQueryComponents("0x2222222222222222222222222222222222222222", baseStreamID, baseActionID, baseArgs)
	require.NoError(t, err)
	require.NotEqual(t, base, diffProvider)

	// Different stream ID (must be exactly 32 chars)
	diffStream, err := EncodeQueryComponents(baseProvider, "ethusd00000000000000000000000000", baseActionID, baseArgs)
	require.NoError(t, err)
	require.NotEqual(t, base, diffStream)

	// Different action ID
	diffAction, err := EncodeQueryComponents(baseProvider, baseStreamID, "get_index", baseArgs)
	require.NoError(t, err)
	require.NotEqual(t, base, diffAction)

	// Different args
	diffArgs, err := EncodeQueryComponents(baseProvider, baseStreamID, baseActionID, []byte{0xFF})
	require.NoError(t, err)
	require.NotEqual(t, base, diffArgs)
}

func TestEncodeQueryComponents_ChecksumAddress(t *testing.T) {
	// Test with both checksummed and non-checksummed addresses
	tests := []struct {
		name    string
		address string
	}{
		{
			name:    "All lowercase",
			address: "0x1111111111111111111111111111111111111111",
		},
		{
			name:    "Checksummed address",
			address: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		},
		{
			name:    "All uppercase hex",
			address: "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeQueryComponents(tt.address, "stream00000000000000000000000000", "get_record", []byte{})
			require.NoError(t, err)
			require.NotEmpty(t, encoded)

			// Verify address is correctly encoded
			addressType, _ := abi.NewType("address", "", nil)
			bytes32Type, _ := abi.NewType("bytes32", "", nil)
			stringType, _ := abi.NewType("string", "", nil)
			bytesType, _ := abi.NewType("bytes", "", nil)

			abiArgs := abi.Arguments{
				{Type: addressType},
				{Type: bytes32Type},
				{Type: stringType},
				{Type: bytesType},
			}

			unpacked, err := abiArgs.Unpack(encoded)
			require.NoError(t, err)

			unpackedAddr, ok := unpacked[0].(common.Address)
			require.True(t, ok)

			expectedAddr := common.HexToAddress(tt.address)
			require.Equal(t, expectedAddr, unpackedAddr)
		})
	}
}

func TestDecodeQueryComponents(t *testing.T) {
	// First encode
	dataProvider := "0x1111111111111111111111111111111111111111"
	streamID := "stbtcusd000000000000000000000000"
	actionID := "get_record"
	args := []byte{0x00, 0x01, 0x02, 0x03}

	encoded, err := EncodeQueryComponents(dataProvider, streamID, actionID, args)
	require.NoError(t, err)

	// Then decode
	decodedProvider, decodedStreamID, decodedActionID, decodedArgs, err := DecodeQueryComponents(encoded)
	require.NoError(t, err)

	// Verify all fields (note: address will be checksummed)
	require.Equal(t, common.HexToAddress(dataProvider).Hex(), decodedProvider)
	require.Equal(t, streamID, decodedStreamID)
	require.Equal(t, actionID, decodedActionID)
	require.Equal(t, args, decodedArgs)
}
