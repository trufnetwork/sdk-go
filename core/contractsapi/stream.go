package contractsapi

import (
	"context"
	"encoding/hex"
	"github.com/kwilteam/kwil-db/core/gatewayclient"

	kwilTypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ## Initializations

type Action struct {
	_client *gatewayclient.GatewayClient
}

var _ types.IAction = (*Action)(nil)

type NewActionOptions struct {
	Client *gatewayclient.GatewayClient
}

var (
	ErrorStreamNotFound = errors.New("stream not found")
	ErrorRecordNotFound = errors.New("record not found")
)

// LoadAction loads an existing stream, so it also checks if the stream is deployed
func LoadAction(options NewActionOptions) (*Action, error) {
	return &Action{
		_client: options.Client,
	}, nil
}

func (s *Action) ToPrimitiveStream() (*PrimitiveAction, error) {
	return PrimitiveStreamFromStream(*s)
}

func (s *Action) GetType(ctx context.Context, locator types.StreamLocator) (types.StreamType, error) {
	results, err := s.getMetadata(ctx, getMetadataParams{
		Stream:  locator,
		Key:     "type",
		Limit:   1,
		Offset:  0,
		OrderBy: "created_at DESC",
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	if len(results) == 0 {
		// type can't ever be disabled
		return "", errors.New("no type found, check if the stream is initialized")
	}

	value, err := results[0].getValueByKey("type")
	if err != nil {
		return "", errors.WithStack(err)
	}

	return types.StreamType(value), nil
}

func (s *Action) GetStreamOwner(ctx context.Context, locator types.StreamLocator) ([]byte, error) {
	values, err := s.getMetadata(ctx, getMetadataParams{
		Stream:  locator,
		Key:     "stream_owner",
		Limit:   1,
		Offset:  0,
		OrderBy: "created_at DESC",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(values) == 0 {
		// owner can't ever be disabled
		return nil, errors.New("no owner found (is the stream initialized?)")
	}

	return hex.DecodeString(values[0].ValueRef)
}

// CheckStreamExists checks if the stream exists
func (s *Action) CheckStreamExists(ctx context.Context, input types.CheckStreamExistsInput) error {
	result, err := s._client.Call(ctx, "", "stream_exists", []any{input.DataProvider, input.StreamId})
	if err != nil {
		return errors.WithStack(err)
	}

	if len(result.QueryResult.Values) == 0 || result.QueryResult.Values[0][0] == false {
		return ErrorStreamNotFound
	}

	return nil
}

func (s *Action) call(ctx context.Context, method string, args []any) (*kwilTypes.QueryResult, error) {
	result, err := s._client.Call(ctx, "", method, args)
	if err != nil || result.Error != nil {
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return nil, errors.New(*result.Error)
	}

	return result.QueryResult, nil
}

func (s *Action) execute(ctx context.Context, method string, args [][]any) (kwilTypes.Hash, error) {
	return s._client.Execute(ctx, "", method, args)
}
