package contractsapi

import (
	"context"

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
type LocalActions struct {
	admin *adminclient.Client
}

// Compile-time interface check
var _ types.ILocalActions = (*LocalActions)(nil)

// LocalActionsOptions contains options for creating a LocalActions instance.
type LocalActionsOptions struct {
	// Admin is a kwil-db admin JSON-RPC client configured with the admin URL
	// and appropriate auth (unix socket / mTLS / basic password). It is not
	// owned by LocalActions — the caller is responsible for its lifecycle.
	Admin *adminclient.Client
}

// LoadLocalActions creates a new LocalActions instance.
func LoadLocalActions(opts LocalActionsOptions) (types.ILocalActions, error) {
	if opts.Admin == nil {
		return nil, errors.New("admin client is required; construct tnclient with WithAdmin()")
	}
	return &LocalActions{admin: opts.Admin}, nil
}

// ═══════════════════════════════════════════════════════════════
// WIRE TYPES (private, mirror node/extensions/tn_local/types.go)
// ═══════════════════════════════════════════════════════════════
//
// These are defined here instead of imported from the node repo because
// sdk-go cannot take a dependency on node (node imports sdk-go). They are
// deliberately tiny and change together with the server types — any
// schema drift will fail the round-trip tests.

type localCreateStreamRequest struct {
	StreamID   string `json:"stream_id"`
	StreamType string `json:"stream_type"`
}
type localCreateStreamResponse struct{}

type localInsertRecordsRequest struct {
	StreamID  []string `json:"stream_id"`
	EventTime []int64  `json:"event_time"`
	Value     []string `json:"value"`
}
type localInsertRecordsResponse struct{}

type localInsertTaxonomyRequest struct {
	StreamID       string   `json:"stream_id"`
	ChildStreamIDs []string `json:"child_stream_ids"`
	Weights        []string `json:"weights"`
	StartDate      int64    `json:"start_date"`
}
type localInsertTaxonomyResponse struct{}

type localGetRecordRequest struct {
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

type localListStreamsRequest struct{}

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
	res := &localCreateStreamResponse{}
	if err := l.admin.CallMethod(ctx, "local.create_stream", req, res); err != nil {
		return errors.Wrap(err, "local.create_stream")
	}
	return nil
}

// InsertRecords → local.insert_records
func (l *LocalActions) InsertRecords(ctx context.Context, input types.LocalInsertRecordsInput) error {
	req := localInsertRecordsRequest{
		StreamID:  input.StreamID,
		EventTime: input.EventTime,
		Value:     input.Value,
	}
	res := &localInsertRecordsResponse{}
	if err := l.admin.CallMethod(ctx, "local.insert_records", req, res); err != nil {
		return errors.Wrap(err, "local.insert_records")
	}
	return nil
}

// InsertTaxonomy → local.insert_taxonomy
func (l *LocalActions) InsertTaxonomy(ctx context.Context, input types.LocalInsertTaxonomyInput) error {
	req := localInsertTaxonomyRequest{
		StreamID:       input.StreamID,
		ChildStreamIDs: input.ChildStreamIDs,
		Weights:        input.Weights,
		StartDate:      input.StartDate,
	}
	res := &localInsertTaxonomyResponse{}
	if err := l.admin.CallMethod(ctx, "local.insert_taxonomy", req, res); err != nil {
		return errors.Wrap(err, "local.insert_taxonomy")
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
	res := &localGetRecordResponse{}
	if err := l.admin.CallMethod(ctx, "local.get_record", req, res); err != nil {
		return nil, errors.Wrap(err, "local.get_record")
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
	res := &localGetIndexResponse{}
	if err := l.admin.CallMethod(ctx, "local.get_index", req, res); err != nil {
		return nil, errors.Wrap(err, "local.get_index")
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

// ListStreams → local.list_streams
func (l *LocalActions) ListStreams(ctx context.Context) ([]types.LocalStreamInfo, error) {
	req := localListStreamsRequest{}
	res := &localListStreamsResponse{}
	if err := l.admin.CallMethod(ctx, "local.list_streams", req, res); err != nil {
		return nil, errors.Wrap(err, "local.list_streams")
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
