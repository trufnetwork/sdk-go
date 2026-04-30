package contractsapi

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// TestInsertMetadata_RejectsAllowZerosKey locks in the client-side
// guard that mirrors the node-side reservation: routing AllowZerosKey
// through the generic insertMetadata helper would let two parallel
// "latest" rows coexist and break the disable-then-insert atomicity
// that SetAllowZeros / set_allow_zeros own. We catch this at the SDK
// layer for a friendlier error than the eventual "use set_allow_zeros"
// from the node.
func TestInsertMetadata_RejectsAllowZerosKey(t *testing.T) {
	action := &Action{}

	streamId, err := util.NewStreamId("st00000000000000000000000000aabb")
	require.NoError(t, err)
	dp, err := util.NewEthereumAddressFromString("0x000000000000000000000000000000000000a110")
	require.NoError(t, err)

	_, err = action.insertMetadata(context.Background(), InsertMetadataInput{
		Stream: types.StreamLocator{StreamId: *streamId, DataProvider: dp},
		Key:    types.AllowZerosKey,
		Value:  types.NewMetadataValue(true),
	})

	require.Error(t, err, "insertMetadata must reject reserved key")
	require.True(t, errors.Is(err, ErrReservedMetadataKey),
		"error must wrap ErrReservedMetadataKey so callers can branch on it")
	require.Contains(t, err.Error(), "allow_zeros",
		"error message must name the offending key")
}

// TestDisableMetadataByRef_RejectsAllowZerosKey covers the dual path:
// disabling by ref skips the metadata lookup if the key is reserved,
// matching the insertMetadata guard so neither shortcut bypasses the
// dedicated SetAllowZeros mutator.
func TestDisableMetadataByRef_RejectsAllowZerosKey(t *testing.T) {
	action := &Action{}

	streamId, err := util.NewStreamId("st00000000000000000000000000aabb")
	require.NoError(t, err)
	dp, err := util.NewEthereumAddressFromString("0x000000000000000000000000000000000000a110")
	require.NoError(t, err)

	_, err = action.disableMetadataByRef(context.Background(), DisableMetadataByRefInput{
		Stream: types.StreamLocator{StreamId: *streamId, DataProvider: dp},
		Key:    types.AllowZerosKey,
		Ref:    "anything",
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrReservedMetadataKey))
}

// TestInsertMetadata_AllowsOtherKeys is a guardrail for the guard
// itself: a regression that broadens the rejection to any metadata key
// would brick the existing typed wrappers (SetReadVisibility, etc.).
// For each non-reserved key we actually invoke the helper and require
// that it gets PAST the guard. The follow-on s.execute call has a nil
// _client and panics — that panic is the proof the guard let us through.
// What matters is no ErrReservedMetadataKey is returned for a non-reserved key.
func TestInsertMetadata_AllowsOtherKeys(t *testing.T) {
	streamId, err := util.NewStreamId("st00000000000000000000000000aabb")
	require.NoError(t, err)
	dp, err := util.NewEthereumAddressFromString("0x000000000000000000000000000000000000a110")
	require.NoError(t, err)

	cases := []struct {
		key types.MetadataKey
		val types.MetadataValue
	}{
		{types.ReadVisibilityKey, types.NewMetadataValue(0)},
		{types.ComposeVisibilityKey, types.NewMetadataValue(0)},
		{types.AllowReadWalletKey, types.NewMetadataValue("0x0000000000000000000000000000000000000001")},
	}

	for _, tc := range cases {
		t.Run(string(tc.key), func(t *testing.T) {
			action := &Action{}
			tripped := callInsertMetadataChecked(t, action, types.StreamLocator{
				StreamId: *streamId, DataProvider: dp,
			}, tc.key, tc.val)
			require.False(t, tripped, "guard must not trip for non-reserved key %s", tc.key)
		})
	}
}

// callInsertMetadataChecked invokes insertMetadata against an Action with
// no client and returns whether ErrReservedMetadataKey was raised. The
// nil-client panic past the guard is treated as "guard passed" since
// the test's only concern is which keys the guard rejects.
func callInsertMetadataChecked(t *testing.T, action *Action, stream types.StreamLocator, key types.MetadataKey, val types.MetadataValue) (tripped bool) {
	t.Helper()
	defer func() {
		_ = recover() // nil _client deref past the guard — expected
	}()
	_, err := action.insertMetadata(context.Background(), InsertMetadataInput{
		Stream: stream,
		Key:    key,
		Value:  val,
	})
	if err != nil && errors.Is(err, ErrReservedMetadataKey) {
		tripped = true
	}
	return tripped
}
