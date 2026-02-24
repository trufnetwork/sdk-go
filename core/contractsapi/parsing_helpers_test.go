package contractsapi

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

// TestParsingHelpers validates the helper functions used to parse gateway responses.
// It covers variations in data formats returned by the gateway (e.g., Hex vs Base64 for bytes, String vs Number for int64).
// Regression test for issues reported in sdk-py/issues/1.
func TestParsingHelpers(t *testing.T) {
	// 1. Test extractBytesColumn with Hex string (Gateway behavior)
	t.Run("extractBytesColumn_Hex", func(t *testing.T) {
		// Mock a 0x-prefixed hex string (e.g. an address)
		// 0x1234
		hexStr := "0x1234"
		expectedBytes, _ := hex.DecodeString("1234")

		var result []byte
		err := extractBytesColumn(hexStr, &result, 0, "wallet_address")

		if err != nil {
			t.Fatalf("extractBytesColumn failed on hex string: %v", err)
		}

		if string(result) != string(expectedBytes) {
			t.Errorf("expected %x, got %x", expectedBytes, result)
		}
	})

	// 1b. Test extractBytesColumn with Hybrid Base64 string (starts with 0x but is Base64)
	t.Run("extractBytesColumn_HybridBase64", func(t *testing.T) {
		// We use a known valid 32-byte base64 string
		validB64 := "AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA="
		hybridStr := "0x" + validB64
		
		var result []byte
		err := extractBytesColumn(hybridStr, &result, 0, "hash")

		if err != nil {
			t.Fatalf("extractBytesColumn failed on hybrid base64 string: %v", err)
		}

		// Verify result matches base64 content
		expectedBytes, _ := base64.StdEncoding.DecodeString(validB64)
		if string(result) != string(expectedBytes) {
			t.Errorf("expected %x, got %x", expectedBytes, result)
		}
	})

	// 2. Test extractInt64Column with non-string type (e.g. float64 from JSON)
	t.Run("extractInt64Column_Float64", func(t *testing.T) {
		// Mock a float64 value (common for JSON numbers)
		val := float64(12345)
		var expected int64 = 12345

		var result int64
		err := extractInt64Column(val, &result, 0, "amount")

		if err != nil {
			t.Fatalf("extractInt64Column failed on float64: %v", err)
		}

		if result != expected {
			t.Errorf("expected %d, got %d", expected, result)
		}
	})

	// 3. Test extractInt64Column with int (if unmarshalled as int)
	t.Run("extractInt64Column_Int", func(t *testing.T) {
		val := int(54321)
		var expected int64 = 54321

		var result int64
		err := extractInt64Column(val, &result, 0, "amount")

		if err != nil {
			t.Fatalf("extractInt64Column failed on int: %v", err)
		}

		if result != expected {
			t.Errorf("expected %d, got %d", expected, result)
		}
	})

	// 4. Test extractIntColumn with float64 (common for JSON numbers)
	t.Run("extractIntColumn_Float64", func(t *testing.T) {
		val := float64(123)
		var expected int = 123

		var result int
		err := extractIntColumn(val, &result, 0, "price")

		if err != nil {
			t.Fatalf("extractIntColumn failed on float64: %v", err)
		}

		if result != expected {
			t.Errorf("expected %d, got %d", expected, result)
		}
	})
}
