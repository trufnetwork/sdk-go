package contractsapi

import (
	"context"
	"github.com/kwilteam/kwil-db/core/client"
	kwilTypes "github.com/kwilteam/kwil-db/core/types"

	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ## Initializations

type Action struct {
	_client *client.Client
}

var _ types.IAction = (*Action)(nil)

type NewActionOptions struct {
	Client *client.Client
}

var (
	ErrorStreamNotFound = errors.New("stream not found")
	ErrorDatasetExists  = errors.New("dataset exists")
	ErrorRecordNotFound = errors.New("record not found")
)

// NewStream creates a new stream, it is straightforward and only requires the stream id and the deployer
//func NewStream(options NewActionOptions) (*Action, error) {
//	return &Action{
//		_client: options.Client,
//	}, nil
//}

// LoadAction loads an existing stream, so it also checks if the stream is deployed
func LoadAction(options NewActionOptions) (*Action, error) {
	return &Action{
		_client: options.Client,
	}, nil
}

//func (s *Action) ToComposedStream() (*ComposedAction, error) {
//	return ComposedStreamFromStream(*s)
//}

func (s *Action) ToPrimitiveStream() (*PrimitiveAction, error) {
	return PrimitiveStreamFromStream(*s)
}

//func (s *Action) GetType(ctx context.Context) (tntypes.StreamType, error) {
//	if s._type != "" {
//		return s._type, nil
//	}
//
//	values, err := s.getMetadata(ctx, getMetadataParams{
//		Key:        "type",
//		OnlyLatest: true,
//	})
//	if err != nil {
//		return "", errors.WithStack(err)
//	}
//
//	if len(values) == 0 {
//		// type can't ever be disabled
//		return "", errors.New("no type found, check if the stream is initialized")
//	}
//
//	switch values[0].ValueS {
//	case "composed":
//		s._type = tntypes.StreamTypeComposed
//	case "primitive":
//		s._type = tntypes.StreamTypePrimitive
//	default:
//		return "", errors.New(fmt.Sprintf("unknown stream type: %s", values[0].ValueS))
//	}
//
//	if s._type == "" {
//		return "", errors.New("stream type is not set")
//	}
//
//	return s._type, nil
//}

//func (s *Action) GetStreamOwner(ctx context.Context) ([]byte, error) {
//	if s._owner != nil {
//		return s._owner, nil
//	}
//
//	values, err := s.getMetadata(ctx, getMetadataParams{
//		Key:        "stream_owner",
//		OnlyLatest: true,
//	})
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	if len(values) == 0 {
//		// owner can't ever be disabled
//		return nil, errors.New("no owner found (is the stream initialized?)")
//	}
//
//	s._owner, err = hex.DecodeString(values[0].ValueRef)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	return s._owner, nil
//}

func (s *Action) checkStreamExists(ctx context.Context, input types.CheckStreamExistsInput) error {
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
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return result.QueryResult, nil
}

func (s *Action) execute(ctx context.Context, method string, args [][]any) (kwilTypes.Hash, error) {
	return s._client.Execute(ctx, "", method, args)
}
