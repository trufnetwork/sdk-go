package integration

import (
	"context"
	"testing"
	"time"

	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// ## Helper functions

// waitTxToBeMinedWithSuccess waits for a transaction to be successful, failing the test if it fails.
func waitTxToBeMinedWithSuccess(t *testing.T, ctx context.Context, client *tnclient.Client, txHash kwiltypes.Hash) {
	txRes, err := client.WaitForTx(ctx, txHash, time.Second)
	assertNoErrorOrFail(t, err, "Transaction failed")
	if !assert.Equal(t, kwiltypes.CodeOk, kwiltypes.TxCode(txRes.Result.Code), "Transaction code not OK: %s", txRes.Result.Log) {
		t.FailNow()
	}
}

// assertNoErrorOrFail asserts that an error is nil, failing the test if it is not.
func assertNoErrorOrFail(t *testing.T, err error, msg string) {
	if !assert.NoError(t, err, msg) {
		t.FailNow()
	}
}

func deployTestPrimitiveStreamWithData(
	t *testing.T,
	ctx context.Context,
	tnClient *tnclient.Client,
	streamIds []util.StreamId,
	data []types.InsertRecordInput,
) {
	streamDefs := make([]types.StreamDefinition, len(streamIds))
	for i, streamId := range streamIds {
		streamDefs[i] = types.StreamDefinition{
			StreamId:   streamId,
			StreamType: types.StreamTypePrimitive,
		}
	}
	batchDeployTxHash, err := tnClient.BatchDeployStreams(ctx, streamDefs)
	assertNoErrorOrFail(t, err, "Failed to deploy streams")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, batchDeployTxHash)

	if len(data) > 0 {
		primitiveActions, err := tnClient.LoadPrimitiveActions()
		assertNoErrorOrFail(t, err, "Failed to load stream")

		txHashInsert, err := primitiveActions.InsertRecords(ctx, data)
		assertNoErrorOrFail(t, err, "Failed to insert records")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInsert)
	}
}

func deployTestComposedStreamWithTaxonomy(
	t *testing.T,
	ctx context.Context,
	tnClient *tnclient.Client,
	streamId util.StreamId,
	taxonomies types.Taxonomy,
) {
	deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypeComposed)
	assertNoErrorOrFail(t, err, "Failed to deploy stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	deployedStream, err := tnClient.LoadComposedActions()
	assertNoErrorOrFail(t, err, "Failed to load stream")

	txHashTax, err := deployedStream.InsertTaxonomy(ctx, taxonomies)
	assertNoErrorOrFail(t, err, "Failed to set taxonomy")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashTax)
}
