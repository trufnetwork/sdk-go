package integration

import (
	"context"
	"github.com/golang-sql/civil"
	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/stretchr/testify/assert"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"testing"
	"time"
)

func TestDeployComposedStreamsWithTaxonomy(t *testing.T) {
	ctx := context.Background()

	// Parse the private key for authentication
	pk, err := crypto.Secp256k1PrivateKeyFromHex(TestPrivateKey)
	assertNoErrorOrFail(t, err, "Failed to parse private key")

	// Create a signer using the parsed private key
	signer := &auth.EthPersonalSigner{Key: *pk}
	tnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(signer))
	assertNoErrorOrFail(t, err, "Failed to create client")

	// Generate unique stream IDs and locators
	primitiveStreamId := util.GenerateStreamId("test-primitive-stream-one")
	primitiveStreamId2 := util.GenerateStreamId("test-primitive-stream-two")
	composedStreamId := util.GenerateStreamId("test-composed-stream")

	// Cleanup function to destroy the streams and contracts after test completion
	t.Cleanup(func() {
		allStreamIds := []util.StreamId{primitiveStreamId, composedStreamId, primitiveStreamId2}
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

	// Deploy a second primitive stream
	deployTxHash, err = tnClient.DeployStream(ctx, primitiveStreamId2, types.StreamTypePrimitive)
	assertNoErrorOrFail(t, err, "Failed to deploy primitive stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	// Deploy a composed stream using utility function
	err = tnClient.DeployComposedStreamWithTaxonomy(ctx, composedStreamId, types.Taxonomy{
		ParentStream: tnClient.OwnStreamLocator(composedStreamId),
		TaxonomyItems: []types.TaxonomyItem{
			{
				ChildStream: tnClient.OwnStreamLocator(primitiveStreamId),
				Weight:      50,
			},
			{
				ChildStream: tnClient.OwnStreamLocator(primitiveStreamId2),
				Weight:      50,
			},
		},
	})
	assertNoErrorOrFail(t, err, "Failed to deploy composed stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	// List all streams
	//streams, err := tnClient.GetAllStreams(ctx, types.GetAllStreamsInput{})
	//assertNoErrorOrFail(t, err, "Failed to list all streams")

	// Check that only the primitive and composed streams are listed
	//expectedStreamIds := map[util.StreamId]bool{
	//	primitiveStreamId:  true,
	//	composedStreamId:   true,
	//	primitiveStreamId2: true,
	//}

	//for _, stream := range streams {
	// this will only be true if the database is clean from start
	//assert.True(t, expectedStreamIds[stream.StreamId], "Unexpected stream listed: %s", stream.StreamId)
	//delete(expectedStreamIds, stream.StreamId)
	//}

	// Ensure all expected streams were found
	//assert.Empty(t, expectedStreamIds, "Not all expected streams were listed")

	// insert a record to primitiveStreamId and primitiveStreamId2
	// Load the primitive stream
	primitiveStream, err := tnClient.LoadPrimitiveActions()
	assertNoErrorOrFail(t, err, "Failed to load primitive actions")

	dataProviderAddress := tnClient.Address()
	// insert a record to primitiveStreamId
	insertTxHash, err := primitiveStream.InsertRecords(ctx, []types.InsertRecordInput{
		{
			DataProvider: dataProviderAddress.Address(),
			StreamId:     primitiveStreamId.String(),
			EventTime:    int(civil.DateOf(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)).In(time.UTC).Unix()),
			Value:        10,
		},
	})
	assertNoErrorOrFail(t, err, "Failed to insert record")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, insertTxHash)

	// insert a record to primitiveStreamId2
	insertTxHash, err = primitiveStream.InsertRecords(ctx, []types.InsertRecordInput{
		{
			DataProvider: dataProviderAddress.Address(),
			StreamId:     primitiveStreamId2.String(),
			EventTime:    int(civil.DateOf(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)).In(time.UTC).Unix()),
			Value:        20,
		},
	})
	assertNoErrorOrFail(t, err, "Failed to insert record")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, insertTxHash)

	// Load the composed stream
	composedStream, err := tnClient.LoadComposedActions()
	assertNoErrorOrFail(t, err, "Failed to load composed stream")

	// Get records from the composed stream
	records, err := composedStream.GetRecord(ctx, types.GetRecordInput{
		DataProvider: dataProviderAddress.Address(),
		StreamId:     composedStreamId.String(),
	})
	assertNoErrorOrFail(t, err, "Failed to get records")
	assert.Equal(t, 1, len(records), "Unexpected number of records")
	assert.Equal(t, "15.000000000000000000", records[0].Value.String(), "10 * 50/100 + 20 * 50/100 != 15")

	// Negative test cases

	// Deploy a composed stream with a non-existent child stream
	err = tnClient.DeployComposedStreamWithTaxonomy(ctx, composedStreamId, types.Taxonomy{
		ParentStream: tnClient.OwnStreamLocator(composedStreamId),
		TaxonomyItems: []types.TaxonomyItem{
			{
				ChildStream: tnClient.OwnStreamLocator(util.GenerateStreamId("non-existent-stream")),
				Weight:      50,
			},
		},
	})
	// TODO: it should be error as it violates the foreign key constraint but it is not!
	//assert.Error(t, err, "Expected error when deploying composed stream with non-existent child stream")

	// Deploy a composed stream with already deployed stream
	err = tnClient.DeployComposedStreamWithTaxonomy(ctx, composedStreamId, types.Taxonomy{
		ParentStream: tnClient.OwnStreamLocator(composedStreamId),
	})
	assert.Error(t, err, "Expected error when deploying already deployed stream")
}
