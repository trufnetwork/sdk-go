package contractsapi

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/trufnetwork/kwil-db/core/types"
)

// EncodeActionArgs encodes action arguments into canonical bytes using Kwil's native encoding.
// This matches the format used by tn_utils.EncodeActionArgs in the node.
//
// Format: [arg_count:uint32][length:uint32][encoded_arg1][length:uint32][encoded_arg2]...
// Where each encoded_arg uses types.EncodedValue.MarshalBinary() format
//
// Supported types (via types.EncodeValue):
//   - nil
//   - int, int8, int16, int32, int64, uint, uint16, uint32, uint64
//   - string
//   - []byte
//   - bool
//   - [16]byte, types.UUID (UUID)
//   - types.Decimal
//   - Arrays of the above types (e.g., []string, []int64)
//
// Returns an error if any argument cannot be encoded by Kwil's type system.
func EncodeActionArgs(args []any) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write argument count (little-endian uint32)
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(args))); err != nil {
		return nil, fmt.Errorf("failed to write arg count: %w", err)
	}

	// Encode each argument using Kwil's native encoding
	for i, arg := range args {
		encodedVal, err := types.EncodeValue(arg)
		if err != nil {
			return nil, fmt.Errorf("failed to encode arg %d: %w", i, err)
		}

		// Serialize the EncodedValue
		argBytes, err := encodedVal.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arg %d: %w", i, err)
		}

		// Write length-prefixed argument bytes (little-endian uint32)
		if err := binary.Write(buf, binary.LittleEndian, uint32(len(argBytes))); err != nil {
			return nil, fmt.Errorf("failed to write arg %d length: %w", i, err)
		}
		if _, err := buf.Write(argBytes); err != nil {
			return nil, fmt.Errorf("failed to write arg %d bytes: %w", i, err)
		}
	}

	return buf.Bytes(), nil
}
