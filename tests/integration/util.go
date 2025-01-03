package integration

import (
	"context"
	"github.com/golang-sql/civil"
	"github.com/kwilteam/kwil-db/core/types/transactions"
	"github.com/stretchr/testify/assert"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"testing"
	"time"
)

// ## Constants

const TestPrivateKey = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
const TestKwilProvider = "http://localhost:8484"

// ## Helper functions

// unsafeParseDate is a helper function to parse a date string into a civil.Date, panicking on error.
func unsafeParseDate(dateStr string) *civil.Date {
	date, err := civil.ParseDate(dateStr)
	if err != nil {
		panic(err)
	}
	return &date
}

// waitTxToBeMinedWithSuccess waits for a transaction to be successful, failing the test if it fails.
func waitTxToBeMinedWithSuccess(t *testing.T, ctx context.Context, client *tnclient.Client, txHash transactions.TxHash) {
	txRes, err := client.WaitForTx(ctx, txHash, time.Second)
	assertNoErrorOrFail(t, err, "Transaction failed")
	if !assert.Equal(t, transactions.CodeOk, transactions.TxCode(txRes.TxResult.Code), "Transaction code not OK: %s", txRes.TxResult.Log) {
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
	streamId util.StreamId,
	data []types.InsertRecordInput,
) {
	deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
	assertNoErrorOrFail(t, err, "Failed to deploy stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	address, err := util.NewEthereumAddressFromBytes(tnClient.GetSigner().Identity())
	assertNoErrorOrFail(t, err, "Failed to create signer address")

	streamLocator := types.StreamLocator{
		StreamId:     streamId,
		DataProvider: address,
	}

	deployedStream, err := tnClient.LoadPrimitiveStream(streamLocator)
	assertNoErrorOrFail(t, err, "Failed to load stream")

	txHashInit, err := deployedStream.InitializeStream(ctx)
	assertNoErrorOrFail(t, err, "Failed to initialize stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInit)

	txHashInsert, err := deployedStream.InsertRecords(ctx, data)
	assertNoErrorOrFail(t, err, "Failed to insert record")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInsert)
}

func deployTestPrimitiveStreamUnixWithData(
	t *testing.T,
	ctx context.Context,
	tnClient *tnclient.Client,
	streamId util.StreamId,
	data []types.InsertRecordUnixInput,
) {
	deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitiveUnix)
	assertNoErrorOrFail(t, err, "Failed to deploy stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	address, err := util.NewEthereumAddressFromBytes(tnClient.GetSigner().Identity())
	assertNoErrorOrFail(t, err, "Failed to create signer address")

	streamLocator := types.StreamLocator{
		StreamId:     streamId,
		DataProvider: address,
	}

	deployedStream, err := tnClient.LoadPrimitiveStream(streamLocator)
	assertNoErrorOrFail(t, err, "Failed to load stream")

	txHashInit, err := deployedStream.InitializeStream(ctx)
	assertNoErrorOrFail(t, err, "Failed to initialize stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInit)

	txHashInsert, err := deployedStream.InsertRecordsUnix(ctx, data)
	assertNoErrorOrFail(t, err, "Failed to insert record")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInsert)
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

	address, err := util.NewEthereumAddressFromBytes(tnClient.GetSigner().Identity())
	assertNoErrorOrFail(t, err, "Failed to create signer address")

	streamLocator := types.StreamLocator{
		StreamId:     streamId,
		DataProvider: address,
	}

	deployedStream, err := tnClient.LoadComposedStream(streamLocator)
	assertNoErrorOrFail(t, err, "Failed to load stream")

	txHashInit, err := deployedStream.InitializeStream(ctx)
	assertNoErrorOrFail(t, err, "Failed to initialize stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInit)

	txHashTax, err := deployedStream.SetTaxonomy(ctx, taxonomies)
	assertNoErrorOrFail(t, err, "Failed to set taxonomy")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashTax)
}
