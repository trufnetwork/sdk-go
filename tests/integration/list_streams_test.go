package integration

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"

	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

func TestListStreams(t *testing.T) {
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

	//// List all streams
	streams, err := tnClient.ListStreams(ctx, types.ListStreamsInput{})
	assertNoErrorOrFail(t, err, "Failed to list all streams")

	// Check that only the primitive and composed streams are listed
	expectedStreamIds := map[string]bool{
		primitiveStreamId.String(): true,
		composedStreamId.String():  true,
	}

	for _, stream := range streams {
		// this will only be true if the database is clean from start
		//assert.True(t, expectedStreamIds[stream.StreamId], "Unexpected stream listed: %s", stream.StreamId)
		delete(expectedStreamIds, stream.StreamId)
	}

	// Ensure all expected streams were found
	assert.Empty(t, expectedStreamIds, "Not all expected streams were listed")
}
