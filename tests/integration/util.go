package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

const AnonWalletPK = "0000000000000000000000000000000000000000000000000000000000000abc"

// ## Helper functions

// waitTxToBeMinedWithSuccess waits for a transaction to be successful, failing the test if it fails.
// It returns the transaction response so callers can make further assertions on the result.
func waitTxToBeMinedWithSuccess(t *testing.T, ctx context.Context, client *tnclient.Client, txHash kwiltypes.Hash) *kwiltypes.TxQueryResponse {
	txRes, err := client.WaitForTx(ctx, txHash, time.Second)
	require.NoError(t, err, "Transaction failed")
	require.Equal(t, kwiltypes.CodeOk, kwiltypes.TxCode(txRes.Result.Code), "Transaction code not OK: %s", txRes.Result.Log)
	return txRes
}

// waitTxToBeMinedWithFailure waits for a transaction to be unsuccessful, failing the test if it succeeds.
// It returns the transaction response so callers can make further assertions on the error log.
func waitTxToBeMinedWithFailure(t *testing.T, ctx context.Context, client *tnclient.Client, txHash kwiltypes.Hash) *kwiltypes.TxQueryResponse {
	txRes, err := client.WaitForTx(ctx, txHash, time.Second)
	require.NoError(t, err, "WaitForTx for a failing transaction should not error")
	require.NotEqual(t, kwiltypes.CodeOk, kwiltypes.TxCode(txRes.Result.Code), "Transaction code was OK, but failure was expected. Log: %s", txRes.Result.Log)
	return txRes
}

// assertNoErrorOrFail asserts that an error is nil, failing the test if it is not.
func assertNoErrorOrFail(t *testing.T, err error, msg string) {
	if !assert.NoError(t, err, msg) {
		t.FailNow()
	}
}

func deployTestPrimitiveStreamWithData(
	t *testing.T,
	ctx context.Context,
	tnClient *tnclient.Client,
	streamIds []util.StreamId,
	data []types.InsertRecordInput,
) {
	streamDefs := make([]types.StreamDefinition, len(streamIds))
	for i, streamId := range streamIds {
		streamDefs[i] = types.StreamDefinition{
			StreamId:   streamId,
			StreamType: types.StreamTypePrimitive,
		}
	}
	batchDeployTxHash, err := tnClient.BatchDeployStreams(ctx, streamDefs)
	require.NoError(t, err, "Failed to deploy streams")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, batchDeployTxHash)

	if len(data) > 0 {
		primitiveActions, err := tnClient.LoadPrimitiveActions()
		require.NoError(t, err, "Failed to load stream")

		txHashInsert, err := primitiveActions.InsertRecords(ctx, data)
		require.NoError(t, err, "Failed to insert records")
		waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashInsert)
	}
}

func deployTestComposedStreamWithTaxonomy(
	t *testing.T,
	ctx context.Context,
	tnClient *tnclient.Client,
	streamId util.StreamId,
	taxonomies types.Taxonomy,
) {
	deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypeComposed)
	require.NoError(t, err, "Failed to deploy stream")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, deployTxHash)

	deployedStream, err := tnClient.LoadComposedActions()
	require.NoError(t, err, "Failed to load stream")

	txHashTax, err := deployedStream.InsertTaxonomy(ctx, taxonomies)
	require.NoError(t, err, "Failed to set taxonomy")
	waitTxToBeMinedWithSuccess(t, ctx, tnClient, txHashTax)
}

func authorizeWalletToDeployStreams(
	t *testing.T,
	ctx context.Context,
	fixture *ServerFixture,
	wallet *kwilcrypto.Secp256k1PrivateKey,
) {
	mgrClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(&auth.EthPersonalSigner{Key: *fixture.ManagerPrivateKey}))
	require.NoError(t, err, "failed to create client")

	roleMgmt, err := mgrClient.LoadRoleManagementActions()
	require.NoError(t, err, "Failed to load role management actions")

	pubKey, err := auth.GetUserIdentifier(wallet.Public())
	require.NoError(t, err, "Failed to get user identifier")

	addr, err := util.NewEthereumAddressFromString(pubKey)
	require.NoError(t, err, "Failed to convert user identifier to address")

	txHash, err := roleMgmt.GrantRole(ctx, types.GrantRoleInput{
		Owner:    "system",
		RoleName: "network_writer",
		Wallets:  []util.EthereumAddress{addr},
	})
	require.NoError(t, err, "Failed to grant role")
	waitTxToBeMinedWithSuccess(t, ctx, mgrClient, txHash)
}
