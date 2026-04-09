package contractsapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	adminclient "github.com/trufnetwork/kwil-db/core/rpc/client/admin/jsonrpc"
	rpcjson "github.com/trufnetwork/kwil-db/core/rpc/json"
	"github.com/trufnetwork/sdk-go/core/types"
)

// localRPCServer is a minimal httptest-backed JSON-RPC server that captures
// the method + params of each incoming request and returns a pre-programmed
// result. It's used to assert that LocalActions translates Go inputs into
// the exact wire shape expected by node/extensions/tn_local.
type localRPCServer struct {
	mu            sync.Mutex
	lastMethod    string
	lastParamsRaw json.RawMessage
	// resultByMethod maps JSON-RPC method name → marshalled result body.
	// If absent for a method, the server returns an empty object {}.
	resultByMethod map[string]json.RawMessage
	// errorByMethod maps method → rpc error to return (overrides result).
	errorByMethod map[string]*rpcjson.Error
	server        *httptest.Server
}

func newLocalRPCServer() *localRPCServer {
	s := &localRPCServer{
		resultByMethod: make(map[string]json.RawMessage),
		errorByMethod:  make(map[string]*rpcjson.Error),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc/v1", s.handle)
	s.server = httptest.NewServer(mux)
	return s
}

func (s *localRPCServer) close() { s.server.Close() }

// url returns the base URL (without /rpc/v1) that tnclient / adminclient
// expect — they append /rpc/v1 themselves.
func (s *localRPCServer) baseURL() string { return s.server.URL }

func (s *localRPCServer) setResult(method string, result any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(result)
	if err != nil {
		panic(fmt.Sprintf("setResult: failed to marshal fixture for %q: %v", method, err))
	}
	s.resultByMethod[method] = b
}

func (s *localRPCServer) setError(method string, err *rpcjson.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorByMethod[method] = err
}

func (s *localRPCServer) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	var req rpcjson.Request
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.lastMethod = req.Method
	s.lastParamsRaw = req.Params
	rpcErr := s.errorByMethod[req.Method]
	result := s.resultByMethod[req.Method]
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	if rpcErr != nil {
		resp := rpcjson.NewErrorResponse(req.ID, rpcErr)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Default empty-object result if method not pre-programmed
	if result == nil {
		result = json.RawMessage(`{}`)
	}
	resp := rpcjson.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *localRPCServer) capturedMethod() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastMethod
}

func (s *localRPCServer) capturedParams() json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastParamsRaw
}

// newTestLocalActions wires a LocalActions backed by a live httptest admin
// server. Caller must defer srv.close().
func newTestLocalActions(t *testing.T) (types.ILocalActions, *localRPCServer) {
	t.Helper()
	srv := newLocalRPCServer()
	u, err := url.Parse(srv.baseURL())
	require.NoError(t, err)
	admin := adminclient.NewClient(u)
	local, err := LoadLocalActions(LocalActionsOptions{Admin: admin})
	require.NoError(t, err)
	return local, srv
}

// ═══════════════════════════════════════════════════════════════
// LOADER TESTS
// ═══════════════════════════════════════════════════════════════

func TestLoadLocalActions_NilAdmin(t *testing.T) {
	_, err := LoadLocalActions(LocalActionsOptions{Admin: nil})
	require.Error(t, err)
	require.Contains(t, err.Error(), "admin client is required")
}

// ═══════════════════════════════════════════════════════════════
// CREATE STREAM
// ═══════════════════════════════════════════════════════════════

func TestLocalActions_CreateStream_MethodAndParams(t *testing.T) {
	local, srv := newTestLocalActions(t)
	defer srv.close()

	err := local.CreateStream(context.Background(), types.LocalCreateStreamInput{
		StreamID:   "st00000000000000000000000000test",
		StreamType: types.StreamTypePrimitive,
	})
	require.NoError(t, err)

	require.Equal(t, "local.create_stream", srv.capturedMethod())

	var params map[string]any
	require.NoError(t, json.Unmarshal(srv.capturedParams(), &params))
	// No data_provider must be on the wire — the whole point of Phase A.
	require.NotContains(t, params, "data_provider",
		"CreateStream request must not include data_provider")
	require.Equal(t, "st00000000000000000000000000test", params["stream_id"])
	require.Equal(t, "primitive", params["stream_type"])
}

func TestLocalActions_CreateStream_PropagatesError(t *testing.T) {
	local, srv := newTestLocalActions(t)
	defer srv.close()

	srv.setError("local.create_stream", rpcjson.NewError(
		rpcjson.ErrorInvalidParams, "stream already exists: st...", nil))

	err := local.CreateStream(context.Background(), types.LocalCreateStreamInput{
		StreamID:   "st00000000000000000000000000test",
		StreamType: types.StreamTypePrimitive,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "stream already exists")
}

// ═══════════════════════════════════════════════════════════════
// INSERT RECORDS
// ═══════════════════════════════════════════════════════════════

func TestLocalActions_InsertRecords_MethodAndParams(t *testing.T) {
	local, srv := newTestLocalActions(t)
	defer srv.close()

	err := local.InsertRecords(context.Background(), types.LocalInsertRecordsInput{
		StreamID:  []string{"st00000000000000000000000000test", "st00000000000000000000000000test"},
		EventTime: []int64{1000, 2000},
		Value:     []string{"1.5", "2.5"},
	})
	require.NoError(t, err)

	require.Equal(t, "local.insert_records", srv.capturedMethod())

	var params map[string]any
	require.NoError(t, json.Unmarshal(srv.capturedParams(), &params))
	require.NotContains(t, params, "data_provider",
		"InsertRecords request must not include data_provider")
	require.Contains(t, params, "stream_id")
	require.Contains(t, params, "event_time")
	require.Contains(t, params, "value")
}

// ═══════════════════════════════════════════════════════════════
// INSERT TAXONOMY
// ═══════════════════════════════════════════════════════════════

func TestLocalActions_InsertTaxonomy_MethodAndParams(t *testing.T) {
	local, srv := newTestLocalActions(t)
	defer srv.close()

	err := local.InsertTaxonomy(context.Background(), types.LocalInsertTaxonomyInput{
		StreamID:       "st0000000000000000000000composed",
		ChildStreamIDs: []string{"st000000000000000000000000child1"},
		Weights:        []string{"1.0"},
		StartDate:      100,
	})
	require.NoError(t, err)

	require.Equal(t, "local.insert_taxonomy", srv.capturedMethod())

	var params map[string]any
	require.NoError(t, json.Unmarshal(srv.capturedParams(), &params))
	require.NotContains(t, params, "data_provider",
		"InsertTaxonomy request must not include data_provider")
	require.NotContains(t, params, "child_data_providers",
		"InsertTaxonomy request must not include child_data_providers")
	require.Contains(t, params, "child_stream_ids")
	require.Equal(t, "st0000000000000000000000composed", params["stream_id"])
	require.Equal(t, float64(100), params["start_date"])
}

// ═══════════════════════════════════════════════════════════════
// GET RECORD
// ═══════════════════════════════════════════════════════════════

func TestLocalActions_GetRecord_DecodesRecords(t *testing.T) {
	local, srv := newTestLocalActions(t)
	defer srv.close()

	srv.setResult("local.get_record", map[string]any{
		"records": []map[string]any{
			{"event_time": 1000, "value": "10.5", "created_at": 42},
			{"event_time": 2000, "value": "20.5", "created_at": 43},
		},
	})

	from := int64(500)
	records, err := local.GetRecord(context.Background(), types.LocalGetRecordInput{
		StreamID: "st00000000000000000000000000test",
		FromTime: &from,
	})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, int64(1000), records[0].EventTime)
	require.Equal(t, "10.5", records[0].Value)
	require.Equal(t, int64(42), records[0].CreatedAt)
	require.Equal(t, int64(2000), records[1].EventTime)
	require.Equal(t, "20.5", records[1].Value)

	require.Equal(t, "local.get_record", srv.capturedMethod())

	var params map[string]any
	require.NoError(t, json.Unmarshal(srv.capturedParams(), &params))
	require.NotContains(t, params, "data_provider",
		"GetRecord request must not include data_provider")
	require.Equal(t, float64(500), params["from_time"])
	require.NotContains(t, params, "to_time", "nil to_time must be omitted")
}

func TestLocalActions_GetRecord_EmptyResult(t *testing.T) {
	local, srv := newTestLocalActions(t)
	defer srv.close()

	srv.setResult("local.get_record", map[string]any{"records": []any{}})

	records, err := local.GetRecord(context.Background(), types.LocalGetRecordInput{
		StreamID: "st00000000000000000000000000test",
	})
	require.NoError(t, err)
	require.NotNil(t, records)
	require.Empty(t, records)
}

// ═══════════════════════════════════════════════════════════════
// GET INDEX
// ═══════════════════════════════════════════════════════════════

func TestLocalActions_GetIndex_DecodesRecords(t *testing.T) {
	local, srv := newTestLocalActions(t)
	defer srv.close()

	srv.setResult("local.get_index", map[string]any{
		"records": []map[string]any{
			{"event_time": 1000, "value": "100.000000000000000000"},
			{"event_time": 2000, "value": "200.000000000000000000"},
		},
	})

	baseTime := int64(1000)
	records, err := local.GetIndex(context.Background(), types.LocalGetIndexInput{
		StreamID: "st00000000000000000000000000test",
		BaseTime: &baseTime,
	})
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "100.000000000000000000", records[0].Value)
	require.Equal(t, "200.000000000000000000", records[1].Value)

	require.Equal(t, "local.get_index", srv.capturedMethod())

	var params map[string]any
	require.NoError(t, json.Unmarshal(srv.capturedParams(), &params))
	require.Equal(t, float64(1000), params["base_time"])
}

// ═══════════════════════════════════════════════════════════════
// LIST STREAMS
// ═══════════════════════════════════════════════════════════════

func TestLocalActions_ListStreams_DecodesStreams(t *testing.T) {
	local, srv := newTestLocalActions(t)
	defer srv.close()

	srv.setResult("local.list_streams", map[string]any{
		"streams": []map[string]any{
			{
				"data_provider": "0xabcdef1234567890abcdef1234567890abcdef12",
				"stream_id":     "st00000000000000000000000000test",
				"stream_type":   "primitive",
				"created_at":    42,
			},
		},
	})

	streams, err := local.ListStreams(context.Background())
	require.NoError(t, err)
	require.Len(t, streams, 1)
	// DataProvider is preserved from the response even though it's always
	// the node's own address — mirrors consensus list_streams shape.
	require.Equal(t, "0xabcdef1234567890abcdef1234567890abcdef12", streams[0].DataProvider)
	require.Equal(t, "st00000000000000000000000000test", streams[0].StreamID)
	require.Equal(t, types.StreamType("primitive"), streams[0].StreamType)
	require.Equal(t, int64(42), streams[0].CreatedAt)

	require.Equal(t, "local.list_streams", srv.capturedMethod())
}

func TestLocalActions_ListStreams_Empty(t *testing.T) {
	local, srv := newTestLocalActions(t)
	defer srv.close()

	srv.setResult("local.list_streams", map[string]any{"streams": []any{}})

	streams, err := local.ListStreams(context.Background())
	require.NoError(t, err)
	require.NotNil(t, streams)
	require.Empty(t, streams)
}
