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

func TestListAllStreams(t *testing.T) {
	ctx := context.Background()

	// Parse the private key for authentication
	pk, err := crypto.Secp256k1PrivateKeyFromHex(TestPrivateKey)
	assertNoErrorOrFail(t, err, "Failed to parse private key")

	// Create a signer using the parsed private key
	signer := &auth.EthPersonalSigner{Key: *pk}
	tnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(signer))
	assertNoErrorOrFail(t, err, "Failed to create client")

	// Generate unique stream IDs and locators
	primitiveStreamId := util.GenerateStreamId("test-allstreams-primitive-stream")
	composedStreamId := util.GenerateStreamId("test-allstreams-composed-stream")
	//notAStreamName := "not_a_stream"

	// Cleanup function to destroy the streams and contracts after test completion
	t.Cleanup(func() {
		allStreamIds := []util.StreamId{primitiveStreamId, composedStreamId}
		for _, id := range allStreamIds {
			destroyResult, err := tnClient.DestroyStream(ctx, id)
			assertNoErrorOrFail(t, err, "Failed to destroy stream")
			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
		}
	})

	// Deploy a primitive stream
	deployTxHash, err := tnClient.DeployStream(ctx, primitiveStreamId, types.StreamTypePrimitive)
	assertNoErrorOrFail(t, err, "Failed to deploy primitive stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	// Deploy a composed stream
	deployTxHash, err = tnClient.DeployStream(ctx, composedStreamId, types.StreamTypeComposed)
	assertNoErrorOrFail(t, err, "Failed to deploy composed stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	// Deploy a non-stream contract
	//notAStreamSchema, err := parse.Parse(assets.NotAStreamContent)
	//notAStreamSchema.Name = notAStreamName
	//assertNoErrorOrFail(t, err, "Failed to parse non-stream contract content")
	//
	//// Cleanup function to destroy the non-stream contract after test completion
	//t.Cleanup(func() {
	//	_, err := tnClient.GetKwilClient().DropDatabase(ctx, notAStreamName, client.WithSyncBroadcast(true))
	//	assertNoErrorOrFail(t, err, "Failed to destroy non-stream contract")
	//})
	//
	//_, err = tnClient.GetKwilClient().DeployDatabase(ctx, notAStreamSchema, client.WithSyncBroadcast(true))
	//assertNoErrorOrFail(t, err, "Failed to deploy non-stream contract")
	//
	//// List all streams
	//streams, err := tnClient.GetAllStreams(ctx, types.GetAllStreamsInput{})
	//assertNoErrorOrFail(t, err, "Failed to list all streams")
	//
	//// Check that only the primitive and composed streams are listed
	//expectedStreamIds := map[util.StreamId]bool{
	//	primitiveStreamId: true,
	//	composedStreamId:  true,
	//}
	//
	//for _, stream := range streams {
	//	// this will only be true if the database is clean from start
	//	//assert.True(t, expectedStreamIds[stream.StreamId], "Unexpected stream listed: %s", stream.StreamId)
	//	delete(expectedStreamIds, stream.StreamId)
	//}
	//
	//// Ensure all expected streams were found
	//assert.Empty(t, expectedStreamIds, "Not all expected streams were listed")
	//
	//// Check non-initalized stream
	//initializedStreams, err := tnClient.GetAllInitializedStreams(ctx, types.GetAllStreamsInput{})
	//assertNoErrorOrFail(t, err, "Failed to list all streams")
	//
	//// Count the number of initialized streams before the test
	//initialInitializedStreamsCount := len(initializedStreams)
	//
	//// initialize the stream primitiveStreamId
	//primitiveStream, err := tnClient.LoadStream(types.StreamLocator{
	//	StreamId:     primitiveStreamId,
	//	DataProvider: tnClient.Address(),
	//})
	//assertNoErrorOrFail(t, err, "Failed to load primitive stream")
	//txHash, err := primitiveStream.InitializeStream(ctx)
	//assertNoErrorOrFail(t, err, "Failed to initialize primitive stream")
	//waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
	//
	//// initialize the stream composedStreamId
	//composedStream, err := tnClient.LoadStream(types.StreamLocator{
	//	StreamId:     composedStreamId,
	//	DataProvider: tnClient.Address(),
	//})
	//assertNoErrorOrFail(t, err, "Failed to load composed stream")
	//
	//txHash, err = composedStream.InitializeStream(ctx)
	//assertNoErrorOrFail(t, err, "Failed to initialize composed stream")
	//waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHash)
	//
	//// Check initialized stream
	//initializedStreams, err = tnClient.GetAllInitializedStreams(ctx, types.GetAllStreamsInput{})
	//assertNoErrorOrFail(t, err, "Failed to list all streams")
	//assert.Equal(t, initialInitializedStreamsCount+2, len(initializedStreams), "It should be 2 as 2 streams are initialized")
}
