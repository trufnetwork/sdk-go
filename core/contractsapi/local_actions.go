package contractsapi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	adminclient "github.com/trufnetwork/kwil-db/core/rpc/client/admin/jsonrpc"
	"github.com/trufnetwork/sdk-go/core/types"
)

// LocalActions implements types.ILocalActions by calling local.* methods on
// a Kwil admin JSON-RPC server. It uses kwil-db's generic admin client as a
// pure transport — kwil-db knows nothing about TN-specific vocabulary, so
// every method is a thin wrapper around adminclient.CallMethod.
//
// Construction: do not instantiate directly. Use tnclient.Client.LoadLocalActions(),
// which constructs an admin client from the WithAdmin() option and hands it here.
//
// Optional signer: if the server has require_signature=true, every request
// must carry a `_auth` field signed by the node operator's secp256k1 key.
// Set Signer in LocalActionsOptions (or use tnclient.WithLocalSigner) and
// LocalActions will attach `_auth` to every call transparently.
type LocalActions struct {
	admin  *adminclient.Client
	signer *ecdsa.PrivateKey // nil = no _auth attached (server flag must be off)
}

// Compile-time interface check
var _ types.ILocalActions = (*LocalActions)(nil)

// LocalActionsOptions contains options for creating a LocalActions instance.
type LocalActionsOptions struct {
	// Admin is a kwil-db admin JSON-RPC client configured with the admin URL
	// and appropriate auth (unix socket / mTLS / basic password). It is not
	// owned by LocalActions — the caller is responsible for its lifecycle.
	Admin *adminclient.Client

	// Signer is the operator's secp256k1 private key. Optional. When set,
	// LocalActions attaches an `_auth` envelope (sig, ts, ver) to every
	// request — required for nodes with require_signature=true. When unset,
	// no `_auth` is attached and only nodes with the flag off will accept
	// the request.
	Signer *ecdsa.PrivateKey
}

// LoadLocalActions creates a new LocalActions instance.
func LoadLocalActions(opts LocalActionsOptions) (types.ILocalActions, error) {
	if opts.Admin == nil {
		return nil, errors.New("admin client is required; use tnclient.WithAdmin() or tnclient.NewLocalClient()")
	}
	return &LocalActions{admin: opts.Admin, signer: opts.Signer}, nil
}

// ─── Auth envelope and signing ──────────────────────────────────────────
//
// Wire format of the per-request auth header. Mirrors the server-side
// AuthHeader in node/extensions/tn_local/auth.go. The `_auth` field rides
// along with the request via struct embedding (see localAuthEnvelope) so
// SDKs that don't sign just leave it absent.

const localAuthVersion = "tn_local.auth.v1"

// JSON-RPC method names. Mirror the server-side constants in
// node/extensions/tn_local/constants.go — these are part of the canonical
// payload (the digest binds the signature to the method), so renaming
// either side without coordinating breaks every signed call.
const (
	methodCreateStream    = "local.create_stream"
	methodInsertRecords   = "local.insert_records"
	methodInsertTaxonomy  = "local.insert_taxonomy"
	methodDeleteStream    = "local.delete_stream"
	methodDisableTaxonomy = "local.disable_taxonomy"
	methodGetRecord       = "local.get_record"
	methodGetIndex        = "local.get_index"
	methodListStreams     = "local.list_streams"
)

type localAuthHeader struct {
	Sig string `json:"sig"`
	Ts  int64  `json:"ts"`
	Ver string `json:"ver"`
}

type localAuthEnvelope struct {
	Auth *localAuthHeader `json:"_auth,omitempty"`
}

func (e *localAuthEnvelope) getAuth() *localAuthHeader  { return e.Auth }
func (e *localAuthEnvelope) setAuth(a *localAuthHeader) { e.Auth = a }

// localAuthSetter is satisfied by every request type that embeds
// localAuthEnvelope.
type localAuthSetter interface {
	getAuth() *localAuthHeader
	setAuth(*localAuthHeader)
}

// canonicalJSON re-encodes v as JSON with sorted keys, no whitespace, no
// HTML escaping, and full numeric precision. Mirrors the verifier in
// node/extensions/tn_local/auth.go — both sides MUST produce byte-identical
// output for a signature signed here to verify there.
func canonicalJSON(v any) ([]byte, error) {
	first, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonical marshal: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(first))
	dec.UseNumber() // preserve int64 precision past 2^53
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("canonical reparse: %w", err)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // verbatim < > &, matches Python/JS defaults
	if err := enc.Encode(generic); err != nil {
		return nil, fmt.Errorf("canonical re-encode: %w", err)
	}
	out := buf.Bytes()
	if n := len(out); n > 0 && out[n-1] == '\n' {
		out = out[:n-1] // strip trailing newline json.Encoder appends
	}
	return out, nil
}

// attachAuth signs the request (with Auth field nil) and sets the resulting
// auth header. No-op when l.signer is nil — the server then rejects (if its
// flag is on) or accepts (if off). Either is correct depending on deployment.
func (l *LocalActions) attachAuth(method string, req localAuthSetter) error {
	if l.signer == nil {
		return nil
	}
	req.setAuth(nil) // ensure clean baseline before signing
	paramsBytes, err := canonicalJSON(req)
	if err != nil {
		return fmt.Errorf("auth canonicalize: %w", err)
	}
	tsMs := time.Now().UnixMilli()
	paramsSha := sha256.Sum256(paramsBytes)
	// Layout matches node/extensions/tn_local/auth.go canonicalDigest:
	//   prefix + "\n" + method + "\n" + sha256_hex(params) + "\n" + ts_ms
	payload := localAuthVersion + "\n" + method + "\n" + hex.EncodeToString(paramsSha[:]) + "\n" + strconv.FormatInt(tsMs, 10)
	digest := crypto.Keccak256([]byte(payload))
	sig, err := crypto.Sign(digest, l.signer)
	if err != nil {
		return fmt.Errorf("auth sign: %w", err)
	}
	if sig[64] < 27 {
		sig[64] += 27 // normalize V to {27,28} for EVM compatibility
	}
	req.setAuth(&localAuthHeader{
		Sig: "0x" + hex.EncodeToString(sig),
		Ts:  tsMs,
		Ver: localAuthVersion,
	})
	return nil
}

// ═══════════════════════════════════════════════════════════════
// WIRE TYPES (private, mirror node/extensions/tn_local/types.go)
// ═══════════════════════════════════════════════════════════════
//
// These are defined here instead of imported from the node repo because
// sdk-go cannot take a dependency on node (node imports sdk-go). They are
// deliberately tiny and change together with the server types — any
// schema drift will fail the round-trip tests.

// All request types embed localAuthEnvelope so an optional `_auth` field
// is always available to attachAuth. Embedding (rather than copying the
// field on each type) gives us a single AuthSetter implementation.

type localCreateStreamRequest struct {
	localAuthEnvelope
	StreamID   string `json:"stream_id"`
	StreamType string `json:"stream_type"`
}
type localCreateStreamResponse struct{}

type localInsertRecordsRequest struct {
	localAuthEnvelope
	StreamID  []string `json:"stream_id"`
	EventTime []int64  `json:"event_time"`
	Value     []string `json:"value"`
}
type localInsertRecordsResponse struct{}

type localInsertTaxonomyRequest struct {
	localAuthEnvelope
	StreamID       string   `json:"stream_id"`
	ChildStreamIDs []string `json:"child_stream_ids"`
	Weights        []string `json:"weights"`
	StartDate      int64    `json:"start_date"`
}
type localInsertTaxonomyResponse struct{}

type localGetRecordRequest struct {
	localAuthEnvelope
	StreamID string `json:"stream_id"`
	FromTime *int64 `json:"from_time,omitempty"`
	ToTime   *int64 `json:"to_time,omitempty"`
}

type localRecordOutputWire struct {
	EventTime int64  `json:"event_time"`
	Value     string `json:"value"`
	CreatedAt int64  `json:"created_at"`
}
type localGetRecordResponse struct {
	Records []localRecordOutputWire `json:"records"`
}

type localGetIndexRequest struct {
	localAuthEnvelope
	StreamID string `json:"stream_id"`
	FromTime *int64 `json:"from_time,omitempty"`
	ToTime   *int64 `json:"to_time,omitempty"`
	BaseTime *int64 `json:"base_time,omitempty"`
}

type localIndexOutputWire struct {
	EventTime int64  `json:"event_time"`
	Value     string `json:"value"`
}
type localGetIndexResponse struct {
	Records []localIndexOutputWire `json:"records"`
}

type localDeleteStreamRequest struct {
	localAuthEnvelope
	StreamID string `json:"stream_id"`
}
type localDeleteStreamResponse struct{}

type localDisableTaxonomyRequest struct {
	localAuthEnvelope
	StreamID      string `json:"stream_id"`
	GroupSequence int    `json:"group_sequence"`
}
type localDisableTaxonomyResponse struct{}

type localListStreamsRequest struct {
	localAuthEnvelope
}

type localStreamInfoWire struct {
	DataProvider string `json:"data_provider"`
	StreamID     string `json:"stream_id"`
	StreamType   string `json:"stream_type"`
	CreatedAt    int64  `json:"created_at"`
}
type localListStreamsResponse struct {
	Streams []localStreamInfoWire `json:"streams"`
}

// ═══════════════════════════════════════════════════════════════
// METHOD IMPLEMENTATIONS
// ═══════════════════════════════════════════════════════════════

// CreateStream → local.create_stream
func (l *LocalActions) CreateStream(ctx context.Context, input types.LocalCreateStreamInput) error {
	req := localCreateStreamRequest{
		StreamID:   input.StreamID,
		StreamType: string(input.StreamType),
	}
	if err := l.attachAuth(methodCreateStream, &req); err != nil {
		return errors.Wrap(err, methodCreateStream)
	}
	res := &localCreateStreamResponse{}
	if err := l.admin.CallMethod(ctx, methodCreateStream, req, res); err != nil {
		return errors.Wrap(err, methodCreateStream)
	}
	return nil
}

// InsertRecords → local.insert_records
func (l *LocalActions) InsertRecords(ctx context.Context, input types.LocalInsertRecordsInput) error {
	if len(input.StreamID) != len(input.EventTime) || len(input.StreamID) != len(input.Value) {
		return fmt.Errorf("local.insert_records: array lengths mismatch (stream_id=%d, event_time=%d, value=%d)",
			len(input.StreamID), len(input.EventTime), len(input.Value))
	}
	req := localInsertRecordsRequest{
		StreamID:  input.StreamID,
		EventTime: input.EventTime,
		Value:     input.Value,
	}
	if err := l.attachAuth(methodInsertRecords, &req); err != nil {
		return errors.Wrap(err, methodInsertRecords)
	}
	res := &localInsertRecordsResponse{}
	if err := l.admin.CallMethod(ctx, methodInsertRecords, req, res); err != nil {
		return errors.Wrap(err, methodInsertRecords)
	}
	return nil
}

// InsertTaxonomy → local.insert_taxonomy
func (l *LocalActions) InsertTaxonomy(ctx context.Context, input types.LocalInsertTaxonomyInput) error {
	if len(input.ChildStreamIDs) != len(input.Weights) {
		return fmt.Errorf("local.insert_taxonomy: array lengths mismatch (child_stream_ids=%d, weights=%d)",
			len(input.ChildStreamIDs), len(input.Weights))
	}
	req := localInsertTaxonomyRequest{
		StreamID:       input.StreamID,
		ChildStreamIDs: input.ChildStreamIDs,
		Weights:        input.Weights,
		StartDate:      input.StartDate,
	}
	if err := l.attachAuth(methodInsertTaxonomy, &req); err != nil {
		return errors.Wrap(err, methodInsertTaxonomy)
	}
	res := &localInsertTaxonomyResponse{}
	if err := l.admin.CallMethod(ctx, methodInsertTaxonomy, req, res); err != nil {
		return errors.Wrap(err, methodInsertTaxonomy)
	}
	return nil
}

// GetRecord → local.get_record
func (l *LocalActions) GetRecord(ctx context.Context, input types.LocalGetRecordInput) ([]types.LocalRecordOutput, error) {
	req := localGetRecordRequest{
		StreamID: input.StreamID,
		FromTime: input.FromTime,
		ToTime:   input.ToTime,
	}
	if err := l.attachAuth(methodGetRecord, &req); err != nil {
		return nil, errors.Wrap(err, methodGetRecord)
	}
	res := &localGetRecordResponse{}
	if err := l.admin.CallMethod(ctx, methodGetRecord, req, res); err != nil {
		return nil, errors.Wrap(err, methodGetRecord)
	}
	records := make([]types.LocalRecordOutput, 0, len(res.Records))
	for _, r := range res.Records {
		records = append(records, types.LocalRecordOutput{
			EventTime: r.EventTime,
			Value:     r.Value,
			CreatedAt: r.CreatedAt,
		})
	}
	return records, nil
}

// GetIndex → local.get_index
func (l *LocalActions) GetIndex(ctx context.Context, input types.LocalGetIndexInput) ([]types.LocalIndexOutput, error) {
	req := localGetIndexRequest{
		StreamID: input.StreamID,
		FromTime: input.FromTime,
		ToTime:   input.ToTime,
		BaseTime: input.BaseTime,
	}
	if err := l.attachAuth(methodGetIndex, &req); err != nil {
		return nil, errors.Wrap(err, methodGetIndex)
	}
	res := &localGetIndexResponse{}
	if err := l.admin.CallMethod(ctx, methodGetIndex, req, res); err != nil {
		return nil, errors.Wrap(err, methodGetIndex)
	}
	records := make([]types.LocalIndexOutput, 0, len(res.Records))
	for _, r := range res.Records {
		records = append(records, types.LocalIndexOutput{
			EventTime: r.EventTime,
			Value:     r.Value,
		})
	}
	return records, nil
}

// DeleteStream → local.delete_stream
func (l *LocalActions) DeleteStream(ctx context.Context, input types.LocalDeleteStreamInput) error {
	req := localDeleteStreamRequest{
		StreamID: input.StreamID,
	}
	if err := l.attachAuth(methodDeleteStream, &req); err != nil {
		return errors.Wrap(err, methodDeleteStream)
	}
	res := &localDeleteStreamResponse{}
	if err := l.admin.CallMethod(ctx, methodDeleteStream, req, res); err != nil {
		return errors.Wrap(err, methodDeleteStream)
	}
	return nil
}

// DisableTaxonomy → local.disable_taxonomy
func (l *LocalActions) DisableTaxonomy(ctx context.Context, input types.LocalDisableTaxonomyInput) error {
	req := localDisableTaxonomyRequest{
		StreamID:      input.StreamID,
		GroupSequence: input.GroupSequence,
	}
	if err := l.attachAuth(methodDisableTaxonomy, &req); err != nil {
		return errors.Wrap(err, methodDisableTaxonomy)
	}
	res := &localDisableTaxonomyResponse{}
	if err := l.admin.CallMethod(ctx, methodDisableTaxonomy, req, res); err != nil {
		return errors.Wrap(err, methodDisableTaxonomy)
	}
	return nil
}

// ListStreams → local.list_streams
func (l *LocalActions) ListStreams(ctx context.Context) ([]types.LocalStreamInfo, error) {
	req := localListStreamsRequest{}
	if err := l.attachAuth(methodListStreams, &req); err != nil {
		return nil, errors.Wrap(err, methodListStreams)
	}
	res := &localListStreamsResponse{}
	if err := l.admin.CallMethod(ctx, methodListStreams, req, res); err != nil {
		return nil, errors.Wrap(err, methodListStreams)
	}
	streams := make([]types.LocalStreamInfo, 0, len(res.Streams))
	for _, s := range res.Streams {
		streams = append(streams, types.LocalStreamInfo{
			DataProvider: s.DataProvider,
			StreamID:     s.StreamID,
			StreamType:   types.StreamType(s.StreamType),
			CreatedAt:    s.CreatedAt,
		})
	}
	return streams, nil
}
