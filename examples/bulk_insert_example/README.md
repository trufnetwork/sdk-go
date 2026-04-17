# Bulk Insert Example

Demonstrates `BulkInserter` — pipelined high-throughput record insertion that
keeps a single signer within the protocol's 10-row-per-tx cap while
broadcasting hundreds of transactions per minute.

## What it does

1. Connects to a local TN node with a test private key
2. Generates a stream ID and best-effort drops any existing stream with that ID
3. Deploys a fresh primitive stream
4. Bulk-inserts 25 synthetic records via `BulkInserter` (3 chunks of 10/10/5)
5. Reads the records back and confirms count + values
6. Drops the test stream

## Why use BulkInserter

Calling `client.LoadPrimitiveActions().InsertRecords(...)` in a loop forces the
SDK to wait for each transaction to be **included in a block** (~1–2s per call)
before broadcasting the next. For 1,000 records that's 25+ minutes.

`BulkInserter` instead:

- Caches the nonce locally (one initial fetch from the ledger, then increments)
- Broadcasts each chunk fire-and-forget (`WithSyncBroadcast(false)`) — admission
  takes ~50ms versus inclusion's 1–2s
- Drains inflight hashes in batches via `WaitTx`
- Retries automatically on `ErrInvalidNonce` (resets the cache and refetches)
  and `ErrMempoolFull` (backs off, keeps the cache)

Result: 1,000 records land in roughly one minute on a typical node, instead of
half an hour.

## Running

Spin up a local node first (from the `node` repo):

```bash
task single:start
```

Then run the example:

```bash
go run ./examples/bulk_insert_example
```

The example uses the test-only private key
`0000000000000000000000000000000000000000000000000000000000000001`. Do not use
this key for real funds.

## Expected output

```
2026/04/17 17:30:01 connected as 0x7e5f4552091a69125d5dfcb7b8c2659029395bdf
2026/04/17 17:30:01 stream id: stbulkinsert000000000000000000000
2026/04/17 17:30:01 (no existing stream to drop, or drop failed: ...)
2026/04/17 17:30:02 stream deployed (tx 0x...)
2026/04/17 17:30:02 broadcasting 25 records via BulkInserter (batchSize=10)...
2026/04/17 17:30:03 done: 3 chunks broadcast + drained in 1.05s (350ms/chunk avg)

First 3 records read back:
  EventTime=1711234567 Value=1.000000000000000000
  EventTime=1711320967 Value=2.000000000000000000
  EventTime=1711407367 Value=3.000000000000000000
...
Total verified: 25 records
```

## Customizing

- **Larger workloads**: change `numRecords` at the top of `main.go`. The math
  is `ceil(numRecords / 10)` chunks.
- **Different throughput knobs**: pass options to `LoadBulkInserter`:

  ```go
  inserter, err := tnClient.LoadBulkInserter(
      contractsapi.WithBatchSize(10),       // protocol cap
      contractsapi.WithMaxInflight(500),    // drain frequency
      contractsapi.WithMaxAttempts(5),      // retries on transient errors
      contractsapi.WithRetryBackoff(2 * time.Second),
  )
  ```
- **Testnet/mainnet**: change `endpoint` and the private key. Note that the
  account must have the `system:network_writer` role to deploy streams.

## Related

- Source: [`core/contractsapi/bulk_inserter.go`](../../core/contractsapi/bulk_inserter.go)
- Pattern reference: [`tn_attestation/extension.go`](https://github.com/trufnetwork/node/blob/main/extensions/tn_attestation/extension.go)
  in the node repo (PR #1356) — same cached-nonce design that solved the
  attestation cron's "invalid nonce" noise.
