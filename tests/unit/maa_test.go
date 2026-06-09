package unit

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/util"
)

// Golden vectors for the Modular Agent Address (MAA) derivation. They are frozen network-wide and are
// asserted byte-for-byte by the node precompiles and every SDK — a mismatch here means this SDK would
// derive a different agent-wallet address than the chain, sending funds to the wrong wallet. Keep these
// in lockstep with node extensions/tn_utils/maa_test.go and the spec.

func maaHexBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	require.NoError(t, err, "bad hex %q", s)
	return b
}

func maaRepeatByte(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func TestMAAComputeRulesHash_GoldenVectors(t *testing.T) {
	// Vector A — bps fee, two actions (one body-pinned). Input order is place,cancel to prove the
	// canonical sort (cancel < place) is applied regardless of input order. Token-agnostic (no bridge).
	rhA, err := util.ComputeRulesHash(
		"bps", 250, "0",
		[]string{"main", "main"},
		[]string{"ob_place_order", "ob_cancel_order"},
		[][]byte{maaRepeatByte(0xcc, 32), nil},
	)
	require.NoError(t, err)
	require.Equal(t,
		maaHexBytes(t, "df0555d336647bec5e9fe1f6f613086bddf53548b67c52393aef6db4cbef062d"),
		rhA, "vector A rules_hash")

	// Vector B — flat fee 1e18, empty allow-list.
	rhB, err := util.ComputeRulesHash("flat", 0, "1000000000000000000", nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t,
		maaHexBytes(t, "0b1edb0ad70fb94287e50c7b3deaea7bba4e500c4ae6a764ed9021faf091274a"),
		rhB, "vector B rules_hash")
}

func TestMAADeriveRuleID_GoldenVectors(t *testing.T) {
	restricted := maaRepeatByte(0x11, 20)

	rhA := maaHexBytes(t, "df0555d336647bec5e9fe1f6f613086bddf53548b67c52393aef6db4cbef062d")
	idA, err := util.DeriveRuleID(restricted, rhA, maaRepeatByte(0xab, 32))
	require.NoError(t, err)
	require.Equal(t,
		maaHexBytes(t, "a0b517da759b794e2484dc8b9dba8f5211a53dcdf26448f19c7c68699ff7bcf1"),
		idA, "vector A rule_id")
	require.Len(t, idA, 32, "rule_id must be 32 bytes (untruncated)")

	// Empty salt.
	rhB := maaHexBytes(t, "0b1edb0ad70fb94287e50c7b3deaea7bba4e500c4ae6a764ed9021faf091274a")
	idB, err := util.DeriveRuleID(restricted, rhB, nil)
	require.NoError(t, err)
	require.Equal(t,
		maaHexBytes(t, "21f40fbf0fd537f85d283cf7b5f2fe8602c1f4b910aad96ad2dad9f6e82b1ca5"),
		idB, "vector B rule_id")
}

func TestMAADeriveMAAAddress_GoldenVectors(t *testing.T) {
	unrestricted := maaRepeatByte(0x22, 20)
	restricted := maaRepeatByte(0x11, 20)

	idA := maaHexBytes(t, "a0b517da759b794e2484dc8b9dba8f5211a53dcdf26448f19c7c68699ff7bcf1")
	addrA, err := util.DeriveMAAAddress(unrestricted, restricted, idA)
	require.NoError(t, err)
	require.Equal(t,
		maaHexBytes(t, "84da4dbca14d429c719d65a0bb76bd7fa3c5c349"),
		addrA, "vector A maa_address")
	require.Len(t, addrA, 20, "maa_address must be 20 bytes")

	idB := maaHexBytes(t, "21f40fbf0fd537f85d283cf7b5f2fe8602c1f4b910aad96ad2dad9f6e82b1ca5")
	addrB, err := util.DeriveMAAAddress(unrestricted, restricted, idB)
	require.NoError(t, err)
	require.Equal(t,
		maaHexBytes(t, "cb009e348c3ad795aa6d7d81177f0daee4583128"),
		addrB, "vector B maa_address")
}

// End-to-end: from raw inputs straight through to the wallet address, the path a funder uses to know
// where to send funds before the wallet exists.
func TestMAAEndToEndDerivation(t *testing.T) {
	restricted := maaRepeatByte(0x11, 20)
	unrestricted := maaRepeatByte(0x22, 20)

	rulesHash, err := util.ComputeRulesHash(
		"bps", 250, "0",
		[]string{"main", "main"},
		[]string{"ob_place_order", "ob_cancel_order"},
		[][]byte{maaRepeatByte(0xcc, 32), nil},
	)
	require.NoError(t, err)
	ruleID, err := util.DeriveRuleID(restricted, rulesHash, maaRepeatByte(0xab, 32))
	require.NoError(t, err)
	addr, err := util.DeriveMAAAddress(unrestricted, restricted, ruleID)
	require.NoError(t, err)
	require.Equal(t, "0x84da4dbca14d429c719d65a0bb76bd7fa3c5c349", "0x"+hex.EncodeToString(addr))
}

func TestMAAComputeRulesHash_OrderIndependentAndDedup(t *testing.T) {
	base, err := util.ComputeRulesHash("bps", 250, "0",
		[]string{"main", "main"},
		[]string{"ob_place_order", "ob_cancel_order"},
		[][]byte{maaRepeatByte(0xcc, 32), nil})
	require.NoError(t, err)

	// Reversed input order must produce the same hash (canonical sort).
	reordered, err := util.ComputeRulesHash("bps", 250, "0",
		[]string{"main", "main"},
		[]string{"ob_cancel_order", "ob_place_order"},
		[][]byte{nil, maaRepeatByte(0xcc, 32)})
	require.NoError(t, err)
	require.True(t, bytes.Equal(base, reordered), "reordered allow-list changed the hash")

	// A duplicate (namespace, action) must not change the hash (dedup).
	deduped, err := util.ComputeRulesHash("bps", 250, "0",
		[]string{"main", "main", "main"},
		[]string{"ob_place_order", "ob_cancel_order", "ob_place_order"},
		[][]byte{maaRepeatByte(0xcc, 32), nil, maaRepeatByte(0xcc, 32)})
	require.NoError(t, err)
	require.True(t, bytes.Equal(base, deduped), "duplicate entry changed the hash")

	// Conflicting body_hash for a duplicate (namespace, action): the LAST occurrence wins. The earlier
	// 0xdd pin on ob_place_order is dropped in favor of the trailing 0xcc, so the result equals base.
	lastWins, err := util.ComputeRulesHash("bps", 250, "0",
		[]string{"main", "main", "main"},
		[]string{"ob_place_order", "ob_cancel_order", "ob_place_order"},
		[][]byte{maaRepeatByte(0xdd, 32), nil, maaRepeatByte(0xcc, 32)})
	require.NoError(t, err)
	require.True(t, bytes.Equal(base, lastWins), "last-write-wins not honored for a conflicting body_hash")
}

func TestMAAComputeRulesHash_DedupSeparatorNoCollision(t *testing.T) {
	// ("a b","c") and ("a","b c") are DISTINCT pairs. The dedup key must separate namespace from
	// action with a NUL byte (not a space) so they never collide; otherwise `both` would collapse to
	// the last entry and equal `onlySecond`. Locks cross-language parity (node maa.go / sdk-js / sdk-py).
	both, err := util.ComputeRulesHash("bps", 0, "0", []string{"a b", "a"}, []string{"c", "b c"}, nil)
	require.NoError(t, err)
	onlySecond, err := util.ComputeRulesHash("bps", 0, "0", []string{"a"}, []string{"b c"}, nil)
	require.NoError(t, err)
	require.False(t, bytes.Equal(both, onlySecond), "distinct (namespace, action) pairs must not collide")
}

func TestMAAComputeRulesHash_Validation(t *testing.T) {
	_, err := util.ComputeRulesHash("bogus", 0, "0", nil, nil, nil)
	require.Error(t, err, "bad fee_mode")

	_, err = util.ComputeRulesHash("bps", 0, "-1", nil, nil, nil)
	require.Error(t, err, "negative fee_flat")

	_, err = util.ComputeRulesHash("bps", 0, "0",
		[]string{"main"}, []string{"a"}, [][]byte{maaRepeatByte(0x00, 31)})
	require.Error(t, err, "31-byte body_hash")

	_, err = util.ComputeRulesHash("bps", 0, "0",
		[]string{"main"}, []string{"a", "b"}, [][]byte{nil})
	require.Error(t, err, "mismatched parallel-slice lengths")
}

func TestMAADerive_RejectsBadLengths(t *testing.T) {
	good20 := maaRepeatByte(0x11, 20)
	good32 := maaRepeatByte(0x33, 32)

	_, err := util.DeriveRuleID(maaRepeatByte(0x11, 19), good32, nil)
	require.Error(t, err, "19-byte restricted")
	_, err = util.DeriveRuleID(good20, maaRepeatByte(0x33, 31), nil)
	require.Error(t, err, "31-byte rules_hash")

	_, err = util.DeriveMAAAddress(maaRepeatByte(0x22, 19), good20, good32)
	require.Error(t, err, "19-byte unrestricted")
	_, err = util.DeriveMAAAddress(good20, maaRepeatByte(0x11, 21), good32)
	require.Error(t, err, "21-byte restricted")
	_, err = util.DeriveMAAAddress(good20, good20, maaRepeatByte(0x33, 31))
	require.Error(t, err, "31-byte rule_id")
}

func TestMAADeriveMAAAddress_FunderDisambiguates(t *testing.T) {
	r := maaRepeatByte(0x11, 20)
	id := maaRepeatByte(0x33, 32)

	a1, err := util.DeriveMAAAddress(maaRepeatByte(0x22, 20), r, id)
	require.NoError(t, err)
	// Different funder -> different wallet under the same rule.
	a2, err := util.DeriveMAAAddress(maaRepeatByte(0x44, 20), r, id)
	require.NoError(t, err)
	require.False(t, bytes.Equal(a1, a2), "different unrestricted produced the same address")
	// Determinism.
	a1b, err := util.DeriveMAAAddress(maaRepeatByte(0x22, 20), r, id)
	require.NoError(t, err)
	require.True(t, bytes.Equal(a1, a1b), "derivation is not deterministic")
}
