// BulkInserter example.
//
// Demonstrates high-throughput record ingestion using BulkInserter, which
// pipelines insert_records broadcasts with a cached nonce instead of
// awaiting inclusion between every transaction.
//
// Flow:
//  1. Connect to a local node with a test private key
//  2. Generate a stream ID; if a stream with that ID already exists, drop it
//  3. Deploy a fresh primitive stream
//  4. Bulk-insert ~25 records (3 chunks of 10/10/5)
//  5. Read records back and confirm count + values
//  6. Drop the test stream
//
// Run against a local node: `go run ./examples/bulk_insert_example`
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

const (
	// Test-only private key. NEVER use this for real funds.
	testPrivateKey = "0000000000000000000000000000000000000000000000000000000000000001"
	// Default local node endpoint (single-node devnet via `task single:start`).
	endpoint = "http://localhost:8484"
	// Number of records to insert. With batchSize=10 → 3 chunks (10+10+5).
	numRecords = 25
)

func main() {
	ctx := context.Background()

	// 1. Build signer + client
	pk, err := crypto.Secp256k1PrivateKeyFromHex(testPrivateKey)
	if err != nil {
		log.Fatalf("parse private key: %v", err)
	}
	tnClient, err := tnclient.NewClient(ctx, endpoint,
		tnclient.WithSigner(&auth.EthPersonalSigner{Key: *pk}),
	)
	if err != nil {
		log.Fatalf("connect to node at %s: %v", endpoint, err)
	}
	addr := tnClient.Address()
	myAddress := addr.Address()
	log.Printf("connected as %s", myAddress)

	// 2. Generate stream ID and best-effort drop any existing stream
	streamID := util.GenerateStreamId("bulk-insert-example")
	log.Printf("stream id: %s", streamID)
	bestEffortDrop(ctx, tnClient, streamID)

	// 3. Deploy fresh primitive stream
	deployTx, err := tnClient.DeployStream(ctx, streamID, types.StreamTypePrimitive)
	if err != nil {
		log.Fatalf("deploy stream: %v", err)
	}
	if err := waitOK(ctx, tnClient, deployTx, "deploy"); err != nil {
		log.Fatal(err)
	}
	log.Printf("stream deployed (tx %s)", deployTx)

	// Always clean up at the end
	defer bestEffortDrop(ctx, tnClient, streamID)

	// 4. Bulk-insert records via BulkInserter
	inserter, err := tnClient.LoadBulkInserter()
	if err != nil {
		log.Fatalf("load bulk inserter: %v", err)
	}

	inputs := makeInputs(myAddress, streamID.String(), numRecords)
	log.Printf("broadcasting %d records via BulkInserter (batchSize=10)...", len(inputs))

	start := time.Now()
	hashes, err := inserter.InsertAll(ctx, inputs)
	if err != nil {
		log.Fatalf("bulk insert: %v", err)
	}
	log.Printf("done: %d chunks broadcast + drained in %.2fs (%.0fms/chunk avg)",
		len(hashes), time.Since(start).Seconds(),
		float64(time.Since(start).Milliseconds())/float64(len(hashes)))

	// 5. Read records back and confirm
	from := int(time.Now().AddDate(0, 0, -numRecords-1).Unix())
	to := int(time.Now().AddDate(0, 0, 1).Unix())
	actions, err := tnClient.LoadActions()
	if err != nil {
		log.Fatalf("load actions: %v", err)
	}
	result, err := actions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: myAddress,
		StreamId:     streamID.String(),
		From:         &from,
		To:           &to,
	})
	if err != nil {
		log.Fatalf("get record: %v", err)
	}

	if got, want := len(result.Results), numRecords; got != want {
		log.Fatalf("record count mismatch: got %d, want %d", got, want)
	}
	fmt.Printf("\nFirst 3 records read back:\n")
	for _, r := range result.Results[:3] {
		fmt.Printf("  EventTime=%d Value=%s\n", r.EventTime, r.Value.String())
	}
	fmt.Printf("...\nTotal verified: %d records\n", len(result.Results))
}

// makeInputs builds a slice of synthetic InsertRecordInputs at one-day intervals.
func makeInputs(dataProvider, streamID string, n int) []types.InsertRecordInput {
	inputs := make([]types.InsertRecordInput, n)
	startDate := time.Now().AddDate(0, 0, -n)
	for i := 0; i < n; i++ {
		inputs[i] = types.InsertRecordInput{
			DataProvider: dataProvider,
			StreamId:     streamID,
			EventTime:    int(startDate.AddDate(0, 0, i).Unix()),
			Value:        float64(i + 1), // +1 to avoid zero (filtered by consensus)
		}
	}
	return inputs
}

// bestEffortDrop tries to destroy a stream if it exists; it logs and ignores
// errors (most commonly: "stream does not exist").
func bestEffortDrop(ctx context.Context, c *tnclient.Client, streamID util.StreamId) {
	tx, err := c.DestroyStream(ctx, streamID)
	if err != nil {
		log.Printf("(no existing stream to drop, or drop failed: %v)", err)
		return
	}
	if err := waitOK(ctx, c, tx, "destroy"); err != nil {
		log.Printf("(drop tx returned error: %v)", err)
		return
	}
	log.Printf("dropped existing stream (tx %s)", tx)
}

// waitOK waits for a tx and returns an error if it did not commit successfully.
func waitOK(ctx context.Context, c *tnclient.Client, tx kwiltypes.Hash, label string) error {
	res, err := c.WaitForTx(ctx, tx, time.Second)
	if err != nil {
		return fmt.Errorf("%s: wait: %w", label, err)
	}
	if res.Result.Code != uint32(kwiltypes.CodeOk) {
		return fmt.Errorf("%s: tx failed: %s", label, res.Result.Log)
	}
	return nil
}
