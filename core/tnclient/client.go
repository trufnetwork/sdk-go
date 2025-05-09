package tnclient

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	kwilClientType "github.com/kwilteam/kwil-db/core/client/types"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/kwilteam/kwil-db/core/gatewayclient"
	"github.com/kwilteam/kwil-db/core/log"
	kwilType "github.com/kwilteam/kwil-db/core/types"
	"github.com/kwilteam/kwil-db/node/types"
	"github.com/pkg/errors"
	tn_api "github.com/trufnetwork/sdk-go/core/contractsapi"
	"github.com/trufnetwork/sdk-go/core/logging"
	clientType "github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"go.uber.org/zap"
)

type Client struct {
	Signer      auth.Signer `validate:"required"`
	logger      *log.Logger
	kwilClient  *gatewayclient.GatewayClient `validate:"required"`
	kwilOptions *gatewayclient.GatewayOptions
}

var _ clientType.Client = (*Client)(nil)

type Option func(*Client)

func NewClient(ctx context.Context, provider string, options ...Option) (*Client, error) {
	c := &Client{}
	c.kwilOptions = &gatewayclient.GatewayOptions{
		Options: *kwilClientType.DefaultOptions(),
	}
	for _, option := range options {
		option(c)
	}

	kwilClient, err := gatewayclient.NewClient(ctx, provider, c.kwilOptions)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c.kwilClient = kwilClient

	// Validate the client
	if err = c.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	return c, nil
}

func (c *Client) Validate() error {
	validate := validator.New()
	return validate.Struct(c)
}

func WithSigner(signer auth.Signer) Option {
	return func(c *Client) {
		c.kwilOptions.Signer = signer
		c.Signer = signer
	}
}

func WithLogger(logger log.Logger) Option {
	return func(c *Client) {
		c.logger = &logger
		c.kwilOptions.Logger = logger
	}
}

func (c *Client) GetSigner() auth.Signer {
	return c.kwilClient.Signer()
}

func (c *Client) WaitForTx(ctx context.Context, txHash kwilType.Hash, interval time.Duration) (*kwilType.TxQueryResponse, error) {
	return c.kwilClient.WaitTx(ctx, txHash, interval)
}

func (c *Client) GetKwilClient() *gatewayclient.GatewayClient {
	return c.kwilClient
}

func (c *Client) DeployStream(ctx context.Context, streamId util.StreamId, streamType clientType.StreamType) (types.Hash, error) {
	return tn_api.DeployStream(ctx, tn_api.DeployStreamInput{
		StreamId:   streamId,
		StreamType: streamType,
		KwilClient: c.GetKwilClient(),
	})
}

func (c *Client) DestroyStream(ctx context.Context, streamId util.StreamId) (types.Hash, error) {
	return tn_api.DestroyStream(ctx, tn_api.DestroyStreamInput{
		StreamId:   streamId,
		KwilClient: c.GetKwilClient(),
	})
}

func (c *Client) LoadActions() (clientType.IAction, error) {
	return tn_api.LoadAction(tn_api.NewActionOptions{
		Client: c.kwilClient,
	})
}

func (c *Client) LoadPrimitiveActions() (clientType.IPrimitiveAction, error) {
	return tn_api.LoadPrimitiveActions(tn_api.NewActionOptions{
		Client: c.kwilClient,
	})
}

func (c *Client) LoadComposedActions() (clientType.IComposedAction, error) {
	return tn_api.LoadComposedActions(tn_api.NewActionOptions{
		Client: c.kwilClient,
	})
}

func (c *Client) OwnStreamLocator(streamId util.StreamId) clientType.StreamLocator {
	return clientType.StreamLocator{
		StreamId:     streamId,
		DataProvider: c.Address(),
	}
}

func (c *Client) Address() util.EthereumAddress {
	addr, err := auth.EthSecp256k1Authenticator{}.Identifier(c.kwilClient.Signer().CompactID())
	if err != nil {
		// should never happen
		logging.Logger.Panic("failed to get address from signer", zap.Error(err))
	}
	address, err := util.NewEthereumAddressFromString(addr)
	if err != nil {
		logging.Logger.Panic("failed to create address from string", zap.Error(err))
	}
	return address
}

// BatchDeployStreams deploys multiple streams (primitive and composed).
// It returns the transaction hash of the batch operation.
func (c *Client) BatchDeployStreams(ctx context.Context, streamDefs []clientType.StreamDefinition) (kwilType.Hash, error) {
	// Assuming SchemaName for "create_streams" is obtained from somewhere or is a known constant.
	// For now, using an empty string as a placeholder if it's a root/global procedure.
	schemaName := "" // Or c.config.SchemaName, etc.

	return tn_api.BatchDeployStreams(ctx, tn_api.BatchDeployStreamsInput{
		KwilClient:  c.GetKwilClient(),
		Definitions: streamDefs,
		SchemaName:  schemaName,
	})
}

// BatchStreamExists checks for the existence of multiple streams.
func (c *Client) BatchStreamExists(ctx context.Context, streams []clientType.StreamLocator) ([]clientType.StreamExistsResult, error) {
	actions, err := c.LoadActions()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actions for BatchStreamExists")
	}
	return actions.BatchStreamExists(ctx, streams)
}

// BatchFilterStreamsByExistence filters a list of streams based on their existence in the database.
// Use this instead of BatchStreamExists if you want less data returned.
func (c *Client) BatchFilterStreamsByExistence(ctx context.Context, streams []clientType.StreamLocator, returnExisting bool) ([]clientType.StreamLocator, error) {
	actions, err := c.LoadActions()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actions for BatchFilterStreamsByExistence")
	}
	return actions.BatchFilterStreamsByExistence(ctx, streams, returnExisting)
}
