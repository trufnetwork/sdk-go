package types

import "context"

// ═══════════════════════════════════════════════════════════════
// LOCAL STREAM ACTIONS (tn_local extension, admin port)
// ═══════════════════════════════════════════════════════════════
//
// LocalActions exposes the six JSON-RPC methods registered by the node's
// tn_local extension on the Kwil admin server (port 8485). Local streams
// are stored off-chain on a single node — they do not participate in
// consensus and incur no transaction fees.
//
// Ownership model: every local stream is implicitly owned by the node
// operator. The operator's Ethereum address (derived from the node's
// secp256k1 key server-side) is the data_provider for every operation —
// clients never supply a data_provider on the wire. This prevents any
// caller with admin-port access from impersonating other DPs.
//
// Response mirroring: response types keep data_provider fields where the
// mirrored consensus action returns one (e.g. list_streams). The value is
// always equal to the node's own address — redundant but preserved so that
// callers can swap local/on-chain code without reshaping records.
//
// Transport: LocalActions talks to the kwil-db admin JSON-RPC server
// directly, not through the gateway. Auth is whatever the admin server
// requires (unix socket, mTLS, basic password) — see sdk-go's
// tnclient.WithAdmin() for configuration.

// ILocalActions is the client-side interface for tn_local's JSON-RPC methods.
type ILocalActions interface {
	// CreateStream creates a local stream owned by the node operator.
	// Maps to: local.create_stream (tn_local handlers.go CreateStream)
	CreateStream(ctx context.Context, input LocalCreateStreamInput) error

	// InsertRecords appends records to one or more local primitive streams.
	// All streams must be owned by this node; the server enforces ownership.
	// Maps to: local.insert_records (tn_local handlers.go InsertRecords)
	InsertRecords(ctx context.Context, input LocalInsertRecordsInput) error

	// InsertTaxonomy adds a taxonomy group to a local composed stream.
	// Both parent and children must be local streams owned by this node.
	// Maps to: local.insert_taxonomy (tn_local handlers.go InsertTaxonomy)
	InsertTaxonomy(ctx context.Context, input LocalInsertTaxonomyInput) error

	// GetRecord queries records from a local stream (primitive or composed).
	// Both FromTime and ToTime nil returns the latest record.
	// Maps to: local.get_record (tn_local handlers.go GetRecord)
	GetRecord(ctx context.Context, input LocalGetRecordInput) ([]LocalRecordOutput, error)

	// GetIndex queries computed index values from a local stream.
	// Index = (value / base_value) * 100.
	// Maps to: local.get_index (tn_local handlers.go GetIndex)
	GetIndex(ctx context.Context, input LocalGetIndexInput) ([]LocalIndexOutput, error)

	// DeleteStream removes a local stream and all associated data.
	// Maps to: local.delete_stream (tn_local handlers.go DeleteStream)
	DeleteStream(ctx context.Context, input LocalDeleteStreamInput) error

	// DisableTaxonomy disables a taxonomy group on a local composed stream.
	// Maps to: local.disable_taxonomy (tn_local handlers.go DisableTaxonomy)
	DisableTaxonomy(ctx context.Context, input LocalDisableTaxonomyInput) error

	// ListStreams returns all local streams owned by this node.
	// Maps to: local.list_streams (tn_local handlers.go ListStreams)
	ListStreams(ctx context.Context) ([]LocalStreamInfo, error)
}

// ═══════════════════════════════════════════════════════════════
// INPUT TYPES (no data_provider field — server-derived from node key)
// ═══════════════════════════════════════════════════════════════

// LocalCreateStreamInput is the input for local.create_stream.
type LocalCreateStreamInput struct {
	StreamID   string     `json:"stream_id"`
	StreamType StreamType `json:"stream_type"` // primitive or composed
}

// LocalInsertRecordsInput is the input for local.insert_records.
// Parallel arrays — each index (stream_id[i], event_time[i], value[i])
// describes one record. Different records may target different streams,
// but all streams are owned by this node.
type LocalInsertRecordsInput struct {
	StreamID  []string `json:"stream_id"`
	EventTime []int64  `json:"event_time"`
	Value     []string `json:"value"` // decimal string, NUMERIC(36,18)
}

// LocalInsertTaxonomyInput is the input for local.insert_taxonomy.
// Parallel arrays for (child_stream_ids[i], weights[i]). Children are
// always local to the same node — no cross-DP composition.
type LocalInsertTaxonomyInput struct {
	StreamID       string   `json:"stream_id"`
	ChildStreamIDs []string `json:"child_stream_ids"`
	Weights        []string `json:"weights"` // decimal string, NUMERIC(36,18)
	StartDate      int64    `json:"start_date"`
}

// LocalGetRecordInput is the input for local.get_record.
// FromTime and ToTime are optional pointers; nil leaves the bound open.
// Passing nil for both returns the latest record.
type LocalGetRecordInput struct {
	StreamID string `json:"stream_id"`
	FromTime *int64 `json:"from_time,omitempty"`
	ToTime   *int64 `json:"to_time,omitempty"`
}

// LocalGetIndexInput is the input for local.get_index.
// Defaults: BaseTime nil = earliest event_time in the stream.
type LocalGetIndexInput struct {
	StreamID string `json:"stream_id"`
	FromTime *int64 `json:"from_time,omitempty"`
	ToTime   *int64 `json:"to_time,omitempty"`
	BaseTime *int64 `json:"base_time,omitempty"`
}

// LocalDeleteStreamInput is the input for local.delete_stream.
type LocalDeleteStreamInput struct {
	StreamID string `json:"stream_id"`
}

// LocalDisableTaxonomyInput is the input for local.disable_taxonomy.
type LocalDisableTaxonomyInput struct {
	StreamID      string `json:"stream_id"`
	GroupSequence int    `json:"group_sequence"`
}

// ═══════════════════════════════════════════════════════════════
// OUTPUT TYPES (mirror the consensus action shapes exactly)
// ═══════════════════════════════════════════════════════════════

// LocalRecordOutput is a single row from local.get_record. Matches the
// consensus get_record return table; CreatedAt is the block height at
// which the record was inserted (LOCF versioning key).
type LocalRecordOutput struct {
	EventTime int64  `json:"event_time"`
	Value     string `json:"value"`
	CreatedAt int64  `json:"created_at"`
}

// LocalIndexOutput is a single row from local.get_index.
type LocalIndexOutput struct {
	EventTime int64  `json:"event_time"`
	Value     string `json:"value"`
}

// LocalStreamInfo is a single row from local.list_streams.
// DataProvider is always equal to the node's own address — kept for parity
// with the consensus list_streams action so that on-chain and local
// callers use the same response shape.
type LocalStreamInfo struct {
	DataProvider string     `json:"data_provider"`
	StreamID     string     `json:"stream_id"`
	StreamType   StreamType `json:"stream_type"`
	CreatedAt    int64      `json:"created_at"`
}
