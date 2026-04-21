package contractsapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	adminclient "github.com/trufnetwork/kwil-db/core/rpc/client/admin/jsonrpc"
	"github.com/trufnetwork/sdk-go/core/types"
)

// newSignedTestLocalActions wires LocalActions backed by a live httptest
// server, with a freshly-generated signer. Returns the LocalActions, the
// server, and the operator's lowercased Ethereum address (so tests can
// assert the address that the server *would* recover).
func newSignedTestLocalActions(t *testing.T) (types.ILocalActions, *localRPCServer, string) {
	t.Helper()
	srv := newLocalRPCServer()
	u, err := url.Parse(srv.baseURL())
	require.NoError(t, err)
	admin := adminclient.NewClient(u)

	priv, err := crypto.GenerateKey()
	require.NoError(t, err)
	addr := strings.ToLower(crypto.PubkeyToAddress(priv.PublicKey).Hex())

	local, err := LoadLocalActions(LocalActionsOptions{Admin: admin, Signer: priv})
	require.NoError(t, err)
	return local, srv, addr
}

// ─── canonicalJSON cross-SDK invariants ─────────────────────────────────
//
// These mirror the equivalent tests in node/extensions/tn_local/auth_test.go.
// Both sides must emit byte-identical JSON or signatures won't verify.

func TestCanonicalJSON_DoesNotEscapeHTMLChars(t *testing.T) {
	got, err := canonicalJSON(map[string]any{"value": "a<b>c&d"})
	require.NoError(t, err)
	require.Equal(t, `{"value":"a<b>c&d"}`, string(got),
		"canonical JSON must NOT HTML-escape — Python/JS don't either")
}

func TestCanonicalJSON_PreservesLargeIntegerPrecision(t *testing.T) {
	const big int64 = 9_007_199_254_740_993 // 2^53 + 1, breaks float64
	got, err := canonicalJSON(map[string]any{"event_time": big})
	require.NoError(t, err)
	require.Equal(t, `{"event_time":9007199254740993}`, string(got),
		"large int64 values must not lose precision through canonicalJSON")
}

func TestCanonicalJSON_DeterministicAndSorted(t *testing.T) {
	a, err := canonicalJSON(map[string]any{"b": 2, "a": 1})
	require.NoError(t, err)
	b, err := canonicalJSON(map[string]any{"a": 1, "b": 2})
	require.NoError(t, err)
	require.Equal(t, string(a), string(b))
	require.Equal(t, `{"a":1,"b":2}`, string(a))
}

// ─── Wire-format checks (no live verification) ──────────────────────────

func TestLocalActions_NoSigner_DoesNotAttachAuth(t *testing.T) {
	// Existing path — without a Signer, no _auth field appears on the wire.
	// This is what nodes with require_signature=false will see; existing
	// deployments must continue to work unchanged.
	local, srv := newTestLocalActions(t)
	defer srv.close()

	err := local.CreateStream(context.Background(), types.LocalCreateStreamInput{
		StreamID:   "st00000000000000000000000000test",
		StreamType: types.StreamTypePrimitive,
	})
	require.NoError(t, err)

	var params map[string]any
	require.NoError(t, json.Unmarshal(srv.capturedParams(), &params))
	require.NotContains(t, params, "_auth",
		"_auth must NOT appear when no signer is configured")
}

func TestLocalActions_WithSigner_AttachesAuthWithExpectedShape(t *testing.T) {
	local, srv, _ := newSignedTestLocalActions(t)
	defer srv.close()

	err := local.CreateStream(context.Background(), types.LocalCreateStreamInput{
		StreamID:   "st00000000000000000000000000test",
		StreamType: types.StreamTypePrimitive,
	})
	require.NoError(t, err)

	var params map[string]any
	require.NoError(t, json.Unmarshal(srv.capturedParams(), &params))

	// _auth is present and well-formed.
	authMap, ok := params["_auth"].(map[string]any)
	require.True(t, ok, "_auth must be an object on the wire")
	require.Equal(t, localAuthVersion, authMap["ver"])
	sig, ok := authMap["sig"].(string)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(sig, "0x"))
	require.Len(t, sig, 2+130, "65-byte signature → 130 hex chars + 0x")

	tsAny, ok := authMap["ts"]
	require.True(t, ok)
	tsFloat, ok := tsAny.(float64) // JSON numbers decode as float64
	require.True(t, ok)
	ts := int64(tsFloat)
	require.InDelta(t, time.Now().UnixMilli(), ts, 5000,
		"timestamp should be near now (within 5s of test start)")
}

// ─── Cryptographic correctness (matches server-side verifier) ───────────

// TestLocalActions_SignedRequest_RecoversToOperatorAddress simulates what
// the server-side checkAuth() does: rebuild the digest from the wire
// params+method+ts (with _auth stripped), recover the signing address from
// the signature, and assert it matches the operator. This is the contract
// the server depends on — if it ever drifts the headline wrong-key test
// in the node won't help us, because the SDK side will already be sending
// something the server can't verify.
func TestLocalActions_SignedRequest_RecoversToOperatorAddress(t *testing.T) {
	local, srv, expectedAddr := newSignedTestLocalActions(t)
	defer srv.close()

	err := local.CreateStream(context.Background(), types.LocalCreateStreamInput{
		StreamID:   "st00000000000000000000000000test",
		StreamType: types.StreamTypePrimitive,
	})
	require.NoError(t, err)

	method := srv.capturedMethod()
	require.Equal(t, "local.create_stream", method)

	// Decode wire params, extract _auth, strip it, rebuild canonical bytes.
	var params map[string]any
	require.NoError(t, json.Unmarshal(srv.capturedParams(), &params))
	authMap := params["_auth"].(map[string]any)
	delete(params, "_auth")

	canonical, err := canonicalJSON(params)
	require.NoError(t, err)

	tsMs := int64(authMap["ts"].(float64))
	paramsSha := sha256.Sum256(canonical)
	payload := localAuthVersion + "\n" + method + "\n" + hex.EncodeToString(paramsSha[:]) + "\n" + strconv.FormatInt(tsMs, 10)
	digest := crypto.Keccak256([]byte(payload))

	sigHex := strings.TrimPrefix(authMap["sig"].(string), "0x")
	sig, err := hex.DecodeString(sigHex)
	require.NoError(t, err)
	require.Len(t, sig, 65)
	if sig[64] >= 27 {
		sig[64] -= 27 // normalize V back to {0,1} for Ecrecover
	}

	pub, err := crypto.Ecrecover(digest, sig)
	require.NoError(t, err)
	pk, err := crypto.UnmarshalPubkey(pub)
	require.NoError(t, err)
	recovered := strings.ToLower(crypto.PubkeyToAddress(*pk).Hex())

	require.Equal(t, expectedAddr, recovered,
		"signature must recover to the configured operator key")
}

// TestLocalActions_TimestampMonotonic ensures back-to-back signed calls
// produce different timestamps (and therefore different signatures). This
// matters for the server replay cache — two identical sigs in a row would
// hit it, even though the user issued two separate logical operations.
func TestLocalActions_TimestampMonotonic(t *testing.T) {
	local, srv, _ := newSignedTestLocalActions(t)
	defer srv.close()

	err := local.CreateStream(context.Background(), types.LocalCreateStreamInput{
		StreamID: "st00000000000000000000000000aaaa", StreamType: types.StreamTypePrimitive,
	})
	require.NoError(t, err)
	var first map[string]any
	require.NoError(t, json.Unmarshal(srv.capturedParams(), &first))
	firstSig := first["_auth"].(map[string]any)["sig"]

	// Different params + a fresh timestamp → must produce a fresh signature.
	err = local.CreateStream(context.Background(), types.LocalCreateStreamInput{
		StreamID: "st00000000000000000000000000bbbb", StreamType: types.StreamTypePrimitive,
	})
	require.NoError(t, err)
	var second map[string]any
	require.NoError(t, json.Unmarshal(srv.capturedParams(), &second))
	secondSig := second["_auth"].(map[string]any)["sig"]

	require.NotEqual(t, firstSig, secondSig,
		"distinct calls must produce distinct signatures (else replay cache rejects them)")
}

// ─── attachAuth direct unit tests ───────────────────────────────────────

func TestAttachAuth_NilSigner_NoOp(t *testing.T) {
	l := &LocalActions{} // signer is nil
	req := &localCreateStreamRequest{StreamID: "x", StreamType: "primitive"}
	require.NoError(t, l.attachAuth("local.create_stream", req))
	require.Nil(t, req.Auth, "no signer → no _auth attached")
}

func TestAttachAuth_PopulatesAuth(t *testing.T) {
	priv, err := crypto.GenerateKey()
	require.NoError(t, err)
	l := &LocalActions{signer: priv}

	req := &localCreateStreamRequest{StreamID: "st00000000000000000000000000demo", StreamType: "primitive"}
	require.NoError(t, l.attachAuth("local.create_stream", req))
	require.NotNil(t, req.Auth)
	require.Equal(t, localAuthVersion, req.Auth.Ver)
	require.True(t, strings.HasPrefix(req.Auth.Sig, "0x"))
	require.Greater(t, req.Auth.Ts, int64(0))
}
