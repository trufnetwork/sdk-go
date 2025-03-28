package integration

import (
	"context"
	"fmt"
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

// TestPermissions demonstrates the deployment and permission management of primitive and composed streams in TN.
func TestPermissions(t *testing.T) {
	ctx := context.Background()

	// Set up owner assets
	// The owner is the entity deploying and managing the streams
	ownerPk, err := crypto.Secp256k1PrivateKeyFromHex(TestPrivateKey)
	assertNoErrorOrFail(t, err, "Failed to parse private key")
	streamOwnerSigner := &auth.EthPersonalSigner{Key: *ownerPk}
	streamOwnerAddressStr, _ := auth.EthSecp256k1Authenticator{}.Identifier(streamOwnerSigner.CompactID())
	streamOwnerAddress, err := util.NewEthereumAddressFromString(streamOwnerAddressStr)
	ownerTnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(streamOwnerSigner))
	assertNoErrorOrFail(t, err, "Failed to create client")

	// Set up reader assets
	// The reader represents a separate entity that will attempt to access the streams
	readerPk, err := crypto.Secp256k1PrivateKeyFromHex("2222222222222222222222222222222222222222222222222222222222222222")
	assertNoErrorOrFail(t, err, "Failed to parse private key")
	readerSigner := &auth.EthPersonalSigner{Key: *readerPk}
	readerAddressStr, _ := auth.EthSecp256k1Authenticator{}.Identifier(readerSigner.CompactID())
	readerAddress, err := util.NewEthereumAddressFromString(readerAddressStr)
	assertNoErrorOrFail(t, err, "Failed to create signer address")
	readerTnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(readerSigner))
	assertNoErrorOrFail(t, err, "Failed to create client")

	// Generate unique stream IDs for primitive and composed streams
	primitiveStreamId := util.GenerateStreamId("test-wallet-permission-primitive-stream")
	composedStreamId := util.GenerateStreamId("test-wallet-permission-composed-stream")

	primitiveStreamLocator := ownerTnClient.OwnStreamLocator(primitiveStreamId)
	composedStreamLocator := ownerTnClient.OwnStreamLocator(composedStreamId)

	// Set up cleanup to destroy the primitive stream after test completion
	t.Cleanup(func() {
		destroyResult, err := ownerTnClient.DestroyStream(ctx, primitiveStreamId)
		assertNoErrorOrFail(t, err, "Failed to destroy stream")
		waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, destroyResult)
	})

	// Deploy a primitive stream with initial data
	deployTestPrimitiveStreamWithData(t, ctx, ownerTnClient, []util.StreamId{primitiveStreamId}, []types.InsertRecordInput{
		{
			DataProvider: streamOwnerAddress.Address(),
			StreamId:     primitiveStreamId.String(),
			EventTime:    int(civil.Date{Year: 2020, Month: 1, Day: 1}.In(time.UTC).Unix()),
			Value:        1,
		},
	})

	// Helper function to check if retrieved records match expected values
	var checkRecords = func(t *testing.T, rec []types.StreamRecord) {
		assert.Equal(t, 1, len(rec))
		assert.Equal(t, "1.000000000000000000", rec[0].Value.String())
		assert.Equal(t, int(civil.Date{Year: 2020, Month: 1, Day: 1}.In(time.UTC).Unix()), rec[0].EventTime)
	}

	// Load the primitive stream for both owner and reader
	ownerPrimitiveAction, err := ownerTnClient.LoadPrimitiveActions()
	assertNoErrorOrFail(t, err, "Failed to load stream")
	readerPrimitiveAction, err := readerTnClient.LoadPrimitiveActions()
	assertNoErrorOrFail(t, err, "Failed to load stream")

	// Define input for reading records
	readInput := types.GetRecordInput{
		DataProvider: streamOwnerAddress.Address(),
		StreamId:     primitiveStreamId.String(),
		From: func() *int {
			i := int(civil.Date{Year: 2020, Month: 1, Day: 1}.In(time.UTC).Unix())
			return &i
		}(),
		To: func() *int {
			i := int(civil.Date{Year: 2020, Month: 1, Day: 1}.In(time.UTC).Unix())
			return &i
		}(),
	}

	//ownerReadInput := types.GetRecordInput{}

	// Test primitive stream wallet read permissions
	t.Run("TestPrimitiveStreamWalletReadPermission", func(t *testing.T) {
		t.Cleanup(func() {
			// make these changes not interfere with the next test
			// reset visibility to public
			_, err := ownerPrimitiveAction.SetReadVisibility(ctx, types.VisibilityInput{
				Stream:     primitiveStreamLocator,
				Visibility: util.PublicVisibility,
			})
			assertNoErrorOrFail(t, err, "Failed to set read visibility")
			// remove permissions
			txHash, err := ownerPrimitiveAction.DisableReadWallet(ctx, types.ReadWalletInput{
				Stream: primitiveStreamLocator,
				Wallet: readerAddress,
			})
			// TODO: Not implemented yet
			//assertNoErrorOrFail(t, err, "Failed to disable read wallet")

			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash) // only wait the final tx
		})

		// ok - public read
		rec, err := readerPrimitiveAction.GetRecord(ctx, readInput)
		assertNoErrorOrFail(t, err, "Failed to read records")
		checkRecords(t, rec)

		// set the stream to private
		txHash, err := ownerPrimitiveAction.SetReadVisibility(ctx, types.VisibilityInput{
			Stream:     primitiveStreamLocator,
			Visibility: util.PrivateVisibility,
		})
		assertNoErrorOrFail(t, err, "Failed to set read visibility")
		waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

		// ok - private being owner
		// read the stream
		rec, err = ownerPrimitiveAction.GetRecord(ctx, readInput)
		assertNoErrorOrFail(t, err, "Failed to read records")
		checkRecords(t, rec)

		// fail - private without access
		_, err = readerPrimitiveAction.GetRecord(ctx, readInput)
		assert.Error(t, err)

		// ok - private with access
		// allow read access to the reader
		txHash, err = ownerPrimitiveAction.AllowReadWallet(ctx, types.ReadWalletInput{
			Stream: primitiveStreamLocator,
			Wallet: readerAddress,
		})
		assertNoErrorOrFail(t, err, "Failed to allow read wallet")
		waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

		// read the stream
		rec, err = readerPrimitiveAction.GetRecord(ctx, readInput)
		assertNoErrorOrFail(t, err, "Failed to read records")
		checkRecords(t, rec)
	})

	// Test composed stream functionality and permissions
	t.Run("TestComposedStream", func(t *testing.T) {
		// Set up cleanup to destroy the composed stream after test completion
		t.Cleanup(func() {
			destroyResult, err := ownerTnClient.DestroyStream(ctx, composedStreamId)
			assert.NoError(t, err, "Failed to destroy stream")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, destroyResult)
		})

		// Deploy a composed stream using the primitive stream as a child
		deployTestComposedStreamWithTaxonomy(t, ctx, ownerTnClient, composedStreamId, types.Taxonomy{
			ParentStream: composedStreamLocator,
			TaxonomyItems: []types.TaxonomyItem{
				{
					ChildStream: primitiveStreamLocator,
					Weight:      1,
				},
			},
		})

		// Load the composed stream for both owner and reader
		ownerComposedStream, err := ownerTnClient.LoadComposedActions()
		assertNoErrorOrFail(t, err, "Failed to load stream")
		readerComposedStream, err := readerTnClient.LoadComposedActions()
		assertNoErrorOrFail(t, err, "Failed to load stream")

		// Test wallet read permissions for the composed stream
		t.Run("WalletReadPermission", func(t *testing.T) {
			t.Cleanup(func() {
				// make these changes not interfere with the next test
				// reset visibility to public
				txHash, err := ownerComposedStream.SetReadVisibility(ctx, types.VisibilityInput{
					Stream:     composedStreamLocator,
					Visibility: util.PublicVisibility,
				})
				assert.NoError(t, err, "Failed to set read visibility")

				txHash, err = ownerPrimitiveAction.SetReadVisibility(ctx, types.VisibilityInput{
					Stream:     primitiveStreamLocator,
					Visibility: util.PublicVisibility,
				})
				assert.NoError(t, err, "Failed to set read visibility")

				// remove permissions from the reader
				txHash, err = ownerComposedStream.DisableReadWallet(ctx, types.ReadWalletInput{
					Stream: composedStreamLocator,
					Wallet: readerAddress,
				})
				// TODO: Not implemented yet
				//assert.NoError(t, err, "Failed to disable read wallet")

				txHash, err = ownerPrimitiveAction.DisableReadWallet(ctx, types.ReadWalletInput{
					Stream: primitiveStreamLocator,
					Wallet: readerAddress,
				})
				// TODO: Not implemented yet
				//assert.NoError(t, err, "Failed to disable read wallet")

				waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash) // only wait the final tx
			})

			// ok all public
			rec, err := readerComposedStream.GetRecord(ctx, readInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)

			// set just the composed stream to private
			txHash, err := ownerComposedStream.SetReadVisibility(ctx, types.VisibilityInput{
				Stream:     composedStreamLocator,
				Visibility: util.PrivateVisibility,
			})
			assertNoErrorOrFail(t, err, "Failed to set read visibility")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// fail - composed stream is private without access
			_, err = readerComposedStream.GetRecord(ctx, readInput)
			assert.Error(t, err)

			// set the stream to public
			txHash, err = ownerComposedStream.SetReadVisibility(ctx, types.VisibilityInput{
				Stream:     composedStreamLocator,
				Visibility: util.PublicVisibility,
			})
			assertNoErrorOrFail(t, err, "Failed to set read visibility")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// set the child stream to private
			txHash, err = ownerPrimitiveAction.SetReadVisibility(ctx, types.VisibilityInput{
				Stream:     primitiveStreamLocator,
				Visibility: util.PrivateVisibility,
			})
			assertNoErrorOrFail(t, err, "Failed to set read visibility")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
			fmt.Println("set private")

			// fail - child is private without access
			_, err = readerComposedStream.GetRecord(ctx, readInput)
			assert.Error(t, err)

			// allow read access to the reader
			txHash, err = ownerComposedStream.AllowReadWallet(ctx, types.ReadWalletInput{
				Stream: composedStreamLocator,
				Wallet: readerAddress,
			})
			assertNoErrorOrFail(t, err, "Failed to allow read wallet")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// ok - primitive private but w/ access
			rec, err = readerComposedStream.GetRecord(ctx, readInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)

			// set the composed stream to private
			txHash, err = ownerComposedStream.SetReadVisibility(ctx, types.VisibilityInput{
				Stream:     composedStreamLocator,
				Visibility: util.PrivateVisibility,
			})
			assertNoErrorOrFail(t, err, "Failed to set read visibility")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// allow read access to the reader
			txHash, err = ownerComposedStream.AllowReadWallet(ctx, types.ReadWalletInput{
				Stream: composedStreamLocator,
				Wallet: readerAddress,
			})
			assertNoErrorOrFail(t, err, "Failed to allow read wallet")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// ok - all private but w/ access
			rec, err = readerComposedStream.GetRecord(ctx, readInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)
		})

		// Test stream composition permissions
		t.Run("StreamComposePermission", func(t *testing.T) {
			t.Cleanup(func() {
				// make these changes not interfere with the next test
				// reset visibility to public
				txHash, err := ownerComposedStream.SetComposeVisibility(ctx, types.VisibilityInput{
					Stream:     composedStreamLocator,
					Visibility: util.PublicVisibility,
				})
				assert.NoError(t, err, "Failed to set compose visibility")
				// remove permissions
				txHash, err = ownerPrimitiveAction.DisableComposeStream(ctx, composedStreamLocator)
				// TODO: Not implemented yet
				//assert.NoError(t, err, "Failed to disable compose stream")

				waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash) // only wait the final tx
			})

			// ok - public compose
			rec, err := readerComposedStream.GetRecord(ctx, readInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)

			// set the stream to private
			txHash, err := ownerComposedStream.SetComposeVisibility(ctx, types.VisibilityInput{
				Stream:     composedStreamLocator,
				Visibility: util.PrivateVisibility,
			})
			assertNoErrorOrFail(t, err, "Failed to set compose visibility")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// ok - reading primitive directly
			rec, err = readerPrimitiveAction.GetRecord(ctx, readInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)

			// fail - private without access
			_, err = readerComposedStream.GetRecord(ctx, readInput)
			assert.Error(t, err)

			// ok - private with access
			// allow compose access to the reader
			txHash, err = ownerPrimitiveAction.AllowComposeStream(ctx, composedStreamLocator)
			assertNoErrorOrFail(t, err, "Failed to allow compose stream")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// read the stream
			rec, err = readerComposedStream.GetRecord(ctx, readInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)
		})
	})

}
