package integration

//
//import (
//	"context"
//	"github.com/kwilteam/kwil-db/core/crypto"
//	"github.com/kwilteam/kwil-db/core/crypto/auth"
//	"github.com/stretchr/testify/assert"
//	"github.com/trufnetwork/sdk-go/core/tnclient"
//	"github.com/trufnetwork/sdk-go/core/types"
//	"github.com/trufnetwork/sdk-go/core/util"
//	"testing"
//)
//
//// TestPrimitiveStream demonstrates the process of deploying, initializing, writing to,
//// and reading from a primitive stream in TN using the TN SDK.
//func TestPrimitiveStream(t *testing.T) {
//	ctx := context.Background()
//
//	// Parse the private key for authentication
//	// Note: In a production environment, use secure key management practices
//	pk, err := crypto.Secp256k1PrivateKeyFromHex(TestPrivateKey)
//	assertNoErrorOrFail(t, err, "Failed to parse private key")
//
//	// Create a signer using the parsed private key
//	signer := &auth.EthPersonalSigner{Key: *pk}
//
//	// Initialize the TN client with the signer
//	// Replace TestKwilProvider with the appropriate TN provider URL in your environment
//	tnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(signer))
//	assertNoErrorOrFail(t, err, "Failed to create client")
//
//	// Generate a unique stream ID and locator
//	// The stream ID is used to uniquely identify the stream within TN
//	streamId := util.GenerateStreamId("test-primitive-stream")
//	streamLocator := tnClient.OwnStreamLocator(streamId)
//
//	// Set up cleanup to destroy the stream after test completion
//	// This ensures that test streams don't persist in the network
//	t.Cleanup(func() {
//		destroyResult, err := tnClient.DestroyStream(ctx, streamId)
//		assertNoErrorOrFail(t, err, "Failed to destroy stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//	})
//
//	// Subtest for deploying, initializing, writing to, and reading from a primitive stream
//	t.Run("DeploymentWriteAndReadOperations", func(t *testing.T) {
//		// Deploy a primitive stream
//		// This creates the stream contract on the TN
//		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
//		assertNoErrorOrFail(t, err, "Failed to deploy stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
//
//		// Load the deployed stream
//		// This step is necessary to interact with the stream after deployment
//		deployedPrimitiveStream, err := tnClient.LoadPrimitiveStream(streamLocator)
//		assertNoErrorOrFail(t, err, "Failed to load stream")
//
//		// Initialize the stream
//		// This step prepares the stream for data operations
//		txHashInit, err := deployedPrimitiveStream.InitializeStream(ctx)
//		assertNoErrorOrFail(t, err, "Failed to initialize stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInit)
//
//		// Insert a record into the stream
//		// This demonstrates how to write data to the stream
//		txHash, err := deployedPrimitiveStream.InsertRecords(ctx, []types.InsertRecordInput{
//			{
//				Value:     1,
//				DateValue: *unsafeParseDate("2020-01-01"),
//			},
//		})
//		assertNoErrorOrFail(t, err, "Failed to insert record")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
//
//		// Query records from the stream
//		// This demonstrates how to read data from the stream
//		records, err := deployedPrimitiveStream.GetRecord(ctx, types.GetRecordInput{
//			DateFrom: unsafeParseDate("2020-01-01"),
//			DateTo:   unsafeParseDate("2021-01-01"),
//		})
//		assertNoErrorOrFail(t, err, "Failed to query records")
//
//		// Verify the record's content
//		// This ensures that the inserted data matches what we expect
//		assert.Len(t, records, 1, "Expected exactly one record")
//		assert.Equal(t, "1.000000000000000000", records[0].Value.String(), "Unexpected record value")
//		assert.Equal(t, "2020-01-01", records[0].DateValue.String(), "Unexpected record date")
//
//		// Query index from the stream
//		index, err := deployedPrimitiveStream.GetIndex(ctx, types.GetIndexInput{
//			DateFrom: unsafeParseDate("2020-01-01"),
//			DateTo:   unsafeParseDate("2021-01-01"),
//		})
//		assertNoErrorOrFail(t, err, "Failed to query index")
//
//		// Verify the index's content
//		// This ensures that the inserted data matches what we expect
//		assert.Len(t, index, 1, "Expected exactly one index")
//		assert.Equal(t, "100.000000000000000000", index[0].Value.String(), "Unexpected index value")
//		assert.Equal(t, "2020-01-01", index[0].DateValue.String(), "Unexpected index date")
//
//		// Query the first record from the stream
//		firstRecord, err := deployedPrimitiveStream.GetFirstRecord(ctx, types.GetFirstRecordInput{})
//		assertNoErrorOrFail(t, err, "Failed to query first record")
//
//		// Verify the first record's content
//		// This ensures that the inserted data matches what we expect
//		assert.Equal(t, "1.000000000000000000", firstRecord.Value.String(), "Unexpected first record value")
//		assert.Equal(t, "2020-01-01", firstRecord.DateValue.String(), "Unexpected first record date")
//	})
//}
