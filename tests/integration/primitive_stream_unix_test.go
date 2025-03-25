package integration

import (
	"context"
	"testing"

	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// TestPrimitiveActions demonstrates the process of deploying, initializing, writing to,
// and reading from a primitive action in TN using the TN SDK.
func TestPrimitiveActions(t *testing.T) {
	ctx := context.Background()

	// Parse the private key for authentication
	// Note: In a production environment, use secure key management practices
	pk, err := crypto.Secp256k1PrivateKeyFromHex(TestPrivateKey)
	assertNoErrorOrFail(t, err, "Failed to parse private key")

	// Create a signer using the parsed private key
	signer := &auth.EthPersonalSigner{Key: *pk}

	// Initialize the TN client with the signer
	// Replace TestKwilProvider with the appropriate TN provider URL in your environment
	tnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(signer))
	assertNoErrorOrFail(t, err, "Failed to create client")

	// Generate a unique stream ID and locator
	// The stream ID is used to uniquely identify the stream within TN
	streamId := util.GenerateStreamId("test-primitive-stream-unix")

	// Set up cleanup to destroy the stream after test completion
	// This ensures that test streams don't persist in the network
	t.Cleanup(func() {
		destroyResult, err := tnClient.DestroyStream(ctx, streamId)
		assertNoErrorOrFail(t, err, "Failed to destroy stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
	})

	// Deploy and initialize stream once for all subtests
	deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
	assertNoErrorOrFail(t, err, "Failed to deploy stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	deployedPrimitiveStream, err := tnClient.LoadPrimitiveActions()
	assertNoErrorOrFail(t, err, "Failed to load stream")

	//t.Run("EmptyStreamOperations", func(t *testing.T) {
	//	// Query first record from empty stream
	//	firstRecord, err := deployedPrimitiveStream.GetFirstRecordUnix(ctx, types.GetFirstRecordUnixInput{})
	//	assertNoErrorOrFail(t, err, "Failed to query first record")
	//	assert.Nil(t, firstRecord, "Expected nil record from empty stream")
	//
	//	// Query first record with after date from empty stream
	//	afterDate := 1
	//	firstRecordAfter, err := deployedPrimitiveStream.GetFirstRecordUnix(ctx, types.GetFirstRecordUnixInput{
	//		AfterDate: &afterDate,
	//	})
	//	assertNoErrorOrFail(t, err, "Failed to query first record with after date")
	//	assert.Nil(t, firstRecordAfter, "Expected nil record from empty stream with after date")
	//})

	t.Run("DeploymentWriteAndReadOperations", func(t *testing.T) {
		// Insert a record into the stream
		// This demonstrates how to write data to the stream
		addr, _ := auth.EthSecp256k1Authenticator{}.Identifier(signer.CompactID())
		txHash, err := deployedPrimitiveStream.InsertRecord(ctx, types.InsertRecordInput{
			DataProvider: addr,
			StreamId:     streamId.String(),
			EventTime:    1,
			Value:        1,
		})
		assertNoErrorOrFail(t, err, "Failed to insert record")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)

		// Query records from the stream
		// This demonstrates how to read data from the stream
		//mockedDateFromUnix := 1
		//mockedDateToUnix := 1
		//records, err := deployedPrimitiveStream.GetRecordUnix(ctx, types.GetRecordUnixInput{
		//	DateFrom: &mockedDateFromUnix,
		//	DateTo:   &mockedDateToUnix,
		//})
		//assertNoErrorOrFail(t, err, "Failed to query records")

		// Verify the record's content
		// This ensures that the inserted data matches what we expect
		//assert.Len(t, records, 1, "Expected exactly one record")
		//assert.Equal(t, "1.000000000000000000", records[0].Value.String(), "Unexpected record value")
		//assert.Equal(t, 1, records[0].EventTime, "Unexpected record date")

		// Query the first record from the stream
		//firstRecord, err := deployedPrimitiveStream.GetFirstRecordUnix(ctx, types.GetFirstRecordUnixInput{})
		//assertNoErrorOrFail(t, err, "Failed to query first record")

		// Verify the first record's content
		//assert.NotNil(t, firstRecord, "Expected non-nil record")
		//assert.Equal(t, "1.000000000000000000", firstRecord.Value.String(), "Unexpected first record value")
		//assert.Equal(t, 1, firstRecord.EventTime, "Unexpected first record date")

		// Query index from the stream
		//index, err := deployedPrimitiveStream.GetIndexUnix(ctx, types.GetIndexUnixInput{
		//	DateFrom: &mockedDateFromUnix,
		//	DateTo:   &mockedDateToUnix,
		//})
		//assertNoErrorOrFail(t, err, "Failed to query index")

		// Verify the index's content
		// This ensures that the inserted data matches what we expect
		//assert.Len(t, index, 1, "Expected exactly one index")
		//assert.Equal(t, "100.000000000000000000", index[0].Value.String(), "Unexpected index value")
		//assert.Equal(t, 1, index[0].EventTime, "Unexpected index date")
	})
}
