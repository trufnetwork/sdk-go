package integration

//
//import (
//	"context"
//	"fmt"
//	"github.com/golang-sql/civil"
//	"github.com/kwilteam/kwil-db/core/crypto"
//	"github.com/kwilteam/kwil-db/core/crypto/auth"
//	"github.com/stretchr/testify/assert"
//	"github.com/trufnetwork/sdk-go/core/tnclient"
//	"github.com/trufnetwork/sdk-go/core/types"
//	"github.com/trufnetwork/sdk-go/core/util"
//	"testing"
//)
//
//// TestPermissions demonstrates the deployment and permission management of primitive and composed streams in TN.
//func TestPermissions(t *testing.T) {
//	ctx := context.Background()
//
//	// Set up owner assets
//	// The owner is the entity deploying and managing the streams
//	ownerPk, err := crypto.Secp256k1PrivateKeyFromHex(TestPrivateKey)
//	assertNoErrorOrFail(t, err, "Failed to parse private key")
//	streamOwnerSigner := &auth.EthPersonalSigner{Key: *ownerPk}
//	ownerTnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(streamOwnerSigner))
//	assertNoErrorOrFail(t, err, "Failed to create client")
//
//	// Set up reader assets
//	// The reader represents a separate entity that will attempt to access the streams
//	readerPk, err := crypto.Secp256k1PrivateKeyFromHex("1111111111111111111111111111111111111111111111111111111111111111")
//	assertNoErrorOrFail(t, err, "Failed to parse private key")
//	readerSigner := &auth.EthPersonalSigner{Key: *readerPk}
//	readerAddress, err := util.NewEthereumAddressFromBytes(readerSigner.Identity())
//	assertNoErrorOrFail(t, err, "Failed to create signer address")
//	readerTnClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(readerSigner))
//	assertNoErrorOrFail(t, err, "Failed to create client")
//
//	// Generate unique stream IDs for primitive and composed streams
//	primitiveStreamId := util.GenerateStreamId("test-wallet-permission-primitive-stream")
//	composedStreamId := util.GenerateStreamId("test-wallet-permission-composed-stream")
//
//	primitiveStreamLocator := ownerTnClient.OwnStreamLocator(primitiveStreamId)
//	composedStreamLocator := ownerTnClient.OwnStreamLocator(composedStreamId)
//
//	// Set up cleanup to destroy the primitive stream after test completion
//	t.Cleanup(func() {
//		destroyResult, err := ownerTnClient.DestroyStream(ctx, primitiveStreamId)
//		assertNoErrorOrFail(t, err, "Failed to destroy stream")
//		waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, destroyResult)
//	})
//
//	// Deploy a primitive stream with initial data
//	deployTestPrimitiveStreamWithData(t, ctx, ownerTnClient, primitiveStreamId, []types.InsertRecordInput{
//		{
//			Value:     1,
//			EventTime: civil.Date{Year: 2020, Month: 1, Day: 1},
//		},
//	})
//
//	// Helper function to check if retrieved records match expected values
//	var checkRecords = func(t *testing.T, rec []types.StreamRecord) {
//		assert.Equal(t, 1, len(rec))
//		assert.Equal(t, "1.000000000000000000", rec[0].Value.String())
//		assert.Equal(t, civil.Date{Year: 2020, Month: 1, Day: 1}, rec[0].EventTime)
//	}
//
//	// Load the primitive stream for both owner and reader
//	ownerPrimitiveStream, err := ownerTnClient.LoadPrimitiveActions(primitiveStreamLocator)
//	assertNoErrorOrFail(t, err, "Failed to load stream")
//	readerPrimitiveStream, err := readerTnClient.LoadPrimitiveActions(primitiveStreamLocator)
//	assertNoErrorOrFail(t, err, "Failed to load stream")
//
//	// Define input for reading records
//	readInput := types.GetRecordInput{
//		From: &civil.Date{Year: 2020, Month: 1, Day: 1},
//		To:   &civil.Date{Year: 2020, Month: 1, Day: 1},
//	}
//
//	// Test primitive stream wallet read permissions
//	t.Run("TestPrimitiveStreamWalletReadPermission", func(t *testing.T) {
//		t.Cleanup(func() {
//			// make these changes not interfere with the next test
//			// reset visibility to public
//			_, err := ownerPrimitiveStream.SetReadVisibility(ctx, util.PublicVisibility)
//			assertNoErrorOrFail(t, err, "Failed to set read visibility")
//			// remove permissions
//			txHash, err := ownerPrimitiveStream.DisableReadWallet(ctx, readerAddress)
//			assertNoErrorOrFail(t, err, "Failed to disable read wallet")
//
//			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash) // only wait the final tx
//		})
//
//		// ok - public read
//		rec, err := readerPrimitiveStream.GetRecord(ctx, readInput)
//		assertNoErrorOrFail(t, err, "Failed to read records")
//		checkRecords(t, rec)
//
//		// set the stream to private
//		txHash, err := ownerPrimitiveStream.SetReadVisibility(ctx, util.PrivateVisibility)
//		assertNoErrorOrFail(t, err, "Failed to set read visibility")
//		waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
//
//		// ok - private being owner
//		// read the stream
//		rec, err = ownerPrimitiveStream.GetRecord(ctx, readInput)
//		assertNoErrorOrFail(t, err, "Failed to read records")
//		checkRecords(t, rec)
//
//		// fail - private without access
//		_, err = readerPrimitiveStream.GetRecord(ctx, readInput)
//		assert.Error(t, err)
//
//		// ok - private with access
//		// allow read access to the reader
//		txHash, err = ownerPrimitiveStream.AllowReadWallet(ctx, readerAddress)
//		assertNoErrorOrFail(t, err, "Failed to allow read wallet")
//		waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
//
//		// read the stream
//		rec, err = readerPrimitiveStream.GetRecord(ctx, readInput)
//		assertNoErrorOrFail(t, err, "Failed to read records")
//		checkRecords(t, rec)
//	})
//
//	// Test composed stream functionality and permissions
//	t.Run("TestComposedStream", func(t *testing.T) {
//		// Set up cleanup to destroy the composed stream after test completion
//		t.Cleanup(func() {
//			destroyResult, err := ownerTnClient.DestroyStream(ctx, composedStreamId)
//			assert.NoError(t, err, "Failed to destroy stream")
//			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, destroyResult)
//		})
//
//		// Deploy a composed stream using the primitive stream as a child
//		deployTestComposedStreamWithTaxonomy(t, ctx, ownerTnClient, composedStreamId, types.Taxonomy{
//			TaxonomyItems: []types.TaxonomyItem{
//				{
//					ChildStream: primitiveStreamLocator,
//					Weight:      1,
//				},
//			}})
//
//		// Load the composed stream for both owner and reader
//		ownerComposedStream, err := ownerTnClient.LoadComposedActions(ownerTnClient.OwnStreamLocator(composedStreamId))
//		assertNoErrorOrFail(t, err, "Failed to load stream")
//		readerComposedStream, err := readerTnClient.LoadComposedActions(ownerTnClient.OwnStreamLocator(composedStreamId))
//		assertNoErrorOrFail(t, err, "Failed to load stream")
//
//		// Test wallet read permissions for the composed stream
//		t.Run("WalletReadPermission", func(t *testing.T) {
//			t.Cleanup(func() {
//				// make these changes not interfere with the next test
//				// reset visibility to public
//				txHash, err := ownerComposedStream.SetReadVisibility(ctx, util.PublicVisibility)
//				assert.NoError(t, err, "Failed to set read visibility")
//
//				txHash, err = ownerPrimitiveStream.SetReadVisibility(ctx, util.PublicVisibility)
//				assert.NoError(t, err, "Failed to set read visibility")
//
//				// remove permissions from the reader
//				txHash, err = ownerComposedStream.DisableReadWallet(ctx, readerAddress)
//				assert.NoError(t, err, "Failed to disable read wallet")
//
//				txHash, err = ownerPrimitiveStream.DisableReadWallet(ctx, readerAddress)
//				assert.NoError(t, err, "Failed to disable read wallet")
//
//				waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash) // only wait the final tx
//			})
//
//			// ok all public
//			rec, err := readerComposedStream.GetRecord(ctx, readInput)
//			assertNoErrorOrFail(t, err, "Failed to read records")
//			checkRecords(t, rec)
//
//			// set just the composed stream to private
//			txHash, err := ownerComposedStream.SetReadVisibility(ctx, util.PrivateVisibility)
//			assertNoErrorOrFail(t, err, "Failed to set read visibility")
//			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
//
//			// fail - composed stream is private without access
//			_, err = readerComposedStream.GetRecord(ctx, readInput)
//			assert.Error(t, err)
//
//			// set the stream to public
//			txHash, err = ownerComposedStream.SetReadVisibility(ctx, util.PublicVisibility)
//			assertNoErrorOrFail(t, err, "Failed to set read visibility")
//			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
//
//			// set the child stream to private
//			txHash, err = ownerPrimitiveStream.SetReadVisibility(ctx, util.PrivateVisibility)
//			assertNoErrorOrFail(t, err, "Failed to set read visibility")
//			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
//			fmt.Println("set private")
//
//			// fail - child is private without access
//			_, err = readerComposedStream.GetRecord(ctx, readInput)
//			assert.Error(t, err)
//
//			// allow read access to the reader
//			txHash, err = ownerPrimitiveStream.AllowReadWallet(ctx, readerAddress)
//			assertNoErrorOrFail(t, err, "Failed to allow read wallet")
//			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
//
//			// ok - primitive private but w/ access
//			rec, err = readerComposedStream.GetRecord(ctx, readInput)
//			assertNoErrorOrFail(t, err, "Failed to read records")
//			checkRecords(t, rec)
//
//			// set the composed stream to private
//			txHash, err = ownerComposedStream.SetReadVisibility(ctx, util.PrivateVisibility)
//			assertNoErrorOrFail(t, err, "Failed to set read visibility")
//			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
//
//			// allow read access to the reader
//			txHash, err = ownerComposedStream.AllowReadWallet(ctx, readerAddress)
//			assertNoErrorOrFail(t, err, "Failed to allow read wallet")
//			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
//
//			// ok - all private but w/ access
//			rec, err = readerComposedStream.GetRecord(ctx, readInput)
//			assertNoErrorOrFail(t, err, "Failed to read records")
//			checkRecords(t, rec)
//		})
//
//		// Test stream composition permissions
//		t.Run("StreamComposePermission", func(t *testing.T) {
//			t.Cleanup(func() {
//				// make these changes not interfere with the next test
//				// reset visibility to public
//				txHash, err := ownerPrimitiveStream.SetComposeVisibility(ctx, util.PublicVisibility)
//				assert.NoError(t, err, "Failed to set compose visibility")
//				// remove permissions
//				txHash, err = ownerPrimitiveStream.DisableComposeStream(ctx, composedStreamLocator)
//				assert.NoError(t, err, "Failed to disable compose stream")
//
//				waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash) // only wait the final tx
//			})
//
//			// ok - public compose
//			rec, err := readerComposedStream.GetRecord(ctx, readInput)
//			assertNoErrorOrFail(t, err, "Failed to read records")
//			checkRecords(t, rec)
//
//			// set the stream to private
//			txHash, err := ownerPrimitiveStream.SetComposeVisibility(ctx, util.PrivateVisibility)
//			assertNoErrorOrFail(t, err, "Failed to set compose visibility")
//			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
//
//			// ok - reading primitive directly
//			rec, err = readerPrimitiveStream.GetRecord(ctx, readInput)
//			assertNoErrorOrFail(t, err, "Failed to read records")
//			checkRecords(t, rec)
//
//			// fail - private without access
//			_, err = readerComposedStream.GetRecord(ctx, readInput)
//			assert.Error(t, err)
//
//			// ok - private with access
//			// allow compose access to the reader
//			txHash, err = ownerPrimitiveStream.AllowComposeStream(ctx, composedStreamLocator)
//			assertNoErrorOrFail(t, err, "Failed to allow compose stream")
//			waitTxToBeMinedWithSuccess(t, ctx, ownerTnClient, txHash)
//
//			// read the stream
//			rec, err = readerComposedStream.GetRecord(ctx, readInput)
//			assertNoErrorOrFail(t, err, "Failed to read records")
//			checkRecords(t, rec)
//		})
//	})
//
//}
