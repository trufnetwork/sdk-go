package types

import (
	"context"
	"github.com/kwilteam/kwil-db/core/client"
	"time"

	"github.com/kwilteam/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

type Client interface {
	// WaitForTx waits for the transaction to be mined by TN
	WaitForTx(ctx context.Context, txHash types.Hash, interval time.Duration) (*types.TxQueryResponse, error)
	// GetKwilClient returns the kwil client used by the client
	GetKwilClient() *client.Client
	// DeployStream deploys a new stream
	DeployStream(ctx context.Context, streamId util.StreamId, streamType StreamType) (types.Hash, error)
	// DestroyStream destroys a stream
	DestroyStream(ctx context.Context, streamId util.StreamId) (types.Hash, error)
	// LoadStream loads a already deployed stream, permitting its API usage
	LoadActions() (IAction, error)
	// LoadPrimitiveStream loads a already deployed primitive stream, permitting its API usage
	LoadPrimitiveActions() (IPrimitiveAction, error)
	// LoadComposedActions loads a already deployed composed stream, permitting its API usage
	LoadComposedActions() (IComposedAction, error)
	/*
	 * utils for the client
	 */
	// Create a new stream locator
	OwnStreamLocator(streamId util.StreamId) StreamLocator
	// Address of the signer used by the client
	Address() util.EthereumAddress
	// GetAllStreams returns all streams from the Truf network
	//GetAllStreams(ctx context.Context, input GetAllStreamsInput) ([]StreamLocator, error)
	// GetAllInitializedStreams returns all streams from the Truf Network that are initialized
	//GetAllInitializedStreams(ctx context.Context, input GetAllStreamsInput) ([]StreamLocator, error)
	// DeployComposedStreamWithTaxonomy deploys a composed stream with a taxonomy
	DeployComposedStreamWithTaxonomy(ctx context.Context, streamId util.StreamId, taxonomy Taxonomy) error
}

type GetAllStreamsInput struct {
	Owner []byte
}
