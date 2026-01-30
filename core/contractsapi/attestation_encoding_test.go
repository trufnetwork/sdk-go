package contractsapi

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/kwil-db/core/types"
)

func TestEncodeActionArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []any
		wantErr bool
	}{
		{
			name: "empty args",
			args: []any{},
		},
		{
			name: "single string arg",
			args: []any{"test"},
		},
		{
			name: "multiple types",
			args: []any{
				"provider",
				"stream",
				int64(100),
				int64(200),
				nil,
				false,
			},
		},
		{
			name: "with bytes",
			args: []any{
				[]byte("test"),
				"string",
				int64(42),
			},
		},
		{
			name: "all supported types",
			args: []any{
				"string",
				int64(123),
				true,
				false,
				nil,
				[]byte("bytes"),
			},
		},
		{
			name: "unsupported type",
			args: []any{
				map[string]int{"key": 1}, // maps are not supported
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeActionArgs(tt.args)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, encoded)

			// Verify the format
			buf := bytes.NewReader(encoded)

			// Read arg count
			var argCount uint32
			err = binary.Read(buf, binary.LittleEndian, &argCount)
			require.NoError(t, err)
			assert.Equal(t, uint32(len(tt.args)), argCount, "arg count should match")

			// Verify each argument can be read
			for i := 0; i < len(tt.args); i++ {
				var argLen uint32
				err = binary.Read(buf, binary.LittleEndian, &argLen)
				require.NoError(t, err, "should read arg %d length", i)

				argBytes := make([]byte, argLen)
				_, err = buf.Read(argBytes)
				require.NoError(t, err, "should read arg %d bytes", i)

				// Verify the bytes can be unmarshaled as EncodedValue
				var encodedVal types.EncodedValue
				err = encodedVal.UnmarshalBinary(argBytes)
				require.NoError(t, err, "arg %d should unmarshal as EncodedValue", i)

				// Verify the value can be decoded
				_, err = encodedVal.Decode()
				require.NoError(t, err, "arg %d should decode", i)
			}

			// Verify we've read all bytes
			remaining := buf.Len()
			assert.Equal(t, 0, remaining, "should have no remaining bytes")
		})
	}
}

func TestEncodeActionArgs_RoundTrip(t *testing.T) {
	// Test that we can encode and decode the same arguments
	originalArgs := []any{
		"0x4710a8d8f0d845da110086812a32de6d90d7ff5c",
		"stai0000000000000000000000000000",
		int64(1000000),
		int64(2000000),
		nil,
		false,
		[]byte("test bytes"),
	}

	// Encode
	encoded, err := EncodeActionArgs(originalArgs)
	require.NoError(t, err)

	// Decode using similar logic to node's DecodeActionArgs
	buf := bytes.NewReader(encoded)

	// Read arg count
	var argCount uint32
	err = binary.Read(buf, binary.LittleEndian, &argCount)
	require.NoError(t, err)
	assert.Equal(t, uint32(len(originalArgs)), argCount)

	decodedArgs := make([]any, argCount)

	// Decode each argument
	for i := uint32(0); i < argCount; i++ {
		var argLen uint32
		err = binary.Read(buf, binary.LittleEndian, &argLen)
		require.NoError(t, err)

		argBytes := make([]byte, argLen)
		_, err = buf.Read(argBytes)
		require.NoError(t, err)

		var encodedVal types.EncodedValue
		err = encodedVal.UnmarshalBinary(argBytes)
		require.NoError(t, err)

		decodedVal, err := encodedVal.Decode()
		require.NoError(t, err)

		decodedArgs[i] = decodedVal
	}

	// Verify values match (note: types might differ slightly, e.g., int64 vs *int64)
	assert.Len(t, decodedArgs, len(originalArgs))

	for i := range originalArgs {
		if originalArgs[i] == nil {
			assert.Nil(t, decodedArgs[i], "arg %d should be nil", i)
		} else {
			// Type assertions for common types
			switch v := originalArgs[i].(type) {
			case string:
				decoded, ok := decodedArgs[i].(string)
				if !ok {
					// Try pointer
					decodedPtr, ok := decodedArgs[i].(*string)
					require.True(t, ok, "arg %d should be string or *string", i)
					require.NotNil(t, decodedPtr)
					assert.Equal(t, v, *decodedPtr)
				} else {
					assert.Equal(t, v, decoded)
				}
			case int64:
				decoded, ok := decodedArgs[i].(int64)
				if !ok {
					// Try pointer
					decodedPtr, ok := decodedArgs[i].(*int64)
					require.True(t, ok, "arg %d should be int64 or *int64", i)
					require.NotNil(t, decodedPtr)
					assert.Equal(t, v, *decodedPtr)
				} else {
					assert.Equal(t, v, decoded)
				}
			case bool:
				decoded, ok := decodedArgs[i].(bool)
				if !ok {
					// Try pointer
					decodedPtr, ok := decodedArgs[i].(*bool)
					require.True(t, ok, "arg %d should be bool or *bool", i)
					require.NotNil(t, decodedPtr)
					assert.Equal(t, v, *decodedPtr)
				} else {
					assert.Equal(t, v, decoded)
				}
			case []byte:
				decoded, ok := decodedArgs[i].([]byte)
				if !ok {
					// Try pointer
					decodedPtr, ok := decodedArgs[i].(*[]byte)
					require.True(t, ok, "arg %d should be []byte or *[]byte", i)
					require.NotNil(t, decodedPtr)
					assert.Equal(t, v, *decodedPtr)
				} else {
					assert.Equal(t, v, decoded)
				}
			}
		}
	}
}

func TestEncodeActionArgs_WithDecimal(t *testing.T) {
	// Test encoding Decimal types (used for threshold values)
	decimal, err := types.ParseDecimalExplicit("87000.50", 36, 18)
	require.NoError(t, err)

	args := []any{
		"0x1111111111111111111111111111111111111111",
		"stbtcusd000000000000000000000000",
		int64(1735689600),
		decimal, // Threshold as Decimal
		nil,     // FrozenAt
	}

	encoded, err := EncodeActionArgs(args)
	require.NoError(t, err)
	require.NotNil(t, encoded)

	// Verify we can read the encoded format
	buf := bytes.NewReader(encoded)
	var argCount uint32
	err = binary.Read(buf, binary.LittleEndian, &argCount)
	require.NoError(t, err)
	assert.Equal(t, uint32(5), argCount)

	// Decode each argument
	for i := 0; i < 5; i++ {
		var argLen uint32
		err = binary.Read(buf, binary.LittleEndian, &argLen)
		require.NoError(t, err)

		argBytes := make([]byte, argLen)
		_, err = buf.Read(argBytes)
		require.NoError(t, err)

		var encodedVal types.EncodedValue
		err = encodedVal.UnmarshalBinary(argBytes)
		require.NoError(t, err)

		decodedVal, err := encodedVal.Decode()
		require.NoError(t, err)

		// Verify the Decimal argument (index 3)
		if i == 3 {
			decodedDecimal, ok := decodedVal.(*types.Decimal)
			require.True(t, ok, "arg 3 should be *types.Decimal")
			require.NotNil(t, decodedDecimal)
			// The value should match (87000.50 with 18 decimal places)
			assert.Equal(t, "87000.500000000000000000", decodedDecimal.String())
		}
	}
}

func TestEncodeActionArgs_EdgeCases(t *testing.T) {
	t.Run("nil args", func(t *testing.T) {
		encoded, err := EncodeActionArgs(nil)
		require.NoError(t, err)

		// Should encode as 0 args
		buf := bytes.NewReader(encoded)
		var argCount uint32
		err = binary.Read(buf, binary.LittleEndian, &argCount)
		require.NoError(t, err)
		assert.Equal(t, uint32(0), argCount)
	})

	t.Run("args with nil value", func(t *testing.T) {
		encoded, err := EncodeActionArgs([]any{nil})
		require.NoError(t, err)

		buf := bytes.NewReader(encoded)
		var argCount uint32
		err = binary.Read(buf, binary.LittleEndian, &argCount)
		require.NoError(t, err)
		assert.Equal(t, uint32(1), argCount)
	})

	t.Run("large string", func(t *testing.T) {
		largeString := string(make([]byte, 10000))
		encoded, err := EncodeActionArgs([]any{largeString})
		require.NoError(t, err)
		assert.Greater(t, len(encoded), 10000, "encoded size should include overhead")
	})
}
