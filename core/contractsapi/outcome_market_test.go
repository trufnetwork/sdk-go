// Tests for the outcome actions (ids 13-16, node migration 063): input
// validation against the node's bounds, positional argument encoding, decoding,
// bucket bounds, forecast identity and result parsing.

package contractsapi

import (
	"context"
	"encoding/binary"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/forecast"
	"github.com/trufnetwork/sdk-go/core/types"
)

const (
	outcomeProvider      = "0xAbCdEf0123456789aBcDeF0123456789AbCdEf01"
	outcomeProviderLower = "0xabcdef0123456789abcdef0123456789abcdef01"
	outcomeStream        = "stcpiyoy000000000000000000000000"
	outcomeTimestamp     = int64(1759276800)
	outcomeDay           = int64(86400)
)

func validAboveV2() types.PriceAboveThresholdV2Input {
	return types.PriceAboveThresholdV2Input{
		DataProvider: outcomeProvider, StreamID: outcomeStream,
		Timestamp: outcomeTimestamp, Window: 3 * outcomeDay, Tolerance: outcomeDay,
		Threshold: "100000",
	}
}

func validBelowV2() types.PriceBelowThresholdV2Input {
	return types.PriceBelowThresholdV2Input{
		DataProvider: outcomeProvider, StreamID: outcomeStream,
		Timestamp: outcomeTimestamp, Window: 3 * outcomeDay, Tolerance: outcomeDay,
		Threshold: "4.25",
	}
}

func validRangeV2() types.ValueInRangeV2Input {
	return types.ValueInRangeV2Input{
		DataProvider: outcomeProvider, StreamID: outcomeStream,
		Timestamp: outcomeTimestamp, Window: 3 * outcomeDay, Tolerance: outcomeDay,
		MinValue: strPtr("90000"), MaxValue: strPtr("110000"),
	}
}

func validChangeV2() types.IndexChangeInRangeV2Input {
	return types.IndexChangeInRangeV2Input{
		DataProvider: outcomeProvider, StreamID: outcomeStream,
		Timestamp: outcomeTimestamp, Window: 7776000, Tolerance: 172800,
		TimeInterval: yearInSeconds,
		MinChange:    strPtr("2"), MaxChange: strPtr("3"),
	}
}

// ═══════════════════════════════════════════════════════════════
// INPUT VALIDATION
// ═══════════════════════════════════════════════════════════════

func TestOutcomeInput_ValidInputsAreAccepted(t *testing.T) {
	above, below, rng, change := validAboveV2(), validBelowV2(), validRangeV2(), validChangeV2()
	for _, input := range []types.OutcomeActionInput{&above, &below, &rng, &change} {
		assert.NoError(t, input.Validate(), input.ActionName())
	}
	assert.Equal(t, "price_above_threshold_v2", above.ActionName())
	assert.Equal(t, "price_below_threshold_v2", below.ActionName())
	assert.Equal(t, "value_in_range_v2", rng.ActionName())
	assert.Equal(t, "index_change_in_range_v2", change.ActionName())
}

func TestOutcomeInput_TimeBoundsMatchTheNode(t *testing.T) {
	maxSpan, maxTol := types.OutcomeMaxSpanSeconds, types.OutcomeMaxToleranceSeconds
	assert.Equal(t, int64(315360000), maxSpan)
	assert.Equal(t, int64(3888000), maxTol)

	cases := []struct {
		name                         string
		timestamp, window, tolerance int64
		wantErr                      string
	}{
		{"timestamp zero is allowed", 0, 10, 0, ""},
		{"negative timestamp", -1, 10, 0, "timestamp must not be negative"},
		{"zero window", outcomeTimestamp, 0, 0, "window must be in (0, 315360000]"},
		{"negative window", outcomeTimestamp, -1, 0, "window must be in"},
		{"window at the cap", outcomeTimestamp, maxSpan, maxTol, ""},
		{"window past the cap", outcomeTimestamp, maxSpan + 1, 0, "window must be in"},
		{"zero tolerance", outcomeTimestamp, 1, 0, ""},
		{"negative tolerance", outcomeTimestamp, 10, -1, "tolerance must be in [0, 3888000]"},
		{"tolerance past the cap", outcomeTimestamp, maxSpan, maxTol + 1, "tolerance must be in"},
		{"window equal to tolerance", outcomeTimestamp, outcomeDay, outcomeDay, ""},
		{"window below tolerance", outcomeTimestamp, outcomeDay - 1, outcomeDay, "must not be less than tolerance"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			above := validAboveV2()
			above.Timestamp, above.Window, above.Tolerance = tc.timestamp, tc.window, tc.tolerance
			rng := validRangeV2()
			rng.Timestamp, rng.Window, rng.Tolerance = tc.timestamp, tc.window, tc.tolerance
			for _, input := range []types.OutcomeActionInput{&above, &rng} {
				err := input.Validate()
				if tc.wantErr == "" {
					assert.NoError(t, err, input.ActionName())
				} else {
					require.Error(t, err, input.ActionName())
					assert.Contains(t, err.Error(), tc.wantErr)
				}
			}
		})
	}
}

func TestOutcomeInput_IntervalBoundsMatchTheNode(t *testing.T) {
	cases := []struct {
		name                string
		interval, tolerance int64
		wantErr             string
	}{
		{"zero interval", 0, 0, "time_interval must be in (0, 315360000]"},
		{"negative interval", -1, 0, "time_interval must be in"},
		{"interval at the cap", types.OutcomeMaxSpanSeconds, 172800, ""},
		{"interval past the cap", types.OutcomeMaxSpanSeconds + 1, 172800, "time_interval must be in"},
		{"interval twice the tolerance", 2 * outcomeDay, outcomeDay, "more than twice tolerance"},
		{"interval just over twice the tolerance", 2*outcomeDay + 1, outcomeDay, ""},
		{"zero tolerance", 1, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := validChangeV2()
			input.TimeInterval, input.Tolerance = tc.interval, tc.tolerance
			err := input.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestOutcomeInput_ThresholdIsRequiredAndNumeric(t *testing.T) {
	for _, threshold := range []string{"", "NaN", "nan", "ten", "Inf"} {
		above := validAboveV2()
		above.Threshold = threshold
		assert.Error(t, above.Validate(), "threshold %q", threshold)

		below := validBelowV2()
		below.Threshold = threshold
		assert.Error(t, below.Validate(), "threshold %q", threshold)
	}

	negative := validBelowV2()
	negative.Threshold = "-1.5"
	assert.NoError(t, negative.Validate())
}

func TestOutcomeInput_BoundsMatchTheNode(t *testing.T) {
	cases := []struct {
		name     string
		min, max *string
		wantErr  string
	}{
		{"both bounds", strPtr("1"), strPtr("2"), ""},
		{"open below", nil, strPtr("2"), ""},
		{"open above", strPtr("1"), nil, ""},
		{"both open", nil, nil, "at least one of"},
		{"empty min", strPtr(""), strPtr("2"), "use nil for an open tail"},
		{"empty max", strPtr("1"), strPtr(""), "use nil for an open tail"},
		{"NaN min", strPtr("NaN"), strPtr("2"), "got NaN"},
		{"NaN max alone", nil, strPtr("NaN"), "got NaN"},
		{"inverted", strPtr("3"), strPtr("2"), "must be less than"},
		{"equal", strPtr("2"), strPtr("2"), "must be less than"},
		{"equal at action precision", strPtr("2.0"), strPtr("2"), "must be less than"},
		{"equal past 18 decimals", strPtr("2"), strPtr("2.0000000000000000001"), "must be less than"},
		{"ordered as numbers, not text", strPtr("9"), strPtr("10"), ""},
		{"unparseable", strPtr("one"), strPtr("2"), "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rng := validRangeV2()
			rng.MinValue, rng.MaxValue = tc.min, tc.max
			change := validChangeV2()
			change.MinChange, change.MaxChange = tc.min, tc.max
			for _, input := range []types.OutcomeActionInput{&rng, &change} {
				err := input.Validate()
				if tc.wantErr == "" {
					assert.NoError(t, err, input.ActionName())
				} else {
					require.Error(t, err, input.ActionName())
					assert.Contains(t, err.Error(), tc.wantErr)
				}
			}
		})
	}
}

func TestOutcomeInput_Identifiers(t *testing.T) {
	upper := validAboveV2()
	upper.DataProvider = "0X" + strings.ToUpper(outcomeProvider[2:])
	assert.NoError(t, upper.Validate(), "the node reads the provider in either case")

	nonHex := validAboveV2()
	nonHex.DataProvider = "0x" + strings.Repeat("g", 40)
	assert.Error(t, nonHex.Validate())

	unprefixed := validAboveV2()
	unprefixed.DataProvider = "ab" + outcomeProvider[2:]
	assert.Error(t, unprefixed.Validate())

	short := validAboveV2()
	short.StreamID = "stshort"
	assert.Error(t, short.Validate())
}

// ═══════════════════════════════════════════════════════════════
// ARGUMENT ENCODING
// ═══════════════════════════════════════════════════════════════

// decodeOutcomeComponents decodes built query_components into the header and
// the positional args the node action sees.
func decodeOutcomeComponents(t *testing.T, encoded []byte, wantAction string) (string, []any) {
	t.Helper()
	provider, stream, actionID, argsBytes, err := DecodeQueryComponents(encoded)
	require.NoError(t, err)
	require.Equal(t, wantAction, actionID)
	require.Equal(t, outcomeStream, stream)
	args, err := DecodeActionArgs(argsBytes)
	require.NoError(t, err)
	return provider, args
}

// requireNumeric checks an arg is NUMERIC(36,18) with the given text.
func requireNumeric(t *testing.T, arg any, want string) {
	t.Helper()
	d, ok := arg.(*kwiltypes.Decimal)
	require.True(t, ok, "expected *Decimal, got %T", arg)
	require.NotNil(t, d)
	assert.Equal(t, uint16(36), d.Precision())
	assert.Equal(t, uint16(18), d.Scale())
	assert.Equal(t, want, d.String())
}

// requireInt8 checks an arg is INT8 with the given value.
func requireInt8(t *testing.T, arg any, want int64) {
	t.Helper()
	v, ok := arg.(*int64)
	require.True(t, ok, "expected *int64, got %T", arg)
	require.NotNil(t, v)
	assert.Equal(t, want, *v)
}

func TestOutcomeArgs_ThresholdActionsTakeSixArgs(t *testing.T) {
	above, err := BuildPriceAboveThresholdV2QueryComponents(validAboveV2())
	require.NoError(t, err)
	below, err := BuildPriceBelowThresholdV2QueryComponents(validBelowV2())
	require.NoError(t, err)

	for _, tc := range []struct {
		encoded   []byte
		action    string
		threshold string
	}{
		{above, "price_above_threshold_v2", "100000.000000000000000000"},
		{below, "price_below_threshold_v2", "4.250000000000000000"},
	} {
		provider, args := decodeOutcomeComponents(t, tc.encoded, tc.action)
		require.Len(t, args, 6, "no frozen_at")
		assert.Equal(t, outcomeProviderLower, derefString(args[0]), "provider lowercased in the args")
		assert.True(t, strings.EqualFold(outcomeProvider, provider), "header names the same address")
		assert.Equal(t, outcomeStream, derefString(args[1]))
		requireInt8(t, args[2], outcomeTimestamp)
		requireInt8(t, args[3], 3*outcomeDay)
		requireInt8(t, args[4], outcomeDay)
		requireNumeric(t, args[5], tc.threshold)
	}
}

func TestOutcomeArgs_RangeTakesSevenArgsWithNullTails(t *testing.T) {
	encoded, err := BuildValueInRangeV2QueryComponents(validRangeV2())
	require.NoError(t, err)
	_, args := decodeOutcomeComponents(t, encoded, "value_in_range_v2")
	require.Len(t, args, 7)
	assert.Equal(t, outcomeProviderLower, derefString(args[0]))
	requireInt8(t, args[2], outcomeTimestamp)
	requireInt8(t, args[3], 3*outcomeDay)
	requireInt8(t, args[4], outcomeDay)
	requireNumeric(t, args[5], "90000.000000000000000000")
	requireNumeric(t, args[6], "110000.000000000000000000")

	bottom := validRangeV2()
	bottom.MinValue = nil
	encoded, err = BuildValueInRangeV2QueryComponents(bottom)
	require.NoError(t, err)
	_, args = decodeOutcomeComponents(t, encoded, "value_in_range_v2")
	require.Len(t, args, 7, "an open tail is a NULL arg, not a missing one")
	assert.Nil(t, args[5])
	requireNumeric(t, args[6], "110000.000000000000000000")

	top := validRangeV2()
	top.MaxValue = nil
	encoded, err = BuildValueInRangeV2QueryComponents(top)
	require.NoError(t, err)
	_, args = decodeOutcomeComponents(t, encoded, "value_in_range_v2")
	require.Len(t, args, 7)
	requireNumeric(t, args[5], "90000.000000000000000000")
	assert.Nil(t, args[6])
}

func TestOutcomeArgs_IndexChangeTakesEightArgs(t *testing.T) {
	input := validChangeV2()
	input.MinChange = strPtr("-0.5")
	input.MaxChange = nil
	encoded, err := BuildIndexChangeInRangeV2QueryComponents(input)
	require.NoError(t, err)
	_, args := decodeOutcomeComponents(t, encoded, "index_change_in_range_v2")
	require.Len(t, args, 8, "no base_time, no frozen_at")
	assert.Equal(t, outcomeProviderLower, derefString(args[0]))
	requireInt8(t, args[2], outcomeTimestamp)
	requireInt8(t, args[3], 7776000)
	requireInt8(t, args[4], 172800)
	requireInt8(t, args[5], yearInSeconds)
	requireNumeric(t, args[6], "-0.500000000000000000")
	assert.Nil(t, args[7])
}

func TestOutcomeArgs_InvalidInputNeverReachesTheEncoder(t *testing.T) {
	above := validAboveV2()
	above.Threshold = "NaN"
	_, err := BuildPriceAboveThresholdV2QueryComponents(above)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input")

	below := validBelowV2()
	below.Window = 0
	_, err = BuildPriceBelowThresholdV2QueryComponents(below)
	assert.Error(t, err)

	rng := validRangeV2()
	rng.MinValue, rng.MaxValue = strPtr("2"), strPtr("1")
	_, err = BuildValueInRangeV2QueryComponents(rng)
	assert.Error(t, err)

	change := validChangeV2()
	change.TimeInterval = 2 * change.Tolerance
	_, err = BuildIndexChangeInRangeV2QueryComponents(change)
	assert.Error(t, err)
}

func TestOutcomeArgs_CreateRefusesInvalidInputBeforeSending(t *testing.T) {
	// No client: an invalid input must fail before any call is made.
	book := &OrderBook{}
	ctx := context.Background()

	_, err := book.CreatePriceAboveThresholdV2Market(ctx, CreatePriceAboveThresholdV2MarketInput{
		DataProvider: outcomeProvider, StreamID: outcomeStream, Window: 1, Tolerance: 2, Threshold: "1"})
	assert.Error(t, err)
	_, err = book.CreatePriceBelowThresholdV2Market(ctx, CreatePriceBelowThresholdV2MarketInput{
		DataProvider: outcomeProvider, StreamID: outcomeStream, Window: 1})
	assert.Error(t, err)
	_, err = book.CreateValueInRangeV2Market(ctx, CreateValueInRangeV2MarketInput{
		DataProvider: outcomeProvider, StreamID: outcomeStream, Window: 1})
	assert.Error(t, err)
	_, err = book.CreateIndexChangeInRangeV2Market(ctx, CreateIndexChangeInRangeV2MarketInput{
		DataProvider: outcomeProvider, StreamID: outcomeStream, Window: 1, MinChange: strPtr("1")})
	assert.Error(t, err)
}

// ═══════════════════════════════════════════════════════════════
// DECODING
// ═══════════════════════════════════════════════════════════════

func TestOutcomeDecode_RoundTrip(t *testing.T) {
	above, err := BuildPriceAboveThresholdV2QueryComponents(validAboveV2())
	require.NoError(t, err)
	below, err := BuildPriceBelowThresholdV2QueryComponents(validBelowV2())
	require.NoError(t, err)
	openRange := validRangeV2()
	openRange.MaxValue = nil
	rng, err := BuildValueInRangeV2QueryComponents(openRange)
	require.NoError(t, err)
	change, err := BuildIndexChangeInRangeV2QueryComponents(validChangeV2())
	require.NoError(t, err)

	cases := []struct {
		encoded    []byte
		action     string
		typ        string
		thresholds []string
		window     int64
		tolerance  int64
		interval   *int64
	}{
		{above, "price_above_threshold_v2", "above", []string{"100000.000000000000000000"},
			3 * outcomeDay, outcomeDay, nil},
		{below, "price_below_threshold_v2", "below", []string{"4.250000000000000000"},
			3 * outcomeDay, outcomeDay, nil},
		{rng, "value_in_range_v2", "range", []string{"90000.000000000000000000", ""},
			3 * outcomeDay, outcomeDay, nil},
		{change, "index_change_in_range_v2", "change_between",
			[]string{"2.000000000000000000", "3.000000000000000000"}, 7776000, 172800, i64Ptr(yearInSeconds)},
	}
	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			market, err := DecodeMarketData(tc.encoded)
			require.NoError(t, err)
			assert.Equal(t, tc.action, market.ActionID, "the action name tells v1 from v2")
			assert.Equal(t, tc.typ, market.Type)
			assert.Equal(t, tc.thresholds, market.Thresholds)
			assert.Equal(t, outcomeStream, market.StreamID)
			require.NotNil(t, market.Timestamp)
			assert.Equal(t, outcomeTimestamp, *market.Timestamp)
			require.NotNil(t, market.Window)
			assert.Equal(t, tc.window, *market.Window)
			require.NotNil(t, market.Tolerance)
			assert.Equal(t, tc.tolerance, *market.Tolerance)
			assert.Equal(t, tc.interval, market.TimeInterval)
			assert.Nil(t, market.FrozenAt, "v2 has no frozen_at")
			assert.Nil(t, market.BaseTime, "v2 has no base_time")
		})
	}
}

func TestOutcomeDecode_WrongArgCountLeavesTheMarketUnread(t *testing.T) {
	cases := []struct {
		action string
		args   []any
	}{
		{"price_above_threshold_v2", []any{outcomeProviderLower, outcomeStream, outcomeTimestamp, int64(10), int64(0)}},
		{"price_below_threshold_v2", []any{outcomeProviderLower, outcomeStream, outcomeTimestamp, int64(10), int64(0), "1", nil}},
		{"value_in_range_v2", []any{outcomeProviderLower, outcomeStream, outcomeTimestamp, int64(10), int64(0), "1"}},
		{"index_change_in_range_v2", []any{outcomeProviderLower, outcomeStream, outcomeTimestamp, int64(10), int64(0), yearInSeconds, "1"}},
	}
	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			argsBytes, err := EncodeActionArgs(tc.args)
			require.NoError(t, err)
			encoded, err := EncodeQueryComponents(outcomeProviderLower, outcomeStream, tc.action, argsBytes)
			require.NoError(t, err)

			market, err := DecodeMarketData(encoded)
			require.NoError(t, err)
			assert.Empty(t, market.Thresholds)
			assert.Nil(t, market.Timestamp, "an unreadable market keeps its timestamp nil")
			assert.Nil(t, market.Window)
			assert.Nil(t, market.Tolerance)
			assert.Nil(t, market.TimeInterval)
			assert.Error(t, requireQueryTime(1, market))
		})
	}
}

func TestOutcomeDecode_V1MarketsCarryNoWindow(t *testing.T) {
	encoded, err := BuildIndexChangeInRangeQueryComponents(validIndexChangeInput())
	require.NoError(t, err)
	market, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.Nil(t, market.Window)
	assert.Nil(t, market.Tolerance)

	encoded, err = BuildPriceAboveThresholdQueryComponents(types.PriceAboveThresholdInput{
		DataProvider: outcomeProviderLower, StreamID: outcomeStream, Timestamp: outcomeTimestamp,
		Threshold: "1"})
	require.NoError(t, err)
	market, err = DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.Equal(t, "above", market.Type)
	assert.Nil(t, market.Window)
	assert.Nil(t, market.Tolerance)
}

// ═══════════════════════════════════════════════════════════════
// BUCKET BOUNDS AND IDENTITY
// ═══════════════════════════════════════════════════════════════

func TestOutcomeBuckets_RangeIsHalfOpenWithOpenTails(t *testing.T) {
	lower, upper, err := BucketBoundsFromMarketData(&MarketData{Type: "range", Thresholds: []string{"1", "2"}})
	require.NoError(t, err)
	require.NotNil(t, lower)
	require.NotNil(t, upper)
	assert.InDelta(t, 1.0, *lower, 1e-12)
	assert.InDelta(t, 2.0, *upper, 1e-12)

	lower, upper, err = BucketBoundsFromMarketData(&MarketData{Type: "range", Thresholds: []string{"", "2"}})
	require.NoError(t, err)
	assert.Nil(t, lower)
	require.NotNil(t, upper)

	lower, upper, err = BucketBoundsFromMarketData(&MarketData{Type: "range", Thresholds: []string{"1", ""}})
	require.NoError(t, err)
	require.NotNil(t, lower)
	assert.Nil(t, upper)

	_, _, err = BucketBoundsFromMarketData(&MarketData{Type: "range", Thresholds: []string{"", ""}})
	assert.Error(t, err)
	_, _, err = BucketBoundsFromMarketData(&MarketData{Type: "range", Thresholds: []string{"2", "1"}})
	assert.Error(t, err)
	_, _, err = BucketBoundsFromMarketData(&MarketData{Type: "range", Thresholds: []string{"1"}})
	assert.Error(t, err)
}

func TestOutcomeBuckets_LadderTilesTheLine(t *testing.T) {
	// A ladder on value_in_range_v2 with open outer tails, built, decoded and
	// read back as bounds.
	edges := []struct{ min, max *string }{
		{nil, strPtr("90000")},
		{strPtr("90000"), strPtr("100000")},
		{strPtr("100000"), strPtr("110000")},
		{strPtr("110000"), nil},
	}
	var books [][2]*float64
	identity := ""
	for i, edge := range edges {
		input := validRangeV2()
		input.MinValue, input.MaxValue = edge.min, edge.max
		encoded, err := BuildValueInRangeV2QueryComponents(input)
		require.NoError(t, err)
		market, err := DecodeMarketData(encoded)
		require.NoError(t, err)
		require.NoError(t, requireQueryTime(i, market))

		id := marketIdentity(market, "eth_usdc", outcomeTimestamp)
		if i == 0 {
			identity = id
		}
		assert.Equal(t, identity, id, "every bucket of one ladder shares an identity")

		lower, upper, err := BucketBoundsFromMarketData(market)
		require.NoError(t, err)
		books = append(books, [2]*float64{lower, upper})
	}
	assert.Nil(t, books[0][0])
	assert.Nil(t, books[3][1])
	for i := 0; i+1 < len(books); i++ {
		require.NotNil(t, books[i][1])
		assert.Equal(t, *books[i][1], *books[i+1][0])
	}
}

func TestOutcomeIdentity_SeparatesSetsByWindowAndTolerance(t *testing.T) {
	encoded, err := BuildValueInRangeV2QueryComponents(validRangeV2())
	require.NoError(t, err)
	v2, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	want := marketIdentity(v2, "eth_usdc", outcomeTimestamp)

	otherWindow := validRangeV2()
	otherWindow.Window++
	encoded, err = BuildValueInRangeV2QueryComponents(otherWindow)
	require.NoError(t, err)
	market, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.NotEqual(t, want, marketIdentity(market, "eth_usdc", outcomeTimestamp), "different window")

	otherTolerance := validRangeV2()
	otherTolerance.Tolerance--
	encoded, err = BuildValueInRangeV2QueryComponents(otherTolerance)
	require.NoError(t, err)
	market, err = DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.NotEqual(t, want, marketIdentity(market, "eth_usdc", outcomeTimestamp), "different tolerance")

	// A v1 bucket on the same stream and timestamp reads the stream another way.
	v1Args, err := EncodeActionArgs([]any{outcomeProviderLower, outcomeStream, outcomeTimestamp,
		"90000", "110000", nil})
	require.NoError(t, err)
	encoded, err = EncodeQueryComponents(outcomeProviderLower, outcomeStream, "value_in_range", v1Args)
	require.NoError(t, err)
	v1, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.NotEqual(t, want, marketIdentity(v1, "eth_usdc", outcomeTimestamp), "v1 and v2 sets stay apart")

	// v2 threshold buckets with the same span join the range buckets' set.
	encoded, err = BuildPriceAboveThresholdV2QueryComponents(validAboveV2())
	require.NoError(t, err)
	above, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.Equal(t, want, marketIdentity(above, "eth_usdc", outcomeTimestamp))
}

// forecastSet decodes each bucket's query components and forecasts the set the
// way GetMarketForecast does, with one two-sided YES quote per bucket.
func forecastSet(t *testing.T, components [][]byte) *forecast.MarketForecast {
	t.Helper()
	quotes := [][2]int{{8, 14}, {20, 28}, {44, 52}, {12, 20}, {4, 10}}
	require.LessOrEqual(t, len(components), len(quotes))
	books := make([]forecast.BucketDepth, 0, len(components))
	markets := make([]*MarketData, 0, len(components))
	for i, encoded := range components {
		market, err := DecodeMarketData(encoded)
		require.NoError(t, err)
		require.NoError(t, requireQueryTime(i, market))
		markets = append(markets, market)

		lower, upper, err := BucketBoundsFromMarketData(market)
		require.NoError(t, err)
		bids, asks := laddersFromEntries([]types.OrderBookEntry{
			{Price: -quotes[i][0], Amount: 1000},
			{Price: quotes[i][1], Amount: 1000},
		})
		id := i + 1
		books = append(books, forecast.BucketDepth{
			Lower: lower, Upper: upper, YesBids: bids, YesAsks: asks, QueryID: &id,
		})
	}
	f := forecastFromBooks(books, hasOutcomeAction(markets))
	require.NotNil(t, f)
	return f
}

func TestOutcomeForecast_V2SetWarnsThatPricesIncludeTheRefund(t *testing.T) {
	const want = "v2 outcome markets: each YES price includes the $0.50 per share an " +
		"INVALID result pays, so these probabilities include that refund value and " +
		"are not a distribution over valid outcomes only"
	assert.Equal(t, want, ForecastOutcomeRefundWarning)

	// A v2 ladder: below 90000, [90000, 100000), [100000, 110000), above 110000.
	below := validBelowV2()
	below.Threshold = "90000"
	low := validRangeV2()
	low.MinValue, low.MaxValue = strPtr("90000"), strPtr("100000")
	high := validRangeV2()
	high.MinValue, high.MaxValue = strPtr("100000"), strPtr("110000")
	above := validAboveV2()
	above.Threshold = "110000"
	var v2 [][]byte
	for _, build := range []func() ([]byte, error){
		func() ([]byte, error) { return BuildPriceBelowThresholdV2QueryComponents(below) },
		func() ([]byte, error) { return BuildValueInRangeV2QueryComponents(low) },
		func() ([]byte, error) { return BuildValueInRangeV2QueryComponents(high) },
		func() ([]byte, error) { return BuildPriceAboveThresholdV2QueryComponents(above) },
	} {
		encoded, err := build()
		require.NoError(t, err)
		v2 = append(v2, encoded)
	}
	f := forecastSet(t, v2)
	assert.Contains(t, f.Warnings, ForecastOutcomeRefundWarning)
	count := 0
	for _, w := range f.Warnings {
		if w == ForecastOutcomeRefundWarning {
			count++
		}
	}
	assert.Equal(t, 1, count, "the warning is added once per set, not per bucket")

	// The same ladder on the v1 actions carries no such warning.
	var v1 [][]byte
	for _, bucket := range []struct {
		action string
		bounds []any
	}{
		{"price_below_threshold", []any{"90000"}},
		{"value_in_range", []any{"90000", "100000"}},
		{"value_in_range", []any{"100000", "110000"}},
		{"price_above_threshold", []any{"110000"}},
	} {
		args := append([]any{outcomeProviderLower, outcomeStream, outcomeTimestamp}, bucket.bounds...)
		args = append(args, nil)
		encodedArgs, err := EncodeActionArgs(args)
		require.NoError(t, err)
		encoded, err := EncodeQueryComponents(outcomeProviderLower, outcomeStream, bucket.action, encodedArgs)
		require.NoError(t, err)
		v1 = append(v1, encoded)
	}
	f = forecastSet(t, v1)
	assert.NotContains(t, f.Warnings, ForecastOutcomeRefundWarning)
	assert.False(t, hasWarning(f, "v2 outcome markets"))

	// Every outcome action counts, index_change_in_range_v2 (id 13) included.
	encoded, err := BuildIndexChangeInRangeV2QueryComponents(validChangeV2())
	require.NoError(t, err)
	change, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.True(t, hasOutcomeAction([]*MarketData{change}))
}

// ═══════════════════════════════════════════════════════════════
// RESULT PARSING
// ═══════════════════════════════════════════════════════════════

// buildOutcomePayload builds a canonical attestation payload (no signature)
// for actionID whose result holds the given event times and values.
func buildOutcomePayload(t *testing.T, actionID uint16, eventTimes, values []*big.Int) []byte {
	t.Helper()
	uint256Array, err := abi.NewType("uint256[]", "", nil)
	require.NoError(t, err)
	int256Array, err := abi.NewType("int256[]", "", nil)
	require.NoError(t, err)
	result, err := abi.Arguments{{Type: uint256Array}, {Type: int256Array}}.Pack(eventTimes, values)
	require.NoError(t, err)

	lp := func(b []byte) []byte {
		n := len(b)
		return append([]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}, b...)
	}
	payload := []byte{0x01, 0x00, 0, 0, 0, 0, 0, 0x12, 0xD6, 0x87}
	payload = append(payload, lp(make([]byte, 20))...)
	payload = append(payload, lp([]byte(outcomeStream))...)
	payload = append(payload, byte(actionID>>8), byte(actionID))
	payload = append(payload, lp([]byte{0x00, 0x00, 0x00, 0x00})...)
	payload = append(payload, lp(result)...)
	return payload
}

func TestParseOutcomeResult(t *testing.T) {
	e18 := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	at := []*big.Int{big.NewInt(1759276800)}

	for _, tc := range []struct {
		value *big.Int
		want  int
	}{
		{e18, OutcomeYes},
		{big.NewInt(0), OutcomeNo},
		{new(big.Int).Neg(e18), OutcomeInvalid},
	} {
		for _, id := range []uint16{13, 14, 15, 16} {
			outcome, actionID, err := ParseOutcomeResult(buildOutcomePayload(t, id, at, []*big.Int{tc.value}))
			require.NoError(t, err)
			assert.Equal(t, tc.want, outcome)
			assert.Equal(t, id, actionID)
		}
	}
	assert.Equal(t, 1, OutcomeYes)
	assert.Equal(t, 0, OutcomeNo)
	assert.Equal(t, -1, OutcomeInvalid)
}

func TestParseOutcomeResult_RefusesAnythingElse(t *testing.T) {
	e18 := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	at := []*big.Int{big.NewInt(1759276800)}

	for _, value := range []*big.Int{big.NewInt(1), big.NewInt(-1), new(big.Int).Mul(e18, big.NewInt(2))} {
		_, _, err := ParseOutcomeResult(buildOutcomePayload(t, 14, at, []*big.Int{value}))
		assert.Error(t, err, "value %s", value)
	}

	_, _, err := ParseOutcomeResult(buildOutcomePayload(t, 14, []*big.Int{}, []*big.Int{}))
	assert.Error(t, err, "no data point")
	_, _, err = ParseOutcomeResult(buildOutcomePayload(t, 14,
		[]*big.Int{at[0], at[0]}, []*big.Int{e18, e18}))
	assert.Error(t, err, "two data points")

	for _, id := range []uint16{1, 6, 12, 17} {
		_, actionID, err := ParseOutcomeResult(buildOutcomePayload(t, id, at, []*big.Int{e18}))
		require.Error(t, err, "id %d", id)
		assert.Contains(t, err.Error(), "not an outcome action")
		assert.Equal(t, id, actionID)
	}

	full := buildOutcomePayload(t, 14, at, []*big.Int{e18})
	for _, cut := range []int{0, 5, 20, 60, len(full) - 1} {
		_, _, err := ParseOutcomeResult(full[:cut])
		assert.Error(t, err, "payload cut to %d bytes", cut)
	}

	// A length prefix at or above 0x80000000 is refused, not converted to a
	// negative int (which would move the offset backward on a 32-bit build).
	for _, prefix := range []uint32{0x80000000, 0xFFFFFFFF} {
		huge := make([]byte, 10, 10+4)
		huge = binary.BigEndian.AppendUint32(huge, prefix)
		assert.NotPanics(t, func() {
			_, _, err := ParseOutcomeResult(huge)
			assert.Error(t, err, "length prefix %#x", prefix)
		})
	}
}

func TestParseBooleanResult_RefusesOutcomeActions(t *testing.T) {
	e18 := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	for _, id := range []uint16{13, 14, 15, 16} {
		_, actionID, err := ParseBooleanResult(
			buildOutcomePayload(t, id, []*big.Int{big.NewInt(1)}, []*big.Int{e18}))
		require.Error(t, err, "id %d", id)
		assert.Contains(t, err.Error(), "not a binary action")
		assert.Equal(t, id, actionID)
	}
}
