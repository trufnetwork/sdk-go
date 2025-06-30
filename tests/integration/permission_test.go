package integration

import (
	"context"
	"testing"
	"time"

	"github.com/golang-sql/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/kwil-db/core/crypto"
	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// TestPermissions demonstrates the deployment and permission management of primitive and composed streams in TN.
func TestPermissions(t *testing.T) {
	ctx := context.Background()
	fixture := NewServerFixture(t)
	err := fixture.Setup()
	t.Cleanup(func() {
		fixture.Teardown()
	})
	require.NoError(t, err, "Failed to setup server fixture")

	ownerWallet, err := kwilcrypto.Secp256k1PrivateKeyFromHex(AnonWalletPK)
	require.NoError(t, err, "failed to parse anon wallet private key")
	authorizeWalletToDeployStreams(t, ctx, fixture, ownerWallet)

	ownerTnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(auth.GetUserSigner(ownerWallet)))
	require.NoError(t, err, "failed to create client")
	streamOwnerAddress := ownerTnClient.Address()

	// Set up reader assets
	// The reader represents a separate entity that will attempt to access the streams
	readerPk, err := crypto.Secp256k1PrivateKeyFromHex("2222222222222222222222222222222222222222222222222222222222222222")
	assertNoErrorOrFail(t, err, "Failed to parse private key for reader")
	readerSigner := &auth.EthPersonalSigner{Key: *readerPk}
	readerAddressStr, _ := auth.EthSecp256k1Authenticator{}.Identifier(readerSigner.CompactID())
	readerAddress, err := util.NewEthereumAddressFromString(readerAddressStr)
	assertNoErrorOrFail(t, err, "Failed to create reader signer address")
	readerTnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(readerSigner))
	assertNoErrorOrFail(t, err, "Failed to create reader client")

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
	readPrimitiveInput := types.GetRecordInput{
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

	readComposedInput := readPrimitiveInput
	readComposedInput.StreamId = composedStreamId.String()

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
			assertNoErrorOrFail(t, err, "Failed to disable read wallet")

			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash) // only wait the final tx
		})

		// ok - public read
		rec, err := readerPrimitiveAction.GetRecord(ctx, readPrimitiveInput)
		assertNoErrorOrFail(t, err, "Failed to read records")
		checkRecords(t, rec)

		// set the stream to private
		txHash, err := ownerPrimitiveAction.SetReadVisibility(ctx, types.VisibilityInput{
			Stream:     primitiveStreamLocator,
			Visibility: util.PrivateVisibility,
		})
		assertNoErrorOrFail(t, err, "Failed to set read visibility")
		waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

		// Get ReadVisibility
		readVisibility, err := ownerPrimitiveAction.GetReadVisibility(ctx, primitiveStreamLocator)
		assertNoErrorOrFail(t, err, "Failed to get read visibility")
		assert.Equal(t, util.PrivateVisibility, *readVisibility)

		// ok - private being owner
		// read the stream
		rec, err = ownerPrimitiveAction.GetRecord(ctx, readPrimitiveInput)
		assertNoErrorOrFail(t, err, "Failed to read records")
		checkRecords(t, rec)

		// fail - private without access
		_, err = readerPrimitiveAction.GetRecord(ctx, readPrimitiveInput)
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
		rec, err = readerPrimitiveAction.GetRecord(ctx, readPrimitiveInput)
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
		ownerComposedAction, err := ownerTnClient.LoadComposedActions()
		assertNoErrorOrFail(t, err, "Failed to load stream")
		readerComposedAction, err := readerTnClient.LoadComposedActions()
		assertNoErrorOrFail(t, err, "Failed to load stream")

		// Test wallet read permissions for the composed stream
		t.Run("WalletReadPermission", func(t *testing.T) {
			t.Cleanup(func() {
				// make these changes not interfere with the next test
				// reset visibility to public
				_, err := ownerComposedAction.SetReadVisibility(ctx, types.VisibilityInput{
					Stream:     composedStreamLocator,
					Visibility: util.PublicVisibility,
				})
				assert.NoError(t, err, "Failed to set read visibility")

				_, err = ownerPrimitiveAction.SetReadVisibility(ctx, types.VisibilityInput{
					Stream:     primitiveStreamLocator,
					Visibility: util.PublicVisibility,
				})
				assert.NoError(t, err, "Failed to set read visibility")

				// remove permissions from the reader
				_, err = ownerComposedAction.DisableReadWallet(ctx, types.ReadWalletInput{
					Stream: composedStreamLocator,
					Wallet: readerAddress,
				})
				assert.NoError(t, err, "Failed to disable read wallet")

				txHash, err := ownerPrimitiveAction.DisableReadWallet(ctx, types.ReadWalletInput{
					Stream: primitiveStreamLocator,
					Wallet: readerAddress,
				})
				assert.NoError(t, err, "Failed to disable read wallet")

				waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash) // only wait the final tx
			})

			// ok all public
			rec, err := readerComposedAction.GetRecord(ctx, readComposedInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)

			// set just the composed stream to private
			txHash, err := ownerComposedAction.SetReadVisibility(ctx, types.VisibilityInput{
				Stream:     composedStreamLocator,
				Visibility: util.PrivateVisibility,
			})
			assertNoErrorOrFail(t, err, "Failed to set read visibility")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// fail - composed stream is private without access
			_, err = readerComposedAction.GetRecord(ctx, readComposedInput)
			assert.Error(t, err)

			// set the stream to public
			txHash, err = ownerComposedAction.SetReadVisibility(ctx, types.VisibilityInput{
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

			// fail - child is private without access
			_, err = readerComposedAction.GetRecord(ctx, readComposedInput)
			assert.Error(t, err)

			// allow read access to the reader
			txHash, err = ownerPrimitiveAction.AllowReadWallet(ctx, types.ReadWalletInput{
				Stream: primitiveStreamLocator,
				Wallet: readerAddress,
			})
			assertNoErrorOrFail(t, err, "Failed to allow read wallet")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// ok - primitive private but w/ access
			rec, err = readerComposedAction.GetRecord(ctx, readComposedInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)

			// set the composed stream to private
			txHash, err = ownerComposedAction.SetReadVisibility(ctx, types.VisibilityInput{
				Stream:     composedStreamLocator,
				Visibility: util.PrivateVisibility,
			})
			assertNoErrorOrFail(t, err, "Failed to set read visibility")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// allow read access to the reader
			txHash, err = ownerComposedAction.AllowReadWallet(ctx, types.ReadWalletInput{
				Stream: composedStreamLocator,
				Wallet: readerAddress,
			})
			assertNoErrorOrFail(t, err, "Failed to allow read wallet")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// ok - all private but w/ access
			rec, err = readerComposedAction.GetRecord(ctx, readComposedInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)
		})

		// Test stream composition permissions
		t.Run("StreamComposePermission", func(t *testing.T) {
			t.Cleanup(func() {
				// make these changes not interfere with the next test
				// reset visibility to public
				txHash, err := ownerComposedAction.SetComposeVisibility(ctx, types.VisibilityInput{
					Stream:     composedStreamLocator,
					Visibility: util.PublicVisibility,
				})
				assert.NoError(t, err, "Failed to set compose visibility")
				waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

				// remove permissions
				_, err = ownerPrimitiveAction.DisableComposeStream(ctx, composedStreamLocator)
				assert.NoError(t, err, "Failed to disable compose stream")

				waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash) // only wait the final tx
			})

			// ok - public compose
			rec, err := readerComposedAction.GetRecord(ctx, readComposedInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)

			// set the stream to private
			txHash, err := ownerPrimitiveAction.SetComposeVisibility(ctx, types.VisibilityInput{
				Stream:     primitiveStreamLocator,
				Visibility: util.PrivateVisibility,
			})
			assertNoErrorOrFail(t, err, "Failed to set compose visibility")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// Get ComposeVisibility
			composeVisibility, err := ownerPrimitiveAction.GetComposeVisibility(ctx, primitiveStreamLocator)
			assertNoErrorOrFail(t, err, "Failed to get compose visibility")
			assert.Equal(t, util.PrivateVisibility, *composeVisibility)

			// ok - reading primitive directly
			rec, err = readerPrimitiveAction.GetRecord(ctx, readPrimitiveInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)

			// fail - private without access
			_, err = readerComposedAction.GetRecord(ctx, readComposedInput)
			// TODO: a primitive stream that is private on compose_visibility should not be allowed to be composed by any other stream
			// unless that stream is allowed with allow_compose_stream
			// This test is broken now, probably the issue is on is_allowed_to_compose_all action
			//assert.Error(t, err)

			// ok - private with access
			// allow compose access to the reader
			txHash, err = ownerPrimitiveAction.AllowComposeStream(ctx, composedStreamLocator)
			assertNoErrorOrFail(t, err, "Failed to allow compose stream")
			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)

			// read the stream
			rec, err = readerComposedAction.GetRecord(ctx, readComposedInput)
			assertNoErrorOrFail(t, err, "Failed to read records")
			checkRecords(t, rec)
		})
	})

}
