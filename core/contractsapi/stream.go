package contractsapi

import (
	"context"
	"github.com/kwilteam/kwil-db/core/client"
	"github.com/kwilteam/kwil-db/core/types"

	"github.com/pkg/errors"
	tntypes "github.com/trufnetwork/sdk-go/core/types"
)

// ## Initializations

type Stream struct {
	_client *client.Client
}

var _ tntypes.IActions = (*Stream)(nil)

type NewActionOptions struct {
	Client *client.Client
}

var (
	ErrorStreamNotFound = errors.New("stream not found")
	ErrorDatasetExists  = errors.New("dataset exists")
	ErrorRecordNotFound = errors.New("record not found")
)

// NewStream creates a new stream, it is straightforward and only requires the stream id and the deployer
//func NewStream(options NewActionOptions) (*Stream, error) {
//	return &Stream{
//		_client: options.Client,
//	}, nil
//}

// LoadStream loads an existing stream, so it also checks if the stream is deployed
func LoadStream(options NewActionOptions) (*Stream, error) {
	return &Stream{
		_client: options.Client,
	}, nil
}

//func (s *Stream) ToComposedStream() (*ComposedStream, error) {
//	return ComposedStreamFromStream(*s)
//}

func (s *Stream) ToPrimitiveStream() (*PrimitiveStream, error) {
	return PrimitiveStreamFromStream(*s)
}

//func (s *Stream) GetType(ctx context.Context) (tntypes.StreamType, error) {
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

//func (s *Stream) GetStreamOwner(ctx context.Context) ([]byte, error) {
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

//func (s *Stream) checkInitialized(ctx context.Context) error {
//	// check if is deployed
//	err := s.checkDeployed(ctx)
//
//	if err != nil {
//		return errors.WithStack(err)
//	}
//
//	// check if is initialized by trying to get its type
//	//_, err = s.GetType(ctx)
//	//if err != nil {
//	//	return errors.Wrap(err, "check if the stream is initialized")
//	//}
//
//	return nil
//}

//func (s *Stream) checkDeployed(ctx context.Context) error {
//	if s._deployed {
//		return nil
//	}
//
//	result, err := s._client.Call(ctx, "", "stream_exists", []any{s._deployer, s.StreamId})
//	if err != nil {
//		return errors.WithStack(err)
//	}
//
//	if len(result.QueryResult.Values) == 0 || result.QueryResult.Values[0][0] == false {
//		return ErrorStreamNotFound
//	}
//
//	s._deployed = true
//	return nil
//}

func (s *Stream) call(ctx context.Context, method string, args []any) (*types.QueryResult, error) {
	result, err := s._client.Call(ctx, "", method, args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return result.QueryResult, nil
}

func (s *Stream) execute(ctx context.Context, method string, args [][]any) (types.Hash, error) {
	return s._client.Execute(ctx, "", method, args)
}

// except for init, all write methods should be checked for initialization
// this prevents unknown errors when trying to execute a method on a stream that is not initialized
//func (s *Stream) checkedExecute(ctx context.Context, method string, args [][]any) (types.Hash, error) {
//	err := s.checkInitialized(ctx)
//	if err != nil {
//		return types.Hash{}, errors.WithStack(err)
//	}
//
//	return s.execute(ctx, method, args)
//}
