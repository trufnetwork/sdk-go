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

// This file contains integration tests for composed streams in the Truf Network (TN).
// It demonstrates the process of deploying, initializing, and querying a composed stream
// that aggregates data from multiple primitive streams.

// TestComposedStream demonstrates the process of deploying, initializing, and querying
// a composed stream that aggregates data from multiple primitive streams in the TN using the TN SDK.
func TestComposedActions(t *testing.T) {
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

	signerAddress := tnClient.Address()

	// Generate a unique stream ID and locator for the composed stream and its child streams
	streamId := util.GenerateStreamId("test-composed-stream-unix")

	childAStreamId := util.GenerateStreamId("test-composed-stream-child-a-unix")
	childBStreamId := util.GenerateStreamId("test-composed-stream-child-b-unix")

	allStreamIds := []util.StreamId{streamId, childAStreamId, childBStreamId}
	primitiveStreamIds := []util.StreamId{childAStreamId, childBStreamId}

	// Cleanup function to destroy the streams after test completion
	t.Cleanup(func() {
		//return
		for _, id := range allStreamIds {
			destroyResult, err := tnClient.DestroyStream(ctx, id)
			assertNoErrorOrFail(t, err, "Failed to destroy stream")
			waitTxToBeMinedWithSuccess(t, ctx, tnClient, destroyResult)
		}
	})

	// Subtest for deploying, initializing, and querying the composed stream
	t.Run("DeploymentAndReadOperations", func(t *testing.T) {
		// Step 1: Deploy the composed stream
		// This creates the composed stream contract on the TN
		deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypeComposed)
		assertNoErrorOrFail(t, err, "Failed to deploy composed stream")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

		// Load the deployed composed stream
		deployedComposedStream, err := tnClient.LoadComposedActions()
		assertNoErrorOrFail(t, err, "Failed to load composed stream")

		// Check Composed Validity
		err = deployedComposedStream.CheckValidComposedStream(ctx, tnClient.OwnStreamLocator(streamId))
		assertNoErrorOrFail(t, err, "Failed to check composed stream validity")

		// Get Type of the stream
		streamType, err := deployedComposedStream.GetType(ctx, tnClient.OwnStreamLocator(streamId))
		assertNoErrorOrFail(t, err, "Failed to get stream type")
		assert.Equal(t, types.StreamTypeComposed, streamType, "Expected stream type to be composed")

		// Step 2: Deploy child streams with initial data
		// Deploy two primitive child streams with initial data
		// | date       | childA | childB |
		// |------------|--------|--------|
		// | 2020-01-01 | 1      | 3      |
		// | 2020-01-02 | 2      | 4      |

		deployTestPrimitiveStreamWithData(t, ctx, tnClient, primitiveStreamIds, []types.InsertRecordInput{
			// Child A
			{DataProvider: signerAddress.Address(), StreamId: childAStreamId.String(), Value: 2, EventTime: 2},
			{DataProvider: signerAddress.Address(), StreamId: childAStreamId.String(), Value: 3, EventTime: 3},
			{DataProvider: signerAddress.Address(), StreamId: childAStreamId.String(), Value: 4, EventTime: 4},
			{DataProvider: signerAddress.Address(), StreamId: childAStreamId.String(), Value: 5, EventTime: 5},

			// Child B
			{DataProvider: signerAddress.Address(), StreamId: childBStreamId.String(), Value: 3, EventTime: 1},
			{DataProvider: signerAddress.Address(), StreamId: childBStreamId.String(), Value: 4, EventTime: 2},
			{DataProvider: signerAddress.Address(), StreamId: childBStreamId.String(), Value: 5, EventTime: 3},
			{DataProvider: signerAddress.Address(), StreamId: childBStreamId.String(), Value: 6, EventTime: 4},
			{DataProvider: signerAddress.Address(), StreamId: childBStreamId.String(), Value: 7, EventTime: 5},
		})

		// Step 3: Set taxonomies for the composed stream
		// Taxonomies define the structure of the composed stream
		mockStartDate := 3
		txHashTaxonomies, err := deployedComposedStream.InsertTaxonomy(ctx, types.Taxonomy{
			ParentStream: types.StreamLocator{
				StreamId:     streamId,
				DataProvider: signerAddress,
			},
			TaxonomyItems: []types.TaxonomyItem{
				{
					ChildStream: types.StreamLocator{
						StreamId:     childAStreamId,
						DataProvider: signerAddress,
					},
					Weight: 1,
				},
				{
					ChildStream: types.StreamLocator{
						StreamId:     childBStreamId,
						DataProvider: signerAddress,
					},
					Weight: 2,
				}},
			StartDate: &mockStartDate,
		})
		assertNoErrorOrFail(t, err, "Failed to set taxonomies")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashTaxonomies)

		// Describe the taxonomies of the composed stream
		taxonomies, err := deployedComposedStream.DescribeTaxonomies(ctx, types.DescribeTaxonomiesParams{
			Stream:        types.StreamLocator{StreamId: streamId, DataProvider: signerAddress},
			LatestVersion: true,
		})
		assertNoErrorOrFail(t, err, "Failed to describe taxonomies")
		assert.Equal(t, 2, len(taxonomies.TaxonomyItems))
		assert.Equal(t, 3, *taxonomies.StartDate)
		assert.Equal(t, 1, taxonomies.GroupSequence)
		assert.True(t, taxonomies.CreatedAt > 0)
		assert.Equal(t, streamId.String(), taxonomies.ParentStream.StreamId.String())
		assert.Equal(t, signerAddress.Address(), taxonomies.ParentStream.DataProvider.Address())
		assert.Equal(t, childAStreamId.String(), taxonomies.TaxonomyItems[0].ChildStream.StreamId.String())
		assert.Equal(t, signerAddress.Address(), taxonomies.TaxonomyItems[0].ChildStream.DataProvider.Address())
		assert.Equal(t, 1.0, taxonomies.TaxonomyItems[0].Weight)
		assert.Equal(t, mockStartDate, *taxonomies.StartDate)

		// Step 4: Query the composed stream for records
		// Query records within a specific date range
		mockDateFrom := 4
		mockDateTo := 5
		result, err := deployedComposedStream.GetRecord(ctx, types.GetRecordInput{
			DataProvider: signerAddress.Address(),
			StreamId:     streamId.String(),
			From:         &mockDateFrom,
			To:           &mockDateTo,
		})

		assertNoErrorOrFail(t, err, "Failed to get records")
		assert.Equal(t, 2, len(result.Results))

		// Query the records before the set start date
		mockDateFrom2 := 1
		mockDateTo2 := 2
		resultBefore, errBefore := deployedComposedStream.GetRecord(ctx, types.GetRecordInput{
			DataProvider: signerAddress.Address(),
			StreamId:     streamId.String(),
			From:         &mockDateFrom2,
			To:           &mockDateTo2,
		})
		assert.NoError(t, errBefore, "Expected no error when querying records before start date")
		assert.Nil(t, resultBefore.Results, "Results before start date should not be nil as there is no active record before the start date")

		// Function to check the record values
		var checkRecord = func(record types.StreamResult, expectedValue float64) {
			val, err := record.Value.Float64()
			assertNoErrorOrFail(t, err, "Failed to parse value")
			assert.Equal(t, expectedValue, val)
		}

		// Verify the record values
		// (( v1 * w1 ) + ( v2 * w2 )) / (w1 + w2)
		// (( 4 *  1 ) + (  6 *  2 )) / ( 1 +  2) = 16 / 3 = 5.333
		// (( 5 *  1 ) + (  7 *  2 )) / ( 1 +  2) = 19 / 3 = 6.333
		checkRecord(result.Results[0], 5.333333333333333)
		checkRecord(result.Results[1], 6.333333333333333)

		// Step 5: Query the composed stream for index
		// Query the index within a specific date range
		mockDateFrom3 := 3
		mockDateTo3 := 4
		mockBaseDate := 3
		indexResult, err := deployedComposedStream.GetIndex(ctx, types.GetIndexInput{
			DataProvider: signerAddress.Address(),
			StreamId:     streamId.String(),
			From:         &mockDateFrom3,
			To:           &mockDateTo3,
			BaseDate:     &mockBaseDate,
		})

		assertNoErrorOrFail(t, err, "Failed to get index")
		assert.Equal(t, 2, len(indexResult.Results))
		checkRecord(indexResult.Results[0], 100)                // index on base date is expected to be 100
		checkRecord(indexResult.Results[1], 124.44444444444444) // it is x% away from the base date + 1 in percentage

		// Query the index before the set start date
		mockDateFrom4 := 1
		mockDateTo4 := 2
		indexBeforeResult, errBefore := deployedComposedStream.GetIndex(ctx, types.GetIndexInput{
			DataProvider: signerAddress.Address(),
			StreamId:     streamId.String(),
			From:         &mockDateFrom4,
			To:           &mockDateTo4,
		})
		assert.NoError(t, errBefore, "Expected no error when querying index before start date")
		assert.Nil(t, indexBeforeResult.Results, "Index before start date should not be nil as there is no active index before the start date")
	})
}
