// Golden bytes for the outcome actions (ids 13-16). The market hash is taken
// over these args, so every SDK must encode them byte for byte the same way:
// sdk-js and sdk-py check their encoders against testdata/outcome_v2_golden.json.
//
// Regenerate the file after a deliberate encoding change with:
//
//	UPDATE_GOLDEN=1 go test ./core/contractsapi -run TestOutcomeV2Golden

package contractsapi

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/types"
)

const outcomeGoldenPath = "testdata/outcome_v2_golden.json"

// outcomeGolden is one entry of the golden file.
type outcomeGolden struct {
	Action        string         `json:"action"`
	Inputs        map[string]any `json:"inputs"`
	ArgsHex       string         `json:"args_hex"`
	ComponentsHex string         `json:"components_hex"`
}

// goldenProvider is mixed case on purpose: the args must carry it lowercased.
const goldenProvider = "0xAbCdEf0123456789aBcDeF0123456789AbCdEf01"

// outcomeGoldenCases builds one example of each outcome action. The inputs map
// uses the snake_case names sdk-py takes; sdk-js takes the same in camelCase.
func outcomeGoldenCases(t *testing.T) map[string]outcomeGolden {
	t.Helper()
	out := map[string]outcomeGolden{}

	add := func(id string, action string, inputs map[string]any, encoded []byte, err error) {
		require.NoError(t, err, action)
		_, _, gotAction, argsBytes, err := DecodeQueryComponents(encoded)
		require.NoError(t, err)
		require.Equal(t, action, gotAction)
		out[id] = outcomeGolden{
			Action:        action,
			Inputs:        inputs,
			ArgsHex:       hex.EncodeToString(argsBytes),
			ComponentsHex: hex.EncodeToString(encoded),
		}
	}

	// 13: a year-over-year CPI bucket with the month-start defaults.
	change := types.IndexChangeInRangeV2Input{
		DataProvider: goldenProvider, StreamID: "stcpiyoy000000000000000000000000",
		Timestamp: 1759276800, Window: 7776000, Tolerance: 172800, TimeInterval: 31536000,
		MinChange: strPtr("-0.5"), MaxChange: strPtr("2.5"),
	}
	encoded, err := BuildIndexChangeInRangeV2QueryComponents(change)
	add("13", change.ActionName(), map[string]any{
		"data_provider": change.DataProvider, "stream_id": change.StreamID,
		"timestamp": change.Timestamp, "window": change.Window, "tolerance": change.Tolerance,
		"time_interval": change.TimeInterval, "min_change": "-0.5", "max_change": "2.5",
	}, encoded, err)

	// 14 and 15: daily ladder defaults.
	above := types.PriceAboveThresholdV2Input{
		DataProvider: goldenProvider, StreamID: "stbtcusd000000000000000000000000",
		Timestamp: 1767139200, Window: 259200, Tolerance: 86400, Threshold: "100000",
	}
	encoded, err = BuildPriceAboveThresholdV2QueryComponents(above)
	add("14", above.ActionName(), map[string]any{
		"data_provider": above.DataProvider, "stream_id": above.StreamID,
		"timestamp": above.Timestamp, "window": above.Window, "tolerance": above.Tolerance,
		"threshold": above.Threshold,
	}, encoded, err)

	below := types.PriceBelowThresholdV2Input{
		DataProvider: goldenProvider, StreamID: "stunemprate000000000000000000000",
		Timestamp: 1767139200, Window: 259200, Tolerance: 86400, Threshold: "4.25",
	}
	encoded, err = BuildPriceBelowThresholdV2QueryComponents(below)
	add("15", below.ActionName(), map[string]any{
		"data_provider": below.DataProvider, "stream_id": below.StreamID,
		"timestamp": below.Timestamp, "window": below.Window, "tolerance": below.Tolerance,
		"threshold": below.Threshold,
	}, encoded, err)

	// 16: the top bucket of a ladder, open above, so the NULL encoding is pinned.
	rng := types.ValueInRangeV2Input{
		DataProvider: goldenProvider, StreamID: "stbtcusd000000000000000000000000",
		Timestamp: 1767139200, Window: 259200, Tolerance: 86400,
		MinValue: strPtr("110000.5"), MaxValue: nil,
	}
	encoded, err = BuildValueInRangeV2QueryComponents(rng)
	add("16", rng.ActionName(), map[string]any{
		"data_provider": rng.DataProvider, "stream_id": rng.StreamID,
		"timestamp": rng.Timestamp, "window": rng.Window, "tolerance": rng.Tolerance,
		"min_value": "110000.5", "max_value": nil,
	}, encoded, err)

	return out
}

func TestOutcomeV2Golden(t *testing.T) {
	got := outcomeGoldenCases(t)
	for _, id := range []string{"13", "14", "15", "16"} {
		t.Logf("%s %s\n  args_hex:       %s\n  components_hex: %s",
			id, got[id].Action, got[id].ArgsHex, got[id].ComponentsHex)
	}

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		data, err := json.MarshalIndent(got, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(outcomeGoldenPath), 0o755))
		require.NoError(t, os.WriteFile(outcomeGoldenPath, append(data, '\n'), 0o644))
	}

	data, err := os.ReadFile(outcomeGoldenPath)
	require.NoError(t, err, "run with UPDATE_GOLDEN=1 to create it")
	var want map[string]outcomeGolden
	require.NoError(t, json.Unmarshal(data, &want))

	require.Len(t, want, 4)
	for id, g := range got {
		require.Contains(t, want, id)
		assert.Equal(t, want[id].Action, g.Action, id)
		assert.Equal(t, want[id].ArgsHex, g.ArgsHex, "args bytes changed for id %s", id)
		assert.Equal(t, want[id].ComponentsHex, g.ComponentsHex, "query_components changed for id %s", id)
	}
}
