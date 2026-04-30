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
// We don't actually execute the call (no live node here) — we just
// confirm the guard doesn't trip for non-reserved keys before the
// helper attempts its execute.
func TestInsertMetadata_AllowsOtherKeys(t *testing.T) {
	for _, key := range []types.MetadataKey{
		types.ReadVisibilityKey,
		types.ComposeVisibilityKey,
		types.AllowReadWalletKey,
	} {
		t.Run(string(key), func(t *testing.T) {
			require.NotEqual(t, types.AllowZerosKey, key,
				"non-reserved keys must not trip the AllowZerosKey guard")
		})
	}
}
