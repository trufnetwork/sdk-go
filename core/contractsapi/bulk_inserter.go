package contractsapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	kwilclient "github.com/trufnetwork/kwil-db/core/client/types"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/kwil-db/core/log"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	sdktypes "github.com/trufnetwork/sdk-go/core/types"
)

// BulkInsertBroadcaster is the minimal broadcast interface BulkInserter needs.
// sdktypes.IPrimitiveAction satisfies this; declaring a smaller interface
// makes the helper trivially mockable in unit tests.
type BulkInsertBroadcaster interface {
	InsertRecords(ctx context.Context, inputs []sdktypes.InsertRecordInput, opts ...kwilclient.TxOpt) (kwiltypes.Hash, error)
}

// BulkInsertTxClient is the minimal tx/account interface BulkInserter needs.
// kwilclient.Client (and *gatewayclient.GatewayClient) satisfy this.
type BulkInsertTxClient interface {
	GetAccount(ctx context.Context, accountID *kwiltypes.AccountID, status kwiltypes.AccountStatus) (*kwiltypes.Account, error)
	WaitTx(ctx context.Context, txHash kwiltypes.Hash, interval time.Duration) (*kwiltypes.TxQueryResponse, error)
}

// BulkInserter pipelines insert_records broadcasts against a single signer to
// achieve high throughput within the protocol's 10-row-per-tx cap.
//
// Mirrors the cached-nonce + fire-and-forget pattern from
// node/extensions/tn_attestation/extension.go (PR kwilteam/node#1356), which
// solves the same problem for the settlement cron submitter.
//
// The mempool admits transactions strictly in nonce order
// (kwil-db/node/txapp/mempool.go:180-204): tx N+2 only enters once tx N+1 has
// been admitted. Crucially, admission is fast (~50ms HTTP) while inclusion is
// slow (~1-2s block time). Pipelined sequential broadcast over a single
// connection lets us submit ~20 tx/s versus ~0.5 tx/s when waiting for
// inclusion between every broadcast.
//
// Concurrent broadcast from one signer is NOT safe: HTTP reordering produces
// out-of-order arrivals which the mempool rejects with ErrInvalidNonce. Use
// one BulkInserter per signer key, single-threaded.
//
// Recovery: on ErrInvalidNonce the cache is cleared and re-fetched from the
// ledger on the next call. On ErrMempoolFull and "node is catching up" we
// backoff but keep the cache (the nonce is still valid; the network or the
// receiving backend is just busy).
type BulkInserter struct {
	broadcaster BulkInsertBroadcaster
	txClient    BulkInsertTxClient
	accountID   *kwiltypes.AccountID
	logger      log.Logger

	batchSize          int
	maxInflight        int
	maxAttempts        int
	catchupMaxAttempts int
	retryBackoff       time.Duration
	catchupBackoff     time.Duration
	waitInterval       time.Duration
	progressLogEveryN  int

	mu               sync.Mutex
	pendingNonce     int64
	nonceInitialized bool
}

// BulkInserterOption configures a BulkInserter.
type BulkInserterOption func(*BulkInserter)

// WithBatchSize sets how many records go into each insert_records transaction.
// Must be <= the protocol cap (currently 10). Default: 10.
func WithBatchSize(n int) BulkInserterOption {
	return func(b *BulkInserter) {
		if n > 0 {
			b.batchSize = n
		}
	}
}

// WithMaxInflight sets how many broadcasts may be queued before InsertAll
// drains them via WaitTx. Lower values use less memory but await more often.
// Default: 200.
func WithMaxInflight(n int) BulkInserterOption {
	return func(b *BulkInserter) {
		if n > 0 {
			b.maxInflight = n
		}
	}
}

// WithMaxAttempts sets the maximum number of attempts per chunk (initial
// attempt plus retries) on non-catchup transient errors (invalid nonce,
// mempool full). Catch-up errors have their own budget — see
// WithCatchupMaxAttempts. Default: 5.
func WithMaxAttempts(n int) BulkInserterOption {
	return func(b *BulkInserter) {
		if n > 0 {
			b.maxAttempts = n
		}
	}
}

// WithCatchupMaxAttempts sets the maximum number of attempts per chunk on
// "node is catching up" rejections. Separate from WithMaxAttempts because
// real catch-up events on the public RPC backend can run minutes long
// (sentry replaying blocks after a peer flap or restart) while invalid-nonce
// and mempool-full are typically resolved within seconds. Sharing one
// budget would either fail too quickly on a real catch-up or waste
// retries on a hopeless nonce. Default: 20.
func WithCatchupMaxAttempts(n int) BulkInserterOption {
	return func(b *BulkInserter) {
		if n > 0 {
			b.catchupMaxAttempts = n
		}
	}
}

// WithProgressLogEveryN enables INFO-level progress logging from InsertAll
// every N chunks. Useful for long-running bulk loads where the operator
// otherwise has no visibility between the "submitting" and "submitted"
// log lines (which can be hours apart). Logs include chunks done / total,
// rows done, elapsed time, current chunks/sec rate, and an ETA.
// 0 disables progress logging. Default: 0.
func WithProgressLogEveryN(n int) BulkInserterOption {
	return func(b *BulkInserter) {
		if n > 0 {
			b.progressLogEveryN = n
		}
	}
}

// WithRetryBackoff sets the base backoff duration. Actual delay is
// backoff * (attempt + 1). Default: 2s.
func WithRetryBackoff(d time.Duration) BulkInserterOption {
	return func(b *BulkInserter) {
		if d > 0 {
			b.retryBackoff = d
		}
	}
}

// WithCatchupBackoff sets the base backoff used when the broadcast backend
// rejects with "node is catching up". Catch-up events typically resolve in
// tens of seconds (and occasionally minutes when a sentry is replaying
// significant history), so this defaults much higher than WithRetryBackoff.
// Actual delay is catchupBackoff * (attempt + 1). With the default of 15s
// and WithCatchupMaxAttempts default 20, the worst-case wait per chunk is
// 15+30+45+...+300s ≈ 52 minutes — comfortably long enough to ride out
// every catch-up event seen in production so far without abandoning the
// whole batch. Default: 15s.
func WithCatchupBackoff(d time.Duration) BulkInserterOption {
	return func(b *BulkInserter) {
		if d > 0 {
			b.catchupBackoff = d
		}
	}
}

// WithWaitInterval sets the polling interval passed to WaitTx during drain.
// Default: 1s.
func WithWaitInterval(d time.Duration) BulkInserterOption {
	return func(b *BulkInserter) {
		if d > 0 {
			b.waitInterval = d
		}
	}
}

// WithLogger attaches a logger. Default: discard.
func WithLogger(logger log.Logger) BulkInserterOption {
	return func(b *BulkInserter) {
		b.logger = logger
	}
}

// NewBulkInserter constructs a BulkInserter wired to a broadcaster (for
// insert_records) and a tx client (for account/nonce queries and tx waits).
// The signer is used once to derive the account ID for nonce lookups.
//
// In production, pass an sdktypes.IPrimitiveAction as the broadcaster and a
// kwilclient.Client (or *gatewayclient.GatewayClient) as the tx client.
// Tests can pass any types satisfying BulkInsertBroadcaster and
// BulkInsertTxClient.
//
// Most callers should use tnclient.Client.LoadBulkInserter() instead.
func NewBulkInserter(
	broadcaster BulkInsertBroadcaster,
	txClient BulkInsertTxClient,
	signer auth.Signer,
	opts ...BulkInserterOption,
) (*BulkInserter, error) {
	if broadcaster == nil {
		return nil, errors.New("broadcaster is required")
	}
	if txClient == nil {
		return nil, errors.New("tx client is required")
	}
	if signer == nil {
		return nil, errors.New("signer is required")
	}

	accountID, err := kwiltypes.GetSignerAccount(signer)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "derive account id from signer")
	}

	b := &BulkInserter{
		broadcaster:        broadcaster,
		txClient:           txClient,
		accountID:          accountID,
		logger:             log.DiscardLogger,
		batchSize:          10,
		maxInflight:        200,
		maxAttempts:        5,
		catchupMaxAttempts: 20,
		retryBackoff:       2 * time.Second,
		catchupBackoff:     15 * time.Second,
		waitInterval:       1 * time.Second,
		progressLogEveryN:  0,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b, nil
}

// BulkInsertError reports a bulk insert failure.
//
// When DrainFailure is false, FailedChunkIndex is the index of the first
// chunk that failed to broadcast after exhausting retries. The caller can
// resume from inputs[FailedChunkIndex*batchSize:] after fixing the
// underlying issue.
//
// When DrainFailure is true, all broadcasts succeeded but waiting for
// inclusion (WaitTx) failed. FailedChunkIndex equals the total number of
// chunks broadcast (i.e. all of them); the broadcast hashes are returned
// as the first value alongside the error so the caller can investigate
// or poll inclusion separately.
type BulkInsertError struct {
	FailedChunkIndex int
	DrainFailure     bool
	LastError        error
}

func (e *BulkInsertError) Error() string {
	if e.DrainFailure {
		return fmt.Sprintf("bulk insert drain failed after %d chunks broadcast: %v",
			e.FailedChunkIndex, e.LastError)
	}
	return fmt.Sprintf("bulk insert failed at chunk %d: %v", e.FailedChunkIndex, e.LastError)
}

func (e *BulkInsertError) Unwrap() error {
	return e.LastError
}

// InsertAll chunks inputs into batchSize-sized groups, broadcasts each
// chunk pipelined (no wait between broadcasts), and drains the inflight
// queue every maxInflight broadcasts plus once at the end.
//
// Returns the tx hashes in submission order. On a chunk failure after
// retries, returns the hashes broadcast so far plus a *BulkInsertError
// indicating where to resume.
//
// When WithProgressLogEveryN(n>0) is set, emits an INFO log every n chunks
// with chunks done / total / rate / ETA so operators have visibility into
// long-running loads. Without it, the only logs are the WARN lines on
// retry events (invalid nonce, mempool full, catching up) — which means
// silence between start and finish, which may be hours apart.
func (b *BulkInserter) InsertAll(
	ctx context.Context,
	inputs []sdktypes.InsertRecordInput,
) ([]kwiltypes.Hash, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	chunks := chunkInputs(inputs, b.batchSize)
	allHashes := make([]kwiltypes.Hash, 0, len(chunks))
	inflight := make([]kwiltypes.Hash, 0, b.maxInflight)
	start := time.Now()

	for i, chunk := range chunks {
		hash, err := b.broadcastWithRetry(ctx, chunk)
		if err != nil {
			return allHashes, &BulkInsertError{FailedChunkIndex: i, LastError: err}
		}
		allHashes = append(allHashes, hash)
		inflight = append(inflight, hash)

		if b.progressLogEveryN > 0 && (i+1)%b.progressLogEveryN == 0 {
			b.logProgress(i+1, len(chunks), start)
		}

		if len(inflight) >= b.maxInflight {
			if err := b.drain(ctx, inflight); err != nil {
				return allHashes, &BulkInsertError{
					FailedChunkIndex: len(allHashes),
					DrainFailure:     true,
					LastError:        err,
				}
			}
			inflight = inflight[:0]
		}
	}

	if len(inflight) > 0 {
		if err := b.drain(ctx, inflight); err != nil {
			return allHashes, &BulkInsertError{
				FailedChunkIndex: len(allHashes),
				DrainFailure:     true,
				LastError:        err,
			}
		}
	}

	if b.progressLogEveryN > 0 {
		b.logProgress(len(chunks), len(chunks), start)
	}

	return allHashes, nil
}

// logProgress emits a structured progress line. Best effort — the logger may
// be DiscardLogger (the default) in which case this is a no-op besides the
// time math, which is cheap.
func (b *BulkInserter) logProgress(done, total int, start time.Time) {
	elapsed := time.Since(start)
	var chunksPerSec, etaSec float64
	if elapsed.Seconds() > 0 {
		chunksPerSec = float64(done) / elapsed.Seconds()
	}
	if chunksPerSec > 0 {
		etaSec = float64(total-done) / chunksPerSec
	}
	b.logger.Info("bulk_inserter: progress",
		"chunks_done", done,
		"chunks_total", total,
		"rows_done", done*b.batchSize,
		"elapsed_sec", int(elapsed.Seconds()),
		"chunks_per_sec", chunksPerSec,
		"eta_sec", int(etaSec),
	)
}

func (b *BulkInserter) broadcastWithRetry(
	ctx context.Context,
	chunk []sdktypes.InsertRecordInput,
) (hash kwiltypes.Hash, retErr error) {
	var (
		nonce             int64
		nonceLoaded       bool
		transientAttempts int // counts ErrInvalidNonce + ErrMempoolFull tries
		catchupAttempts   int // counts "node is catching up" tries
	)
	// On any error exit (attempt exhaustion, context cancellation during
	// backoff, or an unhandled error), drop the cached nonce. The reserved
	// nonce was never admitted to the network, so a subsequent InsertAll
	// would otherwise broadcast the next nonce first and only recover via
	// ErrInvalidNonce — wasting one round-trip every time. Forcing a fresh
	// GetAccount on the next call resyncs us with what the ledger actually
	// admitted. Idempotent with the ErrInvalidNonce path's reset.
	defer func() {
		if retErr != nil && nonceLoaded {
			b.resetNonce()
		}
	}()
	for {
		// Pull a fresh nonce only on the first attempt OR after an
		// ErrInvalidNonce reset. On ErrMempoolFull we keep the same nonce
		// because the tx was rejected at admission — the mempool's
		// expected nonce for this account hasn't moved.
		if !nonceLoaded {
			n, err := b.nextNonce(ctx)
			if err != nil {
				return kwiltypes.Hash{}, pkgerrors.Wrap(err, "fetch nonce")
			}
			nonce = n
			nonceLoaded = true
		}

		hash, err := b.broadcaster.InsertRecords(ctx, chunk,
			kwilclient.WithNonce(nonce),
			kwilclient.WithSyncBroadcast(false),
		)
		if err == nil {
			return hash, nil
		}

		switch {
		case errors.Is(err, kwiltypes.ErrInvalidNonce):
			if transientAttempts+1 >= b.maxAttempts {
				return kwiltypes.Hash{}, err
			}
			b.logger.Warn("bulk_inserter: invalid nonce, resetting cache",
				"attempt", transientAttempts+1, "nonce", nonce, "err", err)
			b.resetNonce()
			nonceLoaded = false // force re-fetch on next attempt
			if waitErr := b.backoff(ctx, transientAttempts); waitErr != nil {
				return kwiltypes.Hash{}, waitErr
			}
			transientAttempts++
		case errors.Is(err, kwiltypes.ErrMempoolFull):
			if transientAttempts+1 >= b.maxAttempts {
				return kwiltypes.Hash{}, err
			}
			b.logger.Warn("bulk_inserter: mempool full, backing off",
				"attempt", transientAttempts+1, "nonce", nonce, "err", err)
			// Keep nonceLoaded=true so we retry with the same nonce.
			if waitErr := b.backoff(ctx, transientAttempts); waitErr != nil {
				return kwiltypes.Hash{}, waitErr
			}
			transientAttempts++
		case isCatchingUpErr(err):
			// Catch-up gets its own larger budget — see WithCatchupMaxAttempts
			// rationale. Real catch-up events on a public RPC backend (sentry
			// replaying blocks after a peer flap) routinely run minutes long;
			// sharing the transient budget here is what previously aborted
			// 4-hour BulkInserter runs after just 75 seconds of waiting.
			if catchupAttempts+1 >= b.catchupMaxAttempts {
				return kwiltypes.Hash{}, err
			}
			b.logger.Warn("bulk_inserter: backend catching up, backing off",
				"attempt", catchupAttempts+1,
				"max_attempts", b.catchupMaxAttempts,
				"nonce", nonce, "err", err)
			// Keep nonceLoaded=true — the backend never admitted the tx, our
			// cached nonce is still correct.
			if waitErr := b.backoffCatchup(ctx, catchupAttempts); waitErr != nil {
				return kwiltypes.Hash{}, waitErr
			}
			catchupAttempts++
		default:
			return kwiltypes.Hash{}, err
		}
	}
}

func (b *BulkInserter) backoff(ctx context.Context, attempt int) error {
	delay := b.retryBackoff * time.Duration(attempt+1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// backoffCatchup uses a longer base than backoff() because catch-up events
// typically resolve in tens of seconds, not single seconds. Linear ramp.
func (b *BulkInserter) backoffCatchup(ctx context.Context, attempt int) error {
	delay := b.catchupBackoff * time.Duration(attempt+1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// isCatchingUpErr reports whether err is a kwild "node is catching up"
// rejection. kwild emits this from node/node.go as a raw errors.New and the
// RPC layer surfaces it as a BroadcastError with TxCode 65535
// (CodeUnknownError). There is no exported sentinel in kwil-db today, so we
// match by substring on the literal message kwild produces.
//
// TODO: replace with errors.Is(err, kwiltypes.ErrCatchingUp) once kwil-db
// exports a typed sentinel + dedicated TxCode (P1 follow-up tracked in
// 0MainnetPredictionMarket/6IncidentTriage-NodeCatchingUp-2026-04-21.md).
func isCatchingUpErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "node is catching up")
}

func (b *BulkInserter) drain(ctx context.Context, hashes []kwiltypes.Hash) error {
	for _, h := range hashes {
		if _, err := b.txClient.WaitTx(ctx, h, b.waitInterval); err != nil {
			return pkgerrors.Wrapf(err, "wait for tx %s", h)
		}
	}
	return nil
}

func (b *BulkInserter) nextNonce(ctx context.Context) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.nonceInitialized {
		nonce := b.pendingNonce
		b.pendingNonce++
		return nonce, nil
	}

	account, err := b.txClient.GetAccount(ctx, b.accountID, kwiltypes.AccountStatusPending)
	if err != nil {
		return 0, pkgerrors.Wrap(err, "get account")
	}

	nonce := account.Nonce + 1
	b.pendingNonce = nonce + 1
	b.nonceInitialized = true
	return nonce, nil
}

func (b *BulkInserter) resetNonce() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nonceInitialized = false
	b.pendingNonce = 0
}

func chunkInputs(inputs []sdktypes.InsertRecordInput, size int) [][]sdktypes.InsertRecordInput {
	if size <= 0 {
		size = 10
	}
	chunks := make([][]sdktypes.InsertRecordInput, 0, (len(inputs)+size-1)/size)
	for i := 0; i < len(inputs); i += size {
		end := i + size
		if end > len(inputs) {
			end = len(inputs)
		}
		chunks = append(chunks, inputs[i:end])
	}
	return chunks
}
