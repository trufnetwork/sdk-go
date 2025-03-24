package integration

//
//import (
//	"context"
//	"testing"
//
//	"github.com/kwilteam/kwil-db/core/crypto"
//	"github.com/kwilteam/kwil-db/core/crypto/auth"
//	"github.com/stretchr/testify/assert"
//	"github.com/trufnetwork/sdk-go/core/tnclient"
//	"github.com/trufnetwork/sdk-go/core/types"
//	"github.com/trufnetwork/sdk-go/core/util"
//)
//
//// TestHelperStream demonstrates the process of deploying, initializing, writing to,
//// and reading from a primitive stream in TN using the TN SDK with the helper functions.
//func TestHelperStream(t *testing.T) {
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
//	streamId := util.GenerateStreamId("test-primitive-stream-unix")
//	streamLocator := tnClient.OwnStreamLocator(streamId)
//
//	streamId2 := util.GenerateStreamId("test-primitive-stream-unix2")
//	streamLocator2 := tnClient.OwnStreamLocator(streamId2)
//
//	helperStreamId := util.GenerateStreamId("helper_contract")
//	helperStreamLocator := tnClient.OwnStreamLocator(helperStreamId)
//
//	// Set up cleanup to destroy the stream after test completion
//	// This ensures that test streams don't persist in the network
//	t.Cleanup(func() {
//		destroyResult, err := tnClient.DestroyStream(ctx, streamId)
//		assertNoErrorOrFail(t, err, "Failed to destroy stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//
//		destroyResult2, err := tnClient.DestroyStream(ctx, streamId2)
//		assertNoErrorOrFail(t, err, "Failed to destroy stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult2)
//
//		destroyResult3, err := tnClient.DestroyStream(ctx, helperStreamId)
//		assertNoErrorOrFail(t, err, "Failed to destroy stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult3)
//	})
//
//	// Subtest for deploying, initializing, writing to, and reading from a primitive stream with helper functions
//	t.Run("DeploymentWriteAndReadOperationsWithHelper", func(t *testing.T) {
//		// Deploy a primitive stream
//		// This creates the stream contract on the TN
//		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitiveUnix)
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
//		// stream 2
//		deployTxHash2, err := tnClient.DeployStream(ctx, streamId2, types.StreamTypePrimitiveUnix)
//		assertNoErrorOrFail(t, err, "Failed to deploy stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash2)
//
//		deployedPrimitiveStream2, err := tnClient.LoadPrimitiveStream(streamLocator2)
//		assertNoErrorOrFail(t, err, "Failed to load stream")
//
//		txHashInit2, err := deployedPrimitiveStream2.InitializeStream(ctx)
//		assertNoErrorOrFail(t, err, "Failed to initialize stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInit2)
//
//		// Insert a record into the stream using the helper function
//		// This demonstrates how to write data to the stream
//		//txHash, err := deployedPrimitiveStream.InsertRecordsUnix(ctx, []types.InsertRecordUnixInput{
//		//	{
//		//		Value:     1,
//		//		DateValue: 1,
//		//	},
//		//})
//		//assertNoErrorOrFail(t, err, "Failed to insert record")
//		//waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
//
//		// Deploy the helper stream
//		txHashDeployHelper, err := tnClient.DeployStream(ctx, helperStreamId, types.StreamTypeHelper)
//		assertNoErrorOrFail(t, err, "Failed to deploy helper stream")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashDeployHelper)
//
//		// Load the helper stream
//		deployedHelperStream, err := tnClient.LoadHelperStream(helperStreamLocator)
//		assertNoErrorOrFail(t, err, "Failed to load helper stream")
//
//		// Insert a record into the helper stream using the helper function
//		dataProvider := tnClient.Address()
//		txHashInsertHelper, err := deployedHelperStream.InsertRecordsUnix(ctx, types.TnRecordUnixBatch{
//			Rows: []types.TNRecordUnixRow{
//				{
//					DateValue:    "1",
//					Value:        "1",
//					StreamID:     streamId.String(),
//					DataProvider: dataProvider.Address(),
//				},
//				{
//					DateValue:    "1",
//					Value:        "2",
//					StreamID:     streamId2.String(),
//					DataProvider: dataProvider.Address(),
//				},
//			},
//		})
//		assertNoErrorOrFail(t, err, "Failed to insert record")
//		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInsertHelper)
//
//		// Query records from the stream
//		// This demonstrates how to read data from the stream
//		mockedDateFromUnix := 1
//		mockedDateToUnix := 1
//		records, err := deployedPrimitiveStream.GetRecordUnix(ctx, types.GetRecordUnixInput{
//			DateFrom: &mockedDateFromUnix,
//			DateTo:   &mockedDateToUnix,
//		})
//		assertNoErrorOrFail(t, err, "Failed to query records")
//
//		// Verify the record's content
//		// This ensures that the inserted data matches what we expect
//		assert.Len(t, records, 1, "Expected exactly one record")
//		assert.Equal(t, "1.000000000000000000", records[0].Value.String(), "Unexpected record value")
//		assert.Equal(t, 1, records[0].DateValue, "Unexpected record date")
//
//		records2, err := deployedPrimitiveStream2.GetRecordUnix(ctx, types.GetRecordUnixInput{
//			DateFrom: &mockedDateFromUnix,
//			DateTo:   &mockedDateToUnix,
//		})
//		assertNoErrorOrFail(t, err, "Failed to query records")
//
//		assert.Len(t, records2, 1, "Expected exactly one record")
//		assert.Equal(t, "2.000000000000000000", records2[0].Value.String(), "Unexpected record value")
//		assert.Equal(t, 1, records2[0].DateValue, "Unexpected record date")
//
//		// Query index from the stream
//		index, err := deployedPrimitiveStream.GetIndexUnix(ctx, types.GetIndexUnixInput{
//			DateFrom: &mockedDateFromUnix,
//			DateTo:   &mockedDateToUnix,
//		})
//		assertNoErrorOrFail(t, err, "Failed to query index")
//
//		// Verify the index's content
//		// This ensures that the inserted data matches what we expect
//		assert.Len(t, index, 1, "Expected exactly one index")
//		assert.Equal(t, "100.000000000000000000", index[0].Value.String(), "Unexpected index value")
//		assert.Equal(t, 1, index[0].DateValue, "Unexpected index date")
//
//		index2, err := deployedPrimitiveStream2.GetIndexUnix(ctx, types.GetIndexUnixInput{
//			DateFrom: &mockedDateFromUnix,
//			DateTo:   &mockedDateToUnix,
//		})
//
//		assertNoErrorOrFail(t, err, "Failed to query index")
//		assert.Len(t, index2, 1, "Expected exactly one index")
//		assert.Equal(t, "100.000000000000000000", index2[0].Value.String(), "Unexpected index value")
//		assert.Equal(t, 1, index2[0].DateValue, "Unexpected index date")
//	})
//}
//
//// TestFilterInitializedStreams tests the filter_initialized function of the helper contract
//func TestFilterInitializedStreams(t *testing.T) {
//	// Create a server fixture for the test
//	fixture := NewServerFixture(t)
//	err := fixture.Setup()
//	if err != nil {
//		t.Fatalf("Failed to set up server fixture: %v", err)
//	}
//	defer fixture.Teardown()
//
//	// Get the client from the fixture
//	tnClient := fixture.Client()
//	ctx := context.Background()
//
//	// Generate unique stream IDs for the test
//	// We'll create 3 streams: 2 initialized and 1 not initialized
//	firstStreamId := util.GenerateStreamId("test-stream-1")
//	firstStreamLocator := tnClient.OwnStreamLocator(firstStreamId)
//
//	secondStreamId := util.GenerateStreamId("test-stream-2")
//	secondStreamLocator := tnClient.OwnStreamLocator(secondStreamId)
//
//	thirdStreamId := util.GenerateStreamId("test-stream-3")
//	thirdStreamLocator := tnClient.OwnStreamLocator(thirdStreamId)
//
//	helperStreamId := util.GenerateStreamId("helper_contract")
//	helperStreamLocator := tnClient.OwnStreamLocator(helperStreamId)
//
//	// Set up cleanup to destroy the streams after test completion
//	t.Cleanup(func() {
//		// Try to destroy all streams, even if some operations fail
//		destroyResult, err := tnClient.DestroyStream(ctx, firstStreamId)
//		if err == nil {
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//		}
//
//		destroyResult, err = tnClient.DestroyStream(ctx, secondStreamId)
//		if err == nil {
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//		}
//
//		destroyResult, err = tnClient.DestroyStream(ctx, thirdStreamId)
//		if err == nil {
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//		}
//
//		destroyResult, err = tnClient.DestroyStream(ctx, helperStreamId)
//		if err == nil {
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//		}
//	})
//
//	// Deploy the first stream
//	deployTxHash, err := tnClient.DeployStream(ctx, firstStreamId, types.StreamTypePrimitiveUnix)
//	assertNoErrorOrFail(t, err, "Failed to deploy first stream")
//	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
//
//	// Load and initialize the first stream
//	firstStream, err := tnClient.LoadPrimitiveStream(firstStreamLocator)
//	assertNoErrorOrFail(t, err, "Failed to load first stream")
//	txHash, err := firstStream.InitializeStream(ctx)
//	assertNoErrorOrFail(t, err, "Failed to initialize first stream")
//	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
//
//	// Deploy the second stream
//	deployTxHash, err = tnClient.DeployStream(ctx, secondStreamId, types.StreamTypePrimitiveUnix)
//	assertNoErrorOrFail(t, err, "Failed to deploy second stream")
//	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
//
//	// Load and initialize the second stream
//	secondStream, err := tnClient.LoadPrimitiveStream(secondStreamLocator)
//	assertNoErrorOrFail(t, err, "Failed to load second stream")
//	txHash, err = secondStream.InitializeStream(ctx)
//	assertNoErrorOrFail(t, err, "Failed to initialize second stream")
//	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
//
//	// Deploy the third stream but DO NOT initialize it
//	deployTxHash, err = tnClient.DeployStream(ctx, thirdStreamId, types.StreamTypePrimitiveUnix)
//	assertNoErrorOrFail(t, err, "Failed to deploy third stream")
//	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
//
//	// Load the third stream but don't initialize it
//	_, err = tnClient.LoadPrimitiveStream(thirdStreamLocator)
//	assertNoErrorOrFail(t, err, "Failed to load third stream")
//	// Intentionally not initializing the third stream
//
//	// Deploy the helper stream
//	deployTxHash, err = tnClient.DeployStream(ctx, helperStreamId, types.StreamTypeHelper)
//	assertNoErrorOrFail(t, err, "Failed to deploy helper stream")
//	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
//
//	// Load the helper stream
//	helperStream, err := tnClient.LoadHelperStream(helperStreamLocator)
//	assertNoErrorOrFail(t, err, "Failed to load helper stream")
//
//	// Get the data provider address (signer's address)
//	address := tnClient.Address()
//	dataProvider := address.Address()
//
//	// Call filter_initialized on all three streams
//	// We expect only the first two to be returned since the third isn't initialized
//	results, err := helperStream.FilterInitialized(ctx, types.FilterInitializedInput{
//		DataProviders: []string{dataProvider, dataProvider, dataProvider},
//		StreamIDs:     []string{firstStreamId.String(), secondStreamId.String(), thirdStreamId.String()},
//	})
//	assertNoErrorOrFail(t, err, "Failed to filter initialized streams")
//
//	// Verify results
//	assert.Len(t, results, 2, "Expected exactly two initialized streams")
//
//	// Create a map of returned streams for easier verification
//	returnedStreams := make(map[string]bool)
//	for _, result := range results {
//		returnedStreams[result.StreamID] = true
//		assert.Equal(t, dataProvider, result.DataProvider, "Unexpected data provider")
//	}
//
//	// Verify that only the first two streams were returned
//	assert.True(t, returnedStreams[firstStreamId.String()], "First stream should be in results")
//	assert.True(t, returnedStreams[secondStreamId.String()], "Second stream should be in results")
//	assert.False(t, returnedStreams[thirdStreamId.String()], "Third stream should not be in results")
//}
//
//// TestFilterInitializedWithNonExistentStream tests the filter_initialized function with a non-existent stream
//func TestFilterInitializedWithNonExistentStream(t *testing.T) {
//	// Skip this test with an explanation
//	t.Skip("LIMITATION: The filter_initialized procedure in the helper contract cannot handle non-existent streams. " +
//		"When attempting to filter with a non-existent stream, the procedure will fail because ext_get_metadata " +
//		"cannot be called on a non-existent stream. This is an inherent limitation of the current contract design " +
//		"which processes all streams in a batch operation. Individual stream processing would require modifying " +
//		"the contract implementation or handling the errors at the application level.")
//
//	// Create a server fixture
//	fixture := NewServerFixture(t)
//	defer fixture.Teardown()
//
//	// Get the client from the fixture
//	tnClient := fixture.Client()
//	ctx := context.Background()
//
//	// Generate unique stream IDs for the test
//	// We'll create 1 initialized stream and reference 1 non-existent stream
//	existingStreamId := util.GenerateStreamId("test-existing-stream")
//	existingStreamLocator := tnClient.OwnStreamLocator(existingStreamId)
//
//	// This stream ID is never deployed, so it doesn't exist
//	nonExistentStreamId := util.GenerateStreamId("test-non-existent-stream")
//
//	helperStreamId := util.GenerateStreamId("helper_contract")
//	helperStreamLocator := tnClient.OwnStreamLocator(helperStreamId)
//
//	// Set up cleanup to destroy the streams after test completion
//	t.Cleanup(func() {
//		// Try to destroy all streams, even if some operations fail
//		destroyResult, err := tnClient.DestroyStream(ctx, existingStreamId)
//		if err == nil {
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//		}
//
//		destroyResult, err = tnClient.DestroyStream(ctx, helperStreamId)
//		if err == nil {
//			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
//		}
//	})
//
//	// Deploy the existing stream
//	deployTxHash, err := tnClient.DeployStream(ctx, existingStreamId, types.StreamTypePrimitiveUnix)
//	assertNoErrorOrFail(t, err, "Failed to deploy existing stream")
//	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
//
//	// Load and initialize the existing stream
//	existingStream, err := tnClient.LoadPrimitiveStream(existingStreamLocator)
//	assertNoErrorOrFail(t, err, "Failed to load existing stream")
//	txHash, err := existingStream.InitializeStream(ctx)
//	assertNoErrorOrFail(t, err, "Failed to initialize existing stream")
//	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
//
//	// Deploy the helper stream
//	deployTxHash, err = tnClient.DeployStream(ctx, helperStreamId, types.StreamTypeHelper)
//	assertNoErrorOrFail(t, err, "Failed to deploy helper stream")
//	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)
//
//	// Load the helper stream
//	helperStream, err := tnClient.LoadHelperStream(helperStreamLocator)
//	assertNoErrorOrFail(t, err, "Failed to load helper stream")
//
//	// Get the data provider address (signer's address)
//	address := tnClient.Address()
//	dataProvider := address.Address()
//
//	// Call filter_initialized with both the existing and non-existent streams
//	// We expect an error because the non-existent stream will cause the procedure to fail
//	_, err = helperStream.FilterInitialized(ctx, types.FilterInitializedInput{
//		DataProviders: []string{dataProvider, dataProvider},
//		StreamIDs:     []string{existingStreamId.String(), nonExistentStreamId.String()},
//	})
//
//	// Verify that we got an error
//	assert.Error(t, err, "Expected an error when filtering with a non-existent stream")
//	assert.Contains(t, err.Error(), "Procedure \"get_metadata\" not found", "Error should mention that the procedure was not found")
//
//	// Now test with only the existing stream to make sure that works
//	results, err := helperStream.FilterInitialized(ctx, types.FilterInitializedInput{
//		DataProviders: []string{dataProvider},
//		StreamIDs:     []string{existingStreamId.String()},
//	})
//	assertNoErrorOrFail(t, err, "Failed to filter initialized streams with only existing stream")
//
//	// Verify results
//	assert.Len(t, results, 1, "Expected exactly one initialized stream")
//	assert.Equal(t, existingStreamId.String(), results[0].StreamID, "Expected only the existing stream to be returned")
//	assert.Equal(t, dataProvider, results[0].DataProvider, "Unexpected data provider")
//}
