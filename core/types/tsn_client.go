package types

import (
	"context"
	"github.com/trufnetwork/kwil-db/core/gatewayclient"
	"time"

	"github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

type Client interface {
	// WaitForTx waits for the transaction to be mined by TN
	WaitForTx(ctx context.Context, txHash types.Hash, interval time.Duration) (*types.TxQueryResponse, error)
	// GetKwilClient returns the kwil client used by the client
	GetKwilClient() *gatewayclient.GatewayClient
	// DeployStream deploys a new stream
	DeployStream(ctx context.Context, streamId util.StreamId, streamType StreamType) (types.Hash, error)
	// DestroyStream destroys a stream
	DestroyStream(ctx context.Context, streamId util.StreamId) (types.Hash, error)
	// LoadActions loads a already deployed stream, permitting its API usage
	LoadActions() (IAction, error)
	// LoadPrimitiveActions loads a already deployed primitive stream, permitting its API usage
	LoadPrimitiveActions() (IPrimitiveAction, error)
	// LoadComposedActions loads a already deployed composed stream, permitting its API usage
	LoadComposedActions() (IComposedAction, error)
	// LoadRoleManagementActions loads the role management contract API, permitting its API usage
	LoadRoleManagementActions() (IRoleManagement, error)
	// LoadAttestationActions loads the attestation contract API, permitting its API usage
	LoadAttestationActions() (IAttestationAction, error)
	/*
	 * utils for the client
	 */
	// Create a new stream locator
	OwnStreamLocator(streamId util.StreamId) StreamLocator
	// Address of the signer used by the client
	Address() util.EthereumAddress
	// ListStreams returns list streams from the Truf network
	ListStreams(ctx context.Context, input ListStreamsInput) ([]ListStreamsOutput, error)
	// DeployComposedStreamWithTaxonomy deploys a composed stream with a taxonomy
	DeployComposedStreamWithTaxonomy(ctx context.Context, streamId util.StreamId, taxonomy Taxonomy) error
	// BatchDeployStreams deploys multiple streams (primitive and composed).
	// Returns the transaction hash of the batch operation.
	BatchDeployStreams(ctx context.Context, streamDefs []StreamDefinition) (types.Hash, error)
	// BatchStreamExists checks for the existence of multiple streams.
	BatchStreamExists(ctx context.Context, streams []StreamLocator) ([]StreamExistsResult, error)
	// BatchFilterStreamsByExistence filters a list of streams based on their existence in the database.
	// Use this instead of BatchStreamExists if you want less data returned.
	BatchFilterStreamsByExistence(ctx context.Context, streams []StreamLocator, returnExisting bool) ([]StreamLocator, error)
}
