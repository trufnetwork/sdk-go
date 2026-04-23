package contractsapi_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwilclient "github.com/trufnetwork/kwil-db/core/client/types"
	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/contractsapi"
	sdktypes "github.com/trufnetwork/sdk-go/core/types"
)

// --- mocks ---

type mockBroadcaster struct {
	mu          sync.Mutex
	calls       []broadcastCall
	failNext    int       // when > 0, fail the next N calls with failErr (then decrement)
	failErr     error
	hashCounter uint64
}

type broadcastCall struct {
	chunkSize int
	nonce     int64
	syncBcast bool
}

func (m *mockBroadcaster) InsertRecords(_ context.Context, inputs []sdktypes.InsertRecordInput, opts ...kwilclient.TxOpt) (kwiltypes.Hash, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	txOpts := kwilclient.GetTxOpts(opts)
	m.calls = append(m.calls, broadcastCall{
		chunkSize: len(inputs),
		nonce:     txOpts.Nonce,
		syncBcast: txOpts.SyncBcast,
	})

	if m.failNext > 0 {
		m.failNext--
		return kwiltypes.Hash{}, m.failErr
	}

	atomic.AddUint64(&m.hashCounter, 1)
	var h kwiltypes.Hash
	h[0] = byte(m.hashCounter)
	return h, nil
}

func (m *mockBroadcaster) snapshot() []broadcastCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]broadcastCall, len(m.calls))
	copy(out, m.calls)
	return out
}

type mockTxClient struct {
	mu              sync.Mutex
	getAccountCalls int
	waitTxCalls     int
	ledgerNonce     int64 // returned by GetAccount
	getAccountErr   error
	waitTxErr       error
}

func (m *mockTxClient) GetAccount(_ context.Context, _ *kwiltypes.AccountID, _ kwiltypes.AccountStatus) (*kwiltypes.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getAccountCalls++
	if m.getAccountErr != nil {
		return nil, m.getAccountErr
	}
	return &kwiltypes.Account{Nonce: m.ledgerNonce}, nil
}

func (m *mockTxClient) WaitTx(_ context.Context, _ kwiltypes.Hash, _ time.Duration) (*kwiltypes.TxQueryResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.waitTxCalls++
	if m.waitTxErr != nil {
		return nil, m.waitTxErr
	}
	return &kwiltypes.TxQueryResponse{}, nil
}

// --- helpers ---

func newTestSigner(t *testing.T) auth.Signer {
	t.Helper()
	priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	require.NoError(t, err)
	key, ok := priv.(*crypto.Secp256k1PrivateKey)
	require.True(t, ok)
	return &auth.EthPersonalSigner{Key: *key}
}

func makeInputs(n int) []sdktypes.InsertRecordInput {
	out := make([]sdktypes.InsertRecordInput, n)
	for i := range out {
		out[i] = sdktypes.InsertRecordInput{
			DataProvider: "0x0000000000000000000000000000000000000000",
			StreamId:     "stteststream0000000000000000000",
			EventTime:    1700000000 + i,
			Value:        float64(i),
		}
	}
	return out
}

// --- tests ---

func TestBulkInserter_NewBulkInserter_ValidatesArgs(t *testing.T) {
	signer := newTestSigner(t)
	bc := &mockBroadcaster{}
	tc := &mockTxClient{}

	_, err := contractsapi.NewBulkInserter(nil, tc, signer)
	assert.Error(t, err, "nil broadcaster should error")

	_, err = contractsapi.NewBulkInserter(bc, nil, signer)
	assert.Error(t, err, "nil tx client should error")

	_, err = contractsapi.NewBulkInserter(bc, tc, nil)
	assert.Error(t, err, "nil signer should error")

	bi, err := contractsapi.NewBulkInserter(bc, tc, signer)
	require.NoError(t, err)
	require.NotNil(t, bi)
}

func TestBulkInserter_EmptyInput_NoBroadcasts(t *testing.T) {
	bc := &mockBroadcaster{}
	tc := &mockTxClient{}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t))
	require.NoError(t, err)

	hashes, err := bi.InsertAll(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, hashes)
	assert.Empty(t, bc.snapshot(), "no broadcasts on empty input")
	assert.Equal(t, 0, tc.getAccountCalls, "no nonce fetch on empty input")
}

func TestBulkInserter_NonceMonotonicity(t *testing.T) {
	bc := &mockBroadcaster{}
	tc := &mockTxClient{ledgerNonce: 100}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithBatchSize(10),
		contractsapi.WithMaxInflight(1000),
	)
	require.NoError(t, err)

	hashes, err := bi.InsertAll(context.Background(), makeInputs(50))
	require.NoError(t, err)
	assert.Len(t, hashes, 5, "50 inputs / 10 per batch = 5 chunks")

	calls := bc.snapshot()
	require.Len(t, calls, 5)
	for i, c := range calls {
		assert.Equalf(t, int64(101+i), c.nonce, "call %d should use nonce %d (ledger=100, +1+i)", i, 101+i)
		assert.Equal(t, 10, c.chunkSize)
		assert.False(t, c.syncBcast, "broadcasts should be fire-and-forget")
	}
	assert.Equal(t, 1, tc.getAccountCalls, "nonce should be fetched only once")
}

func TestBulkInserter_DrainBatchesEveryMaxInflight(t *testing.T) {
	bc := &mockBroadcaster{}
	tc := &mockTxClient{ledgerNonce: 0}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithBatchSize(10),
		contractsapi.WithMaxInflight(3),
	)
	require.NoError(t, err)

	hashes, err := bi.InsertAll(context.Background(), makeInputs(70)) // 7 chunks
	require.NoError(t, err)
	assert.Len(t, hashes, 7)

	// 7 hashes: drain at 3, drain at 6, final drain at 7. Total WaitTx = 7.
	assert.Equal(t, 7, tc.waitTxCalls, "every hash should be awaited exactly once")
}

func TestBulkInserter_InvalidNonce_ResetsAndRetries(t *testing.T) {
	bc := &mockBroadcaster{
		failNext: 1,
		failErr:  fmt.Errorf("wrapped: %w", kwiltypes.ErrInvalidNonce),
	}
	tc := &mockTxClient{ledgerNonce: 50}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithMaxAttempts(3),
		contractsapi.WithRetryBackoff(1*time.Millisecond),
	)
	require.NoError(t, err)

	hashes, err := bi.InsertAll(context.Background(), makeInputs(10))
	require.NoError(t, err)
	assert.Len(t, hashes, 1)

	// First call failed → should reset and refetch nonce → then succeed
	assert.Equal(t, 2, tc.getAccountCalls, "nonce should be refetched after invalid nonce")
	assert.Len(t, bc.snapshot(), 2, "should retry once")
}

func TestBulkInserter_MempoolFull_BackoffWithoutReset(t *testing.T) {
	bc := &mockBroadcaster{
		failNext: 1,
		failErr:  fmt.Errorf("wrapped: %w", kwiltypes.ErrMempoolFull),
	}
	tc := &mockTxClient{ledgerNonce: 50}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithMaxAttempts(3),
		contractsapi.WithRetryBackoff(1*time.Millisecond),
	)
	require.NoError(t, err)

	hashes, err := bi.InsertAll(context.Background(), makeInputs(10))
	require.NoError(t, err)
	assert.Len(t, hashes, 1)

	// Mempool full does NOT trigger reset — only one ledger fetch
	assert.Equal(t, 1, tc.getAccountCalls, "nonce should NOT be refetched on mempool-full")
	assert.Len(t, bc.snapshot(), 2, "should retry once")

	// Both attempts should reuse the same nonce (51 = ledger+1)
	calls := bc.snapshot()
	assert.Equal(t, int64(51), calls[0].nonce)
	assert.Equal(t, int64(51), calls[1].nonce, "retry should reuse nonce after mempool-full")
}

func TestBulkInserter_CatchingUp_BackoffWithoutReset(t *testing.T) {
	// Mirrors the wire-level shape: kwild rejects with a BroadcastError whose
	// message contains "node is catching up, cannot process transactions
	// right now" (see kwil-db/node/node.go). Since there's no exported
	// sentinel in kwil-db today, BulkInserter detects this by substring on
	// the message.
	bc := &mockBroadcaster{
		failNext: 1,
		failErr:  errors.New("broadcast error: code 65535: node is catching up, cannot process transactions right now"),
	}
	tc := &mockTxClient{ledgerNonce: 50}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithMaxAttempts(3),
		contractsapi.WithCatchupBackoff(1*time.Millisecond),
	)
	require.NoError(t, err)

	hashes, err := bi.InsertAll(context.Background(), makeInputs(10))
	require.NoError(t, err)
	assert.Len(t, hashes, 1)

	// Catching-up does NOT trigger nonce reset — only one ledger fetch
	assert.Equal(t, 1, tc.getAccountCalls, "nonce should NOT be refetched on catching-up")
	assert.Len(t, bc.snapshot(), 2, "should retry once")

	// Both attempts should reuse the same nonce (51 = ledger+1)
	calls := bc.snapshot()
	assert.Equal(t, int64(51), calls[0].nonce)
	assert.Equal(t, int64(51), calls[1].nonce, "retry should reuse nonce after catching-up")
}

func TestBulkInserter_CatchingUp_ContextCancellation(t *testing.T) {
	bc := &mockBroadcaster{
		failNext: 100,
		failErr:  errors.New("broadcast error: node is catching up, cannot process transactions right now"),
	}
	tc := &mockTxClient{ledgerNonce: 0}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithMaxAttempts(10),
		contractsapi.WithCatchupBackoff(100*time.Millisecond),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = bi.InsertAll(ctx, makeInputs(10))
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "context cancellation should propagate during catchup backoff")
}

func TestBulkInserter_PersistentFailure_ReturnsBulkInsertError(t *testing.T) {
	bc := &mockBroadcaster{
		failNext: 100, // always fail
		failErr:  fmt.Errorf("wrapped: %w", kwiltypes.ErrInvalidNonce),
	}
	tc := &mockTxClient{ledgerNonce: 50}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithMaxAttempts(3),
		contractsapi.WithRetryBackoff(1*time.Millisecond),
	)
	require.NoError(t, err)

	hashes, err := bi.InsertAll(context.Background(), makeInputs(20)) // 2 chunks
	require.Error(t, err)
	assert.Empty(t, hashes, "no chunks succeeded")

	var bie *contractsapi.BulkInsertError
	require.ErrorAs(t, err, &bie)
	assert.Equal(t, 0, bie.FailedChunkIndex, "first chunk should have failed")
	assert.True(t, errors.Is(err, kwiltypes.ErrInvalidNonce), "underlying error should unwrap to ErrInvalidNonce")
}

func TestBulkInserter_UnknownError_FailsFast(t *testing.T) {
	customErr := errors.New("some other error")
	bc := &mockBroadcaster{
		failNext: 100,
		failErr:  customErr,
	}
	tc := &mockTxClient{ledgerNonce: 50}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithMaxAttempts(5),
		contractsapi.WithRetryBackoff(1*time.Millisecond),
	)
	require.NoError(t, err)

	_, err = bi.InsertAll(context.Background(), makeInputs(10))
	require.Error(t, err)

	// Unknown error should fail without retrying
	assert.Len(t, bc.snapshot(), 1, "unknown error should fail fast (no retries)")
}

func TestBulkInserter_FailureMidway_ReportsCorrectIndex(t *testing.T) {
	tc := &mockTxClient{ledgerNonce: 0}
	type chunkedFailBroadcaster struct {
		mu       sync.Mutex
		callNum  int
		failFrom int
		failErr  error
	}
	cb := &chunkedFailBroadcaster{
		failFrom: 3,
		failErr:  fmt.Errorf("wrapped: %w", kwiltypes.ErrInvalidNonce),
	}
	insertFn := func(_ context.Context, _ []sdktypes.InsertRecordInput, _ ...kwilclient.TxOpt) (kwiltypes.Hash, error) {
		cb.mu.Lock()
		defer cb.mu.Unlock()
		cb.callNum++
		if cb.callNum >= cb.failFrom {
			return kwiltypes.Hash{}, cb.failErr
		}
		var h kwiltypes.Hash
		h[0] = byte(cb.callNum)
		return h, nil
	}
	wrapper := &funcBroadcaster{insertFn: insertFn}

	bi, err := contractsapi.NewBulkInserter(wrapper, tc, newTestSigner(t),
		contractsapi.WithBatchSize(10),
		contractsapi.WithMaxAttempts(2),
		contractsapi.WithRetryBackoff(1*time.Millisecond),
	)
	require.NoError(t, err)

	hashes, err := bi.InsertAll(context.Background(), makeInputs(50)) // 5 chunks
	require.Error(t, err)
	assert.Len(t, hashes, 2, "first two chunks should have succeeded")

	var bie *contractsapi.BulkInsertError
	require.ErrorAs(t, err, &bie)
	assert.Equal(t, 2, bie.FailedChunkIndex, "third chunk (index 2) should be the failing one")
	assert.False(t, bie.DrainFailure, "broadcast failure, not drain failure")
}

// funcBroadcaster lets a test supply an InsertRecords closure.
type funcBroadcaster struct {
	insertFn func(ctx context.Context, inputs []sdktypes.InsertRecordInput, opts ...kwilclient.TxOpt) (kwiltypes.Hash, error)
}

func (f *funcBroadcaster) InsertRecords(ctx context.Context, inputs []sdktypes.InsertRecordInput, opts ...kwilclient.TxOpt) (kwiltypes.Hash, error) {
	return f.insertFn(ctx, inputs, opts...)
}

func TestBulkInserter_ContextCancellation_DuringBackoff(t *testing.T) {
	bc := &mockBroadcaster{
		failNext: 100,
		failErr:  fmt.Errorf("wrapped: %w", kwiltypes.ErrMempoolFull),
	}
	tc := &mockTxClient{ledgerNonce: 0}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithMaxAttempts(10),
		contractsapi.WithRetryBackoff(100*time.Millisecond),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = bi.InsertAll(ctx, makeInputs(10))
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "context cancellation should propagate")
}

func TestBulkInserter_ChunkingByBatchSize(t *testing.T) {
	bc := &mockBroadcaster{}
	tc := &mockTxClient{}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithBatchSize(7),
	)
	require.NoError(t, err)

	_, err = bi.InsertAll(context.Background(), makeInputs(20)) // 7+7+6
	require.NoError(t, err)

	calls := bc.snapshot()
	require.Len(t, calls, 3)
	assert.Equal(t, 7, calls[0].chunkSize)
	assert.Equal(t, 7, calls[1].chunkSize)
	assert.Equal(t, 6, calls[2].chunkSize, "last chunk gets the remainder")
}

func TestBulkInserter_DrainFailure_FlagsAndReportsAllBroadcast(t *testing.T) {
	bc := &mockBroadcaster{}
	tc := &mockTxClient{
		ledgerNonce: 0,
		waitTxErr:   errors.New("wait failed: timeout"),
	}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t),
		contractsapi.WithBatchSize(10),
		contractsapi.WithMaxInflight(100),
	)
	require.NoError(t, err)

	hashes, err := bi.InsertAll(context.Background(), makeInputs(30)) // 3 chunks
	require.Error(t, err)
	assert.Len(t, hashes, 3, "all 3 chunks were broadcast successfully before final drain failed")

	var bie *contractsapi.BulkInsertError
	require.ErrorAs(t, err, &bie)
	assert.True(t, bie.DrainFailure, "should be flagged as drain failure")
	assert.Equal(t, 3, bie.FailedChunkIndex, "FailedChunkIndex should equal total chunks broadcast")
}

func TestBulkInserter_PassesNonceAndSyncBroadcastOpts(t *testing.T) {
	bc := &mockBroadcaster{}
	tc := &mockTxClient{ledgerNonce: 200}
	bi, err := contractsapi.NewBulkInserter(bc, tc, newTestSigner(t))
	require.NoError(t, err)

	_, err = bi.InsertAll(context.Background(), makeInputs(10))
	require.NoError(t, err)

	calls := bc.snapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, int64(201), calls[0].nonce)
	assert.False(t, calls[0].syncBcast, "BulkInserter must always set SyncBroadcast=false")
}
