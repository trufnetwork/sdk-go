package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/util"
)

// TestAllowZerosKeyType asserts the new metadata key maps to bool —
// failing here surfaces a wiring mismatch that would otherwise only show
// up at runtime when set_allow_zeros is invoked through insert_metadata.
func TestAllowZerosKeyType(t *testing.T) {
	require.Equal(t, MetadataTypeBool, AllowZerosKey.GetType())
	require.Equal(t, "allow_zeros", AllowZerosKey.String())
}

// TestStreamDefinitionAllowZerosDefault locks in the additive-field
// guarantee: a StreamDefinition built without setting AllowZeros must
// default to false, which is what BatchDeployStreams relies on to keep
// today's behavior for existing callers.
func TestStreamDefinitionAllowZerosDefault(t *testing.T) {
	id, err := util.NewStreamId("st00000000000000000000000000aabb")
	require.NoError(t, err)

	def := StreamDefinition{
		StreamId:   *id,
		StreamType: StreamTypePrimitive,
	}
	require.False(t, def.AllowZeros, "zero-value StreamDefinition must have AllowZeros=false")

	defOptIn := StreamDefinition{
		StreamId:   *id,
		StreamType: StreamTypePrimitive,
		AllowZeros: true,
	}
	require.True(t, defOptIn.AllowZeros)
}
