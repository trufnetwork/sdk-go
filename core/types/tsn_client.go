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
	// LoadActions loads a already deployed stream, permitting its API usage
	LoadActions() (IAction, error)
	// LoadPrimitiveActions loads a already deployed primitive stream, permitting its API usage
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
	// ListStreams returns list streams from the Truf network
	ListStreams(ctx context.Context, input ListStreamsInput) ([]ListStreamsOutput, error)
	// DeployComposedStreamWithTaxonomy deploys a composed stream with a taxonomy
	DeployComposedStreamWithTaxonomy(ctx context.Context, streamId util.StreamId, taxonomy Taxonomy) error
}
