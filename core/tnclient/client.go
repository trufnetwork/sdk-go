package tnclient

import (
	"context"
	"github.com/go-playground/validator/v10"
	"github.com/kwilteam/kwil-db/core/client"
	kwilClientType "github.com/kwilteam/kwil-db/core/client/types"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/kwilteam/kwil-db/core/log"
	kwilType "github.com/kwilteam/kwil-db/core/types"
	"github.com/kwilteam/kwil-db/node/types"

	// "github.com/kwilteam/kwil-db/core/types/transactions"
	"github.com/pkg/errors"
	tn_api "github.com/trufnetwork/sdk-go/core/contractsapi"
	"github.com/trufnetwork/sdk-go/core/logging"
	clientType "github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"go.uber.org/zap"
	"time"
)

type Client struct {
	Signer      auth.Signer `validate:"required"`
	logger      *log.Logger
	kwilClient  *client.Client `validate:"required"`
	kwilOptions *kwilClientType.Options
}

var _ clientType.Client = (*Client)(nil)

type Option func(*Client)

func NewClient(ctx context.Context, provider string, options ...Option) (*Client, error) {
	c := &Client{}
	c.kwilOptions = kwilClientType.DefaultOptions()
	for _, option := range options {
		option(c)
	}

	kwilClient, err := client.NewClient(ctx, provider, c.kwilOptions)
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

func (c *Client) GetKwilClient() *client.Client {
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

//func (c *Client) LoadStream(streamLocator clientType.StreamLocator) (clientType.IStream, error) {
//	return tn_api.LoadStream(tn_api.NewStreamOptions{
//		Client:   c.kwilClient,
//		StreamId: streamLocator.StreamId,
//		Deployer: streamLocator.DataProvider.Bytes(),
//	})
//}

//func (c *Client) LoadPrimitiveStream(streamLocator clientType.StreamLocator) (clientType.IPrimitiveStream, error) {
//	return tn_api.LoadPrimitiveStream(tn_api.NewStreamOptions{
//		Client:   c.kwilClient,
//		StreamId: streamLocator.StreamId,
//		Deployer: streamLocator.DataProvider.Bytes(),
//	})
//}
//
//func (c *Client) LoadComposedStream(streamLocator clientType.StreamLocator) (clientType.IComposedStream, error) {
//	return tn_api.LoadComposedStream(tn_api.NewStreamOptions{
//		Client:   c.kwilClient,
//		StreamId: streamLocator.StreamId,
//		Deployer: streamLocator.DataProvider.Bytes(),
//	})
//}

func (c *Client) OwnStreamLocator(streamId util.StreamId) clientType.StreamLocator {
	return clientType.StreamLocator{
		StreamId:     streamId,
		DataProvider: c.Address(),
	}
}

func (c *Client) Address() util.EthereumAddress {
	address, err := util.NewEthereumAddressFromBytes(c.kwilClient.Signer().PubKey().Bytes())
	if err != nil {
		// should never happen
		logging.Logger.Panic("failed to get address from signer", zap.Error(err))
	}
	return address
}
