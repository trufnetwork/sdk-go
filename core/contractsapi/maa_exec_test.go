package contractsapi

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	kwilTypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
)

// goldenMAAExecHex is the frozen zero-argument maa_exec wire vector, byte-identical to kwil-db's
// core/types/payloads_maa_test.go. Asserting against it locks the Go SDK's maa_exec construction to
// the canonical on-chain wire: one byte of disagreement would route the call to the wrong action.
const goldenMAAExecHex = "0000" + // uint16 version = 0 (little-endian)
	"14000000" + "1111111111111111111111111111111111111111" + // maa_address: len=20, then 20 bytes
	"04000000" + "6d61696e" + // namespace "main": len=4, then bytes
	"0e000000" + "6f625f706c6163655f6f72646572" + // action "ob_place_order": len=14, then bytes
	"0000" // uint16 numArgs = 0

func TestBuildMAAExec_GoldenVector(t *testing.T) {
	p, err := buildMAAExec(types.MAAExecuteInput{
		MAAAddress: bytes.Repeat([]byte{0x11}, 20),
		Namespace:  "main",
		Action:     "ob_place_order",
	})
	require.NoError(t, err)

	b, err := p.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, goldenMAAExecHex, hex.EncodeToString(b),
		"maa_exec wire must match the frozen golden vector")
}

func TestBuildMAAExec_NamespaceDefaultsToMain(t *testing.T) {
	p, err := buildMAAExec(types.MAAExecuteInput{
		MAAAddress: bytes.Repeat([]byte{0x11}, 20),
		Namespace:  "", // empty namespace must normalize to "main"
		Action:     "ob_place_order",
	})
	require.NoError(t, err)
	require.Equal(t, "main", p.Namespace)
}

func TestBuildMAAExec_Validation(t *testing.T) {
	good := bytes.Repeat([]byte{0x11}, 20)

	_, err := buildMAAExec(types.MAAExecuteInput{MAAAddress: good[:19], Action: "x"})
	require.Error(t, err, "a 19-byte address must be rejected")

	_, err = buildMAAExec(types.MAAExecuteInput{MAAAddress: good, Action: ""})
	require.Error(t, err, "an empty action must be rejected")
}

func TestBuildMAAExec_EncodesArgsAndRoundTrips(t *testing.T) {
	p, err := buildMAAExec(types.MAAExecuteInput{
		MAAAddress: bytes.Repeat([]byte{0x22}, 20),
		Namespace:  "main",
		Action:     "place_buy_order",
		Args:       []any{"0xabc", int64(42)},
	})
	require.NoError(t, err)
	require.Len(t, p.Arguments, 2, "arguments are encoded one EncodedValue per arg")

	b, err := p.MarshalBinary()
	require.NoError(t, err)
	var got kwilTypes.MAAExec
	require.NoError(t, got.UnmarshalBinary(b))
	require.Equal(t, p.MAAAddress, got.MAAAddress)
	require.Equal(t, p.Namespace, got.Namespace)
	require.Equal(t, p.Action, got.Action)
	require.Len(t, got.Arguments, 2)
}
