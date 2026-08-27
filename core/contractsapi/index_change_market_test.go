// Tests for index_change_in_range (action 12) market construction and decoding.
//
// This action differs from the 040 binary family in two ways that the tests
// below are built around: it takes eight arguments rather than five or six, so
// the bounds sit at indices 5 and 6 and frozen_at at 7; and either bound may be
// NULL to strike an open tail, which no other binary action allows.

package contractsapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
)

const (
	indexChangeProvider = "0x4710a8d8f0d845da110086812a32de6d90d7ff5c"
	indexChangeStream   = "stcpiyoy000000000000000000000000"
	yearInSeconds       = int64(31536000)
)

func strPtr(s string) *string { return &s }
func i64Ptr(v int64) *int64   { return &v }

// validIndexChangeInput is an interior bucket: both bounds struck, no base_time,
// no frozen_at.
func validIndexChangeInput() types.IndexChangeInRangeInput {
	return types.IndexChangeInRangeInput{
		DataProvider: indexChangeProvider,
		StreamID:     indexChangeStream,
		Timestamp:    1735689600,
		TimeInterval: yearInSeconds,
		MinChange:    strPtr("2"),
		MaxChange:    strPtr("3"),
	}
}

// ═══════════════════════════════════════════════════════════════
// INPUT VALIDATION
// ═══════════════════════════════════════════════════════════════

func TestIndexChangeInput_InteriorBucketIsAccepted(t *testing.T) {
	input := validIndexChangeInput()
	require.NoError(t, input.Validate())
	assert.Equal(t, "index_change_in_range", input.ActionName())
}

func TestIndexChangeInput_EitherTailMayBeOpen(t *testing.T) {
	bottom := validIndexChangeInput()
	bottom.MinChange = nil
	assert.NoError(t, bottom.Validate(), "the bottom bucket of a set is struck open below")

	top := validIndexChangeInput()
	top.MaxChange = nil
	assert.NoError(t, top.Validate(), "the top bucket of a set is struck open above")
}

func TestIndexChangeInput_BothTailsOpenIsRejected(t *testing.T) {
	input := validIndexChangeInput()
	input.MinChange = nil
	input.MaxChange = nil

	err := input.Validate()
	require.Error(t, err, "a bucket covering the whole number line settles TRUE always")
	assert.Contains(t, err.Error(), "at least one of min_change or max_change")
}

func TestIndexChangeInput_EmptyStringBoundIsRejected(t *testing.T) {
	// An empty string is how a decoded open tail reads back, not how one is
	// struck. Accepting it here would encode NUMERIC("") and fail at the node
	// instead of locally.
	input := validIndexChangeInput()
	input.MinChange = strPtr("")
	err := input.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use nil for an open tail")
}

func TestIndexChangeInput_TimeIntervalMustBePositive(t *testing.T) {
	for _, interval := range []int64{0, -1, -yearInSeconds} {
		input := validIndexChangeInput()
		input.TimeInterval = interval
		err := input.Validate()
		require.Error(t, err, "interval %d", interval)
		assert.Contains(t, err.Error(), "time_interval must be positive")
	}
}

func TestIndexChangeInput_MalformedIdentifiersAreRejected(t *testing.T) {
	short := validIndexChangeInput()
	short.StreamID = "sttooshort"
	assert.Error(t, short.Validate())

	unprefixed := validIndexChangeInput()
	unprefixed.DataProvider = "4710a8d8f0d845da110086812a32de6d90d7ff5cXX"
	assert.Error(t, unprefixed.Validate())

	untimed := validIndexChangeInput()
	untimed.Timestamp = 0
	assert.Error(t, untimed.Validate())
}

// ═══════════════════════════════════════════════════════════════
// ARGUMENT ENCODING
// ═══════════════════════════════════════════════════════════════

// decodeBuiltArgs runs an input through the real encoder and hands back the
// decoded argument list, which is what the node action sees positionally.
func decodeBuiltArgs(t *testing.T, input types.IndexChangeInRangeInput) []any {
	t.Helper()

	encoded, err := BuildIndexChangeInRangeQueryComponents(input)
	require.NoError(t, err)

	_, _, actionID, argsBytes, err := DecodeQueryComponents(encoded)
	require.NoError(t, err)
	require.Equal(t, "index_change_in_range", actionID)

	args, err := DecodeActionArgs(argsBytes)
	require.NoError(t, err)
	return args
}

func TestIndexChangeArgs_MatchTheActionSignaturePositionally(t *testing.T) {
	// index_change_in_range($data_provider, $stream_id, $timestamp, $base_time,
	//                       $time_interval, $min_change, $max_change, $frozen_at)
	//
	// Encoding is purely positional, so this order is the contract with the node.
	// A market built against a different order is not wrong at creation, it is
	// permanently unsettleable.
	input := validIndexChangeInput()
	input.BaseTime = i64Ptr(1600000000)
	input.FrozenAt = i64Ptr(987654)

	args := decodeBuiltArgs(t, input)
	require.Len(t, args, 8)

	assert.Equal(t, indexChangeProvider, derefString(args[0]))
	assert.Equal(t, indexChangeStream, derefString(args[1]))
	assert.Equal(t, int64(1735689600), derefInt64(t, args[2]))
	assert.Equal(t, int64(1600000000), derefInt64(t, args[3]), "base_time is argument 3")
	assert.Equal(t, yearInSeconds, derefInt64(t, args[4]), "time_interval is argument 4")
	assert.Equal(t, "2.000000000000000000", args[5].(*kwiltypes.Decimal).String())
	assert.Equal(t, "3.000000000000000000", args[6].(*kwiltypes.Decimal).String())
	assert.Equal(t, int64(987654), derefInt64(t, args[7]), "frozen_at is last")
}

func TestIndexChangeArgs_OpenTailEncodesAsNullInPlace(t *testing.T) {
	// The open bound has to stay NULL in its own slot. Dropping it would shift
	// the other bound into the wrong argument and strike the market at a
	// different number than asked for.
	bottom := validIndexChangeInput()
	bottom.MinChange = nil

	args := decodeBuiltArgs(t, bottom)
	require.Len(t, args, 8, "an open tail is a NULL argument, not a missing one")
	assert.Nil(t, args[5], "min_change is NULL")
	require.NotNil(t, args[6])
	assert.Equal(t, "3.000000000000000000", args[6].(*kwiltypes.Decimal).String())

	top := validIndexChangeInput()
	top.MaxChange = nil

	args = decodeBuiltArgs(t, top)
	require.Len(t, args, 8)
	require.NotNil(t, args[5])
	assert.Equal(t, "2.000000000000000000", args[5].(*kwiltypes.Decimal).String())
	assert.Nil(t, args[6], "max_change is NULL")
}

func TestIndexChangeArgs_OmittedBaseTimeAndFrozenAtEncodeAsNull(t *testing.T) {
	args := decodeBuiltArgs(t, validIndexChangeInput())
	require.Len(t, args, 8)
	assert.Nil(t, args[3], "no base_time means the stream's default")
	assert.Nil(t, args[7], "no frozen_at means latest")
}

func TestIndexChangeArgs_NegativeBoundsSurvive(t *testing.T) {
	// Rates of change go negative, unlike every threshold the 040 family carries.
	input := validIndexChangeInput()
	input.MinChange = strPtr("-1.5")
	input.MaxChange = strPtr("-0.5")

	args := decodeBuiltArgs(t, input)
	assert.Equal(t, "-1.500000000000000000", args[5].(*kwiltypes.Decimal).String())
	assert.Equal(t, "-0.500000000000000000", args[6].(*kwiltypes.Decimal).String())
}

func TestIndexChangeArgs_InvertedBoundsAreRejected(t *testing.T) {
	// The node refuses min >= max. A market that gets past this check would be
	// created, attested and then error at settlement, which is unrecoverable.
	inverted := validIndexChangeInput()
	inverted.MinChange = strPtr("3")
	inverted.MaxChange = strPtr("2")

	encoded, err := BuildIndexChangeInRangeQueryComponents(inverted)
	require.Error(t, err)
	assert.Nil(t, encoded)
	assert.Contains(t, err.Error(), "min_change must be less than max_change")

	empty := validIndexChangeInput()
	empty.MinChange = strPtr("2")
	empty.MaxChange = strPtr("2")

	_, err = BuildIndexChangeInRangeQueryComponents(empty)
	require.Error(t, err, "a half-open bucket with equal edges holds nothing")
}

func TestIndexChangeArgs_BoundsAreComparedAtActionPrecision(t *testing.T) {
	// Compared as strings, "2.0" < "2" is false but "10" < "9" is true. Both
	// orderings have to come from the decimal comparison, not the text.
	equal := validIndexChangeInput()
	equal.MinChange = strPtr("2.0")
	equal.MaxChange = strPtr("2")
	_, err := BuildIndexChangeInRangeQueryComponents(equal)
	require.Error(t, err, "2.0 and 2 are the same number")

	ordered := validIndexChangeInput()
	ordered.MinChange = strPtr("9")
	ordered.MaxChange = strPtr("10")
	_, err = BuildIndexChangeInRangeQueryComponents(ordered)
	require.NoError(t, err, "9 < 10 despite sorting the other way as text")
}

func TestIndexChangeArgs_OpenTailSkipsTheOrderCheck(t *testing.T) {
	// Only one bound to compare, so there is no ordering to enforce.
	bottom := validIndexChangeInput()
	bottom.MinChange = nil
	bottom.MaxChange = strPtr("-5")
	_, err := BuildIndexChangeInRangeQueryComponents(bottom)
	assert.NoError(t, err)

	top := validIndexChangeInput()
	top.MinChange = strPtr("100")
	top.MaxChange = nil
	_, err = BuildIndexChangeInRangeQueryComponents(top)
	assert.NoError(t, err)
}

func TestIndexChangeArgs_UnparseableBoundIsRejectedLocally(t *testing.T) {
	input := validIndexChangeInput()
	input.MaxChange = strPtr("three percent")

	encoded, err := BuildIndexChangeInRangeQueryComponents(input)
	require.Error(t, err)
	assert.Nil(t, encoded)
	assert.Contains(t, err.Error(), "invalid max_change")
}

func TestIndexChangeArgs_InvalidInputNeverReachesTheEncoder(t *testing.T) {
	input := validIndexChangeInput()
	input.TimeInterval = 0

	encoded, err := BuildIndexChangeInRangeQueryComponents(input)
	require.Error(t, err)
	assert.Nil(t, encoded)
	assert.Contains(t, err.Error(), "invalid input")
}

// ═══════════════════════════════════════════════════════════════
// DECODING
// ═══════════════════════════════════════════════════════════════

func TestIndexChangeDecode_CarriesTheQueryTimeFromTheRightSlots(t *testing.T) {
	// Eight arguments push frozen_at to index 7. Reading it at 5, where the 040
	// range markets keep it, would return time_interval as a block height.
	input := validIndexChangeInput()
	input.FrozenAt = i64Ptr(987654)

	encoded, err := BuildIndexChangeInRangeQueryComponents(input)
	require.NoError(t, err)

	market, err := DecodeMarketData(encoded)
	require.NoError(t, err)

	assert.Equal(t, "change_between", market.Type)
	require.NotNil(t, market.Timestamp)
	assert.Equal(t, int64(1735689600), *market.Timestamp)
	require.NotNil(t, market.FrozenAt)
	assert.Equal(t, int64(987654), *market.FrozenAt)
	assert.Equal(t, []string{"2.000000000000000000", "3.000000000000000000"}, market.Thresholds)
}

func TestIndexChangeDecode_CarriesWhatTheChangeIsMeasuredOver(t *testing.T) {
	// base_time and time_interval are not strikes, so they stay out of
	// Thresholds -- but they change the question, so a market that dropped them
	// would be indistinguishable from one measuring a different interval.
	input := validIndexChangeInput()
	input.BaseTime = i64Ptr(1600000000)

	encoded, err := BuildIndexChangeInRangeQueryComponents(input)
	require.NoError(t, err)

	market, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	require.NotNil(t, market.BaseTime)
	assert.Equal(t, int64(1600000000), *market.BaseTime)
	require.NotNil(t, market.TimeInterval)
	assert.Equal(t, yearInSeconds, *market.TimeInterval)

	// An absent base date stays nil rather than becoming a zero.
	input.BaseTime = nil
	encoded, err = BuildIndexChangeInRangeQueryComponents(input)
	require.NoError(t, err)
	market, err = DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.Nil(t, market.BaseTime)
	require.NotNil(t, market.TimeInterval)
	assert.Equal(t, yearInSeconds, *market.TimeInterval)
}

func TestIndexChangeDecode_ValueMarketsCarryNoInterval(t *testing.T) {
	// Only index_change_in_range has these arguments. A value market having no
	// interval is what keeps it out of a percent-change bucket set.
	argsBytes, err := EncodeActionArgs(
		[]any{indexChangeProvider, indexChangeStream, int64(1735689600), "1", "2", nil})
	require.NoError(t, err)
	encoded, err := EncodeQueryComponents(
		indexChangeProvider, indexChangeStream, "value_in_range", argsBytes)
	require.NoError(t, err)

	market, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.Equal(t, "between", market.Type)
	assert.Nil(t, market.BaseTime)
	assert.Nil(t, market.TimeInterval)
}

func TestIndexChangeDecode_OpenTailKeepsTheOtherBoundInPlace(t *testing.T) {
	input := validIndexChangeInput()
	input.MinChange = nil

	encoded, err := BuildIndexChangeInRangeQueryComponents(input)
	require.NoError(t, err)

	market, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	require.Len(t, market.Thresholds, 2, "an open tail holds its slot")
	assert.Equal(t, "", market.Thresholds[0])
	assert.Equal(t, "3.000000000000000000", market.Thresholds[1])
}

func TestIndexChangeDecode_TruncatedArgsLeaveTheMarketUnread(t *testing.T) {
	// A market missing its tail arguments must not read as a healthy one. Six
	// arguments is exactly the shape a value_in_range market has, so this is the
	// case most likely to decode into something plausible.
	argsBytes, err := EncodeActionArgs(
		[]any{indexChangeProvider, indexChangeStream, int64(1735689600), nil, yearInSeconds, "2"})
	require.NoError(t, err)

	encoded, err := EncodeQueryComponents(
		indexChangeProvider, indexChangeStream, "index_change_in_range", argsBytes)
	require.NoError(t, err)

	market, err := DecodeMarketData(encoded)
	require.NoError(t, err)
	assert.Equal(t, "change_between", market.Type)
	assert.Empty(t, market.Thresholds)
	assert.Nil(t, market.Timestamp, "an unreadable market keeps its timestamp nil")
	assert.Nil(t, market.FrozenAt)
	assert.Nil(t, market.BaseTime, "a truncated market reports nothing it cannot read")
	assert.Nil(t, market.TimeInterval)
}

// ═══════════════════════════════════════════════════════════════
// BUCKET BOUNDS
// ═══════════════════════════════════════════════════════════════

func TestIndexChangeBuckets_InteriorBucketIsHalfOpen(t *testing.T) {
	lower, upper, err := BucketBoundsFromMarketData(
		&MarketData{Type: "change_between", Thresholds: []string{"2", "3"}})
	require.NoError(t, err)
	require.NotNil(t, lower)
	require.NotNil(t, upper)
	assert.InDelta(t, 2.0, *lower, 1e-12)
	assert.InDelta(t, 3.0, *upper, 1e-12)
}

func TestIndexChangeBuckets_OpenTailsBecomeNilBounds(t *testing.T) {
	lower, upper, err := BucketBoundsFromMarketData(
		&MarketData{Type: "change_between", Thresholds: []string{"", "1"}})
	require.NoError(t, err)
	assert.Nil(t, lower, "the bottom bucket is open below")
	require.NotNil(t, upper)
	assert.InDelta(t, 1.0, *upper, 1e-12)

	lower, upper, err = BucketBoundsFromMarketData(
		&MarketData{Type: "change_between", Thresholds: []string{"4", ""}})
	require.NoError(t, err)
	require.NotNil(t, lower)
	assert.InDelta(t, 4.0, *lower, 1e-12)
	assert.Nil(t, upper, "the top bucket is open above")
}

func TestIndexChangeBuckets_NegativeBoundsAreOrdinary(t *testing.T) {
	lower, upper, err := BucketBoundsFromMarketData(
		&MarketData{Type: "change_between", Thresholds: []string{"-1.5", "-0.5"}})
	require.NoError(t, err)
	require.NotNil(t, lower)
	require.NotNil(t, upper)
	assert.InDelta(t, -1.5, *lower, 1e-12)
	assert.InDelta(t, -0.5, *upper, 1e-12)
}

func TestIndexChangeBuckets_BothTailsOpenIsRejected(t *testing.T) {
	_, _, err := BucketBoundsFromMarketData(
		&MarketData{Type: "change_between", Thresholds: []string{"", ""}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one bound")
}

func TestIndexChangeBuckets_InvertedAndEmptyBucketsAreRejected(t *testing.T) {
	_, _, err := BucketBoundsFromMarketData(
		&MarketData{Type: "change_between", Thresholds: []string{"3", "2"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lower < upper")

	_, _, err = BucketBoundsFromMarketData(
		&MarketData{Type: "change_between", Thresholds: []string{"2", "2"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lower < upper")
}

func TestIndexChangeBuckets_MissingSlotIsRejected(t *testing.T) {
	// Distinct from an open tail: no slot at all means the market did not decode.
	_, _, err := BucketBoundsFromMarketData(
		&MarketData{Type: "change_between", Thresholds: []string{"2"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "threshold slot")

	_, _, err = BucketBoundsFromMarketData(
		&MarketData{Type: "change_between", Thresholds: []string{}})
	require.Error(t, err)
}

func TestIndexChangeBuckets_NonFiniteBoundIsRejected(t *testing.T) {
	for _, bound := range []string{"NaN", "Inf", "+Inf", "-Inf"} {
		_, _, err := BucketBoundsFromMarketData(
			&MarketData{Type: "change_between", Thresholds: []string{bound, "3"}})
		require.Error(t, err, "bound %q", bound)
	}
}

func TestIndexChangeBuckets_RoundTripThroughRealQueryComponents(t *testing.T) {
	// The whole chain a forecast consumer walks: build, decode, derive bounds.
	// Five buckets tiling the line, struck the way a market set is struck.
	edges := []struct {
		min, max *string
	}{
		{nil, strPtr("1")},
		{strPtr("1"), strPtr("2")},
		{strPtr("2"), strPtr("3")},
		{strPtr("3"), strPtr("4")},
		{strPtr("4"), nil},
	}

	var bounds [][2]*float64
	for _, edge := range edges {
		input := validIndexChangeInput()
		input.MinChange = edge.min
		input.MaxChange = edge.max

		encoded, err := BuildIndexChangeInRangeQueryComponents(input)
		require.NoError(t, err)

		market, err := DecodeMarketData(encoded)
		require.NoError(t, err)

		lower, upper, err := BucketBoundsFromMarketData(market)
		require.NoError(t, err)
		bounds = append(bounds, [2]*float64{lower, upper})
	}

	require.Len(t, bounds, 5)
	assert.Nil(t, bounds[0][0])
	assert.Nil(t, bounds[4][1])

	// Each bucket's upper edge is the next one's lower edge, so the set tiles
	// with no gap and no overlap.
	for i := 0; i < len(bounds)-1; i++ {
		require.NotNil(t, bounds[i][1])
		require.NotNil(t, bounds[i+1][0])
		assert.InDelta(t, *bounds[i][1], *bounds[i+1][0], 1e-12, "bucket %d meets bucket %d", i, i+1)
	}
}

// derefString reads a decoded TEXT argument, which comes back as *string.
func derefString(arg any) string {
	switch v := arg.(type) {
	case string:
		return v
	case *string:
		if v == nil {
			return ""
		}
		return *v
	}
	return ""
}

// derefInt64 reads a decoded INT8 argument and fails the test if it is NULL.
func derefInt64(t *testing.T, arg any) int64 {
	t.Helper()
	value := argInt64([]any{arg}, 0)
	require.NotNil(t, value, "expected an integer argument, got %v", arg)
	return *value
}
