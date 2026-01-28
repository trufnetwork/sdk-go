package types

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════
// CREATE MARKET INPUT VALIDATION TESTS
// ═══════════════════════════════════════════════════════════════

// createValidQueryComponents creates a minimal valid QueryComponents byte slice
// This is a simplified mock - actual encoding uses ABI encoding
func createValidQueryComponents(minSize int) []byte {
	// Create a byte slice that's at least 128 bytes (minimum for ABI-encoded tuple)
	if minSize < 128 {
		minSize = 128
	}
	return make([]byte, minSize)
}

func TestCreateMarketInput_Validate_Valid(t *testing.T) {
	validInput := CreateMarketInput{
		Bridge:          "hoodi_tt2",
		QueryComponents: createValidQueryComponents(128),
		SettleTime:      time.Now().Unix() + 3600, // 1 hour in future
		MaxSpread:       5,
		MinOrderSize:    100,
	}

	err := validInput.Validate()
	require.NoError(t, err)
}

func TestCreateMarketInput_Validate_InvalidBridge(t *testing.T) {
	tests := []struct {
		name   string
		bridge string
	}{
		{
			name:   "Empty bridge",
			bridge: "",
		},
		{
			name:   "Invalid bridge name",
			bridge: "invalid_bridge",
		},
		{
			name:   "Wrong case",
			bridge: "HOODI_TT2",
		},
		{
			name:   "Typo in name",
			bridge: "hoodi_tt3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := CreateMarketInput{
				Bridge:          tt.bridge,
				QueryComponents: createValidQueryComponents(128),
				SettleTime:      time.Now().Unix() + 3600,
				MaxSpread:       5,
				MinOrderSize:    100,
			}

			err := input.Validate()
			require.Error(t, err)

			if tt.bridge == "" {
				require.Contains(t, err.Error(), "bridge is required")
			} else {
				require.Contains(t, err.Error(), "bridge must be one of")
			}
		})
	}
}

func TestCreateMarketInput_Validate_AllValidBridges(t *testing.T) {
	validBridges := []string{
		"hoodi_tt2",
		"sepolia_bridge",
		"ethereum_bridge",
	}

	for _, bridge := range validBridges {
		t.Run(bridge, func(t *testing.T) {
			input := CreateMarketInput{
				Bridge:          bridge,
				QueryComponents: createValidQueryComponents(128),
				SettleTime:      time.Now().Unix() + 3600,
				MaxSpread:       5,
				MinOrderSize:    100,
			}

			err := input.Validate()
			require.NoError(t, err)
		})
	}
}

func TestCreateMarketInput_Validate_InvalidQueryComponents(t *testing.T) {
	tests := []struct {
		name            string
		queryComponents []byte
		expectedError   string
	}{
		{
			name:            "Nil query components",
			queryComponents: nil,
			expectedError:   "query_components is required",
		},
		{
			name:            "Empty query components",
			queryComponents: []byte{},
			expectedError:   "query_components is required",
		},
		{
			name:            "Too short (less than 128 bytes)",
			queryComponents: make([]byte, 64),
			expectedError:   "query_components too short",
		},
		{
			name:            "Just under minimum",
			queryComponents: make([]byte, 127),
			expectedError:   "query_components too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := CreateMarketInput{
				Bridge:          "hoodi_tt2",
				QueryComponents: tt.queryComponents,
				SettleTime:      time.Now().Unix() + 3600,
				MaxSpread:       5,
				MinOrderSize:    100,
			}

			err := input.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestCreateMarketInput_Validate_ValidQueryComponentsSize(t *testing.T) {
	// Test minimum valid size (128 bytes)
	input := CreateMarketInput{
		Bridge:          "hoodi_tt2",
		QueryComponents: make([]byte, 128),
		SettleTime:      time.Now().Unix() + 3600,
		MaxSpread:       5,
		MinOrderSize:    100,
	}

	err := input.Validate()
	require.NoError(t, err)

	// Test larger size
	input.QueryComponents = make([]byte, 1024)
	err = input.Validate()
	require.NoError(t, err)
}

func TestCreateMarketInput_Validate_InvalidSettleTime(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name       string
		settleTime int64
	}{
		{
			name:       "Zero",
			settleTime: 0,
		},
		{
			name:       "Negative",
			settleTime: -1,
		},
		{
			name:       "Past time",
			settleTime: now - 3600, // 1 hour ago
		},
		{
			name:       "Current time",
			settleTime: now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := CreateMarketInput{
				Bridge:          "hoodi_tt2",
				QueryComponents: createValidQueryComponents(128),
				SettleTime:      tt.settleTime,
				MaxSpread:       5,
				MinOrderSize:    100,
			}

			err := input.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), "settle_time")
		})
	}
}

func TestCreateMarketInput_Validate_FutureSettleTime(t *testing.T) {
	futureTime := time.Now().Unix() + 86400 // 24 hours in future

	input := CreateMarketInput{
		Bridge:          "hoodi_tt2",
		QueryComponents: createValidQueryComponents(128),
		SettleTime:      futureTime,
		MaxSpread:       5,
		MinOrderSize:    100,
	}

	err := input.Validate()
	require.NoError(t, err)
}

func TestCreateMarketInput_Validate_InvalidMaxSpread(t *testing.T) {
	tests := []struct {
		name      string
		maxSpread int
	}{
		{
			name:      "Zero",
			maxSpread: 0,
		},
		{
			name:      "Negative",
			maxSpread: -1,
		},
		{
			name:      "Too large (51)",
			maxSpread: 51,
		},
		{
			name:      "Too large (100)",
			maxSpread: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := CreateMarketInput{
				Bridge:          "hoodi_tt2",
				QueryComponents: createValidQueryComponents(128),
				SettleTime:      time.Now().Unix() + 3600,
				MaxSpread:       tt.maxSpread,
				MinOrderSize:    100,
			}

			err := input.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), "max_spread must be between 1 and 50")
		})
	}
}

func TestCreateMarketInput_Validate_ValidMaxSpreadRange(t *testing.T) {
	validSpreads := []int{1, 5, 10, 25, 50}

	for _, spread := range validSpreads {
		t.Run(fmt.Sprintf("%d", spread), func(t *testing.T) {
			input := CreateMarketInput{
				Bridge:          "hoodi_tt2",
				QueryComponents: createValidQueryComponents(128),
				SettleTime:      time.Now().Unix() + 3600,
				MaxSpread:       spread,
				MinOrderSize:    100,
			}

			err := input.Validate()
			require.NoError(t, err)
		})
	}
}

func TestCreateMarketInput_Validate_InvalidMinOrderSize(t *testing.T) {
	tests := []struct {
		name         string
		minOrderSize int64
	}{
		{
			name:         "Zero",
			minOrderSize: 0,
		},
		{
			name:         "Negative",
			minOrderSize: -1,
		},
		{
			name:         "Large negative",
			minOrderSize: -1000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := CreateMarketInput{
				Bridge:          "hoodi_tt2",
				QueryComponents: createValidQueryComponents(128),
				SettleTime:      time.Now().Unix() + 3600,
				MaxSpread:       5,
				MinOrderSize:    tt.minOrderSize,
			}

			err := input.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), "min_order_size must be positive")
		})
	}
}

func TestCreateMarketInput_Validate_ValidMinOrderSize(t *testing.T) {
	validSizes := []int64{1, 10, 100, 1000, 1000000}

	for _, size := range validSizes {
		t.Run(fmt.Sprintf("%d", size), func(t *testing.T) {
			input := CreateMarketInput{
				Bridge:          "hoodi_tt2",
				QueryComponents: createValidQueryComponents(128),
				SettleTime:      time.Now().Unix() + 3600,
				MaxSpread:       5,
				MinOrderSize:    size,
			}

			err := input.Validate()
			require.NoError(t, err)
		})
	}
}

func TestCreateMarketInput_Validate_EdgeCases(t *testing.T) {
	t.Run("All minimum valid values", func(t *testing.T) {
		input := CreateMarketInput{
			Bridge:          "hoodi_tt2",
			QueryComponents: make([]byte, 128), // Minimum valid size
			SettleTime:      time.Now().Unix() + 1,
			MaxSpread:       1,
			MinOrderSize:    1,
		}

		err := input.Validate()
		require.NoError(t, err)
	})

	t.Run("All maximum valid values", func(t *testing.T) {
		input := CreateMarketInput{
			Bridge:          "ethereum_bridge",
			QueryComponents: make([]byte, 10000), // Large query components
			SettleTime:      time.Now().Unix() + 31536000, // 1 year in future
			MaxSpread:       50,
			MinOrderSize:    1000000000,
		}

		err := input.Validate()
		require.NoError(t, err)
	})
}

// ═══════════════════════════════════════════════════════════════
// BINARY ACTION INPUT VALIDATION TESTS
// ═══════════════════════════════════════════════════════════════

func TestPriceAboveThresholdInput_Validate(t *testing.T) {
	t.Run("Valid input", func(t *testing.T) {
		input := PriceAboveThresholdInput{
			DataProvider: "0x1111111111111111111111111111111111111111",
			StreamID:     "stbtcusd000000000000000000000000",
			Timestamp:    1735689600,
			Threshold:    "100000",
		}
		require.NoError(t, input.Validate())
	})

	t.Run("Invalid data provider", func(t *testing.T) {
		input := PriceAboveThresholdInput{
			DataProvider: "invalid",
			StreamID:     "stbtcusd000000000000000000000000",
			Timestamp:    1735689600,
			Threshold:    "100000",
		}
		require.Error(t, input.Validate())
	})

	t.Run("Invalid stream ID length", func(t *testing.T) {
		input := PriceAboveThresholdInput{
			DataProvider: "0x1111111111111111111111111111111111111111",
			StreamID:     "btc", // Too short
			Timestamp:    1735689600,
			Threshold:    "100000",
		}
		require.Error(t, input.Validate())
	})

	t.Run("Invalid timestamp", func(t *testing.T) {
		input := PriceAboveThresholdInput{
			DataProvider: "0x1111111111111111111111111111111111111111",
			StreamID:     "stbtcusd000000000000000000000000",
			Timestamp:    0,
			Threshold:    "100000",
		}
		require.Error(t, input.Validate())
	})

	t.Run("Empty threshold", func(t *testing.T) {
		input := PriceAboveThresholdInput{
			DataProvider: "0x1111111111111111111111111111111111111111",
			StreamID:     "stbtcusd000000000000000000000000",
			Timestamp:    1735689600,
			Threshold:    "",
		}
		require.Error(t, input.Validate())
	})
}

func TestValueInRangeInput_Validate(t *testing.T) {
	t.Run("Valid input", func(t *testing.T) {
		input := ValueInRangeInput{
			DataProvider: "0x1111111111111111111111111111111111111111",
			StreamID:     "stbtcusd000000000000000000000000",
			Timestamp:    1735689600,
			MinValue:     "90000",
			MaxValue:     "110000",
		}
		require.NoError(t, input.Validate())
	})

	t.Run("Missing min value", func(t *testing.T) {
		input := ValueInRangeInput{
			DataProvider: "0x1111111111111111111111111111111111111111",
			StreamID:     "stbtcusd000000000000000000000000",
			Timestamp:    1735689600,
			MinValue:     "",
			MaxValue:     "110000",
		}
		require.Error(t, input.Validate())
	})

	t.Run("Missing max value", func(t *testing.T) {
		input := ValueInRangeInput{
			DataProvider: "0x1111111111111111111111111111111111111111",
			StreamID:     "stbtcusd000000000000000000000000",
			Timestamp:    1735689600,
			MinValue:     "90000",
			MaxValue:     "",
		}
		require.Error(t, input.Validate())
	})
}

func TestValueEqualsInput_Validate(t *testing.T) {
	t.Run("Valid input", func(t *testing.T) {
		input := ValueEqualsInput{
			DataProvider: "0x1111111111111111111111111111111111111111",
			StreamID:     "stbtcusd000000000000000000000000",
			Timestamp:    1735689600,
			TargetValue:  "5.25",
			Tolerance:    "0.01",
		}
		require.NoError(t, input.Validate())
	})

	t.Run("Missing target value", func(t *testing.T) {
		input := ValueEqualsInput{
			DataProvider: "0x1111111111111111111111111111111111111111",
			StreamID:     "stbtcusd000000000000000000000000",
			Timestamp:    1735689600,
			TargetValue:  "",
			Tolerance:    "0.01",
		}
		require.Error(t, input.Validate())
	})

	t.Run("Missing tolerance", func(t *testing.T) {
		input := ValueEqualsInput{
			DataProvider: "0x1111111111111111111111111111111111111111",
			StreamID:     "stbtcusd000000000000000000000000",
			Timestamp:    1735689600,
			TargetValue:  "5.25",
			Tolerance:    "",
		}
		require.Error(t, input.Validate())
	})
}

// ═══════════════════════════════════════════════════════════════
// ACTION REGISTRY TESTS
// ═══════════════════════════════════════════════════════════════

func TestActionRegistry(t *testing.T) {
	t.Run("Get numeric action info", func(t *testing.T) {
		info := GetActionInfo("get_record")
		require.NotNil(t, info)
		require.Equal(t, uint16(1), info.ID)
		require.False(t, info.IsBinary)
	})

	t.Run("Get binary action info", func(t *testing.T) {
		info := GetActionInfo("price_above_threshold")
		require.NotNil(t, info)
		require.Equal(t, uint16(6), info.ID)
		require.True(t, info.IsBinary)
	})

	t.Run("Unknown action returns nil", func(t *testing.T) {
		info := GetActionInfo("unknown_action")
		require.Nil(t, info)
	})

	t.Run("IsBinaryAction checks", func(t *testing.T) {
		require.False(t, IsBinaryAction("get_record"))
		require.False(t, IsBinaryAction("get_index"))
		require.True(t, IsBinaryAction("price_above_threshold"))
		require.True(t, IsBinaryAction("price_below_threshold"))
		require.True(t, IsBinaryAction("value_in_range"))
		require.True(t, IsBinaryAction("value_equals"))
	})

	t.Run("IsBinaryActionID checks", func(t *testing.T) {
		require.False(t, IsBinaryActionID(1))
		require.False(t, IsBinaryActionID(5))
		require.True(t, IsBinaryActionID(6))
		require.True(t, IsBinaryActionID(7))
		require.True(t, IsBinaryActionID(8))
		require.True(t, IsBinaryActionID(9))
		require.False(t, IsBinaryActionID(10))
	})

	t.Run("GetActionID", func(t *testing.T) {
		require.Equal(t, uint16(1), GetActionID("get_record"))
		require.Equal(t, uint16(6), GetActionID("price_above_threshold"))
		require.Equal(t, uint16(0), GetActionID("unknown"))
	})

	t.Run("GetActionName", func(t *testing.T) {
		require.Equal(t, "get_record", GetActionName(1))
		require.Equal(t, "price_above_threshold", GetActionName(6))
		require.Equal(t, "", GetActionName(100))
	})
}
