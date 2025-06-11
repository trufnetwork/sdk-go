package integration

import (
	"context"
	"testing"

	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	apitypes "github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// TestRoleManagement verifies granting and revoking the `system:network_writer` role.
func TestRoleManagement(t *testing.T) {
	fixture := NewServerFixture(t)
	err := fixture.Setup()
	t.Cleanup(func() {
		fixture.Teardown()
	})
	require.NoError(t, err, "Failed to setup server fixture")

	ctx := context.Background()

	// ---------------------------------------------------------------------
	// Bootstrap: manager client (already member of system:network_writers_manager)
	// ---------------------------------------------------------------------
	managerClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(&auth.EthPersonalSigner{Key: *fixture.ManagerPrivateKey}))
	require.NoError(t, err, "failed to create manager client")

	managerAddr := managerClient.Address()

	// ---------------------------------------------------------------------
	// newWriter client – will be granted / revoked
	// ---------------------------------------------------------------------
	newWriterPk, err := crypto.Secp256k1PrivateKeyFromHex("2222222222222222222222222222222222222222222222222222222222222222")
	require.NoError(t, err)
	newWriterClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(&auth.EthPersonalSigner{Key: *newWriterPk}))
	require.NoError(t, err)
	newWriterAddr := newWriterClient.Address()

	// randomUser client – never gets the role
	randomPk, err := crypto.Secp256k1PrivateKeyFromHex("3333333333333333333333333333333333333333333333333333333333333333")
	require.NoError(t, err)
	randomClient, err := tnclient.NewClient(ctx, TestKwilProvider, tnclient.WithSigner(&auth.EthPersonalSigner{Key: *randomPk}))
	require.NoError(t, err)
	randomAddr := randomClient.Address()

	// ---------------------------------------------------------------------
	// Load Role Management API
	// ---------------------------------------------------------------------
	roleMgmt, err := managerClient.LoadRoleManagementActions()
	require.NoError(t, err)

	// Helper to check membership of a single wallet
	isWriter := func(wallet util.EthereumAddress) bool {
		res, err := roleMgmt.AreMembersOf(ctx, apitypes.AreMembersOfInput{
			Owner:    "system",
			RoleName: "network_writer",
			Wallets:  []util.EthereumAddress{wallet},
		})
		require.NoError(t, err)
		require.Len(t, res, 1)
		return res[0].IsMember
	}

	// Initial assertions – none of the three wallets are writers.
	assert.False(t, isWriter(managerAddr))
	assert.False(t, isWriter(newWriterAddr))
	assert.False(t, isWriter(randomAddr))

	//---------------------------------------------------------------------
	// Grant role to newWriter
	//---------------------------------------------------------------------
	grantTx, err := roleMgmt.GrantRole(ctx, apitypes.GrantRoleInput{
		Owner:    "system",
		RoleName: "network_writer",
		Wallets:  []util.EthereumAddress{newWriterAddr},
	})
	require.NoError(t, err)
	waitTxToBeMinedWithSuccess(t, ctx, managerClient, grantTx)

	assert.True(t, isWriter(newWriterAddr))

	//---------------------------------------------------------------------
	// newWriter deploys a primitive stream – should succeed
	//---------------------------------------------------------------------
	streamID := util.GenerateStreamId("role-management-go-test")
	txHash, err := newWriterClient.DeployStream(ctx, streamID, apitypes.StreamTypePrimitive)
	require.NoError(t, err, "new writer failed to deploy primitive stream")
	waitTxToBeMinedWithSuccess(t, ctx, newWriterClient, txHash)

	// Clean up the stream after test
	t.Cleanup(func() {
		destroyHash, err := newWriterClient.DestroyStream(ctx, streamID)
		if err == nil {
			waitTxToBeMinedWithSuccess(t, ctx, newWriterClient, destroyHash)
		}
	})

	//---------------------------------------------------------------------
	// Revoke role from newWriter
	//---------------------------------------------------------------------
	revokeTx, err := roleMgmt.RevokeRole(ctx, apitypes.RevokeRoleInput{
		Owner:    "system",
		RoleName: "network_writer",
		Wallets:  []util.EthereumAddress{newWriterAddr},
	})
	require.NoError(t, err)
	waitTxToBeMinedWithSuccess(t, ctx, managerClient, revokeTx)

	assert.False(t, isWriter(newWriterAddr))

	// Deployment should now fail on-chain for newWriter (submission succeeds, tx fails)
	streamIDRevoked := util.GenerateStreamId("role-management-go-test-revoked")
	txRevoked, err := newWriterClient.DeployStream(ctx, streamIDRevoked, apitypes.StreamTypePrimitive)
	// client-side submission should succeed; failure occurs on-chain
	require.NoError(t, err, "tx submission should succeed even though on-chain will fail")
	waitTxToBeMinedWithFailure(t, ctx, newWriterClient, txRevoked)

	// randomUser should also fail on-chain
	randStream := util.GenerateStreamId("role-management-go-test-random")
	txRand, err := randomClient.DeployStream(ctx, randStream, apitypes.StreamTypePrimitive)
	require.NoError(t, err, "tx submission should succeed for random user (failure is on-chain)")
	waitTxToBeMinedWithFailure(t, ctx, randomClient, txRand)
}
