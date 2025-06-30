package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"

	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

func TestListStreams(t *testing.T) {
	ctx := context.Background()
	fixture := NewServerFixture(t)
	err := fixture.Setup()
	t.Cleanup(func() {
		fixture.Teardown()
	})
	require.NoError(t, err, "Failed to setup server fixture")

	deployerWallet, err := kwilcrypto.Secp256k1PrivateKeyFromHex(AnonWalletPK)
	require.NoError(t, err, "failed to parse anon wallet private key")
	tnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(auth.GetUserSigner(deployerWallet)))
	require.NoError(t, err, "failed to create client")

	authorizeWalletToDeployStreams(t, ctx, fixture, deployerWallet)

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
	streams, err := tnClient.ListStreams(ctx, types.ListStreamsInput{BlockHeight: 0})
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
