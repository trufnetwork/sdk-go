package contractsapi

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/kwilteam/kwil-db/core/types"
	"github.com/kwilteam/kwil-db/core/types/client"
	"github.com/kwilteam/kwil-db/core/types/transactions"
	kwilUtils "github.com/kwilteam/kwil-db/core/utils"
	"github.com/pkg/errors"
	tntypes "github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// ## Initializations

type Stream struct {
	StreamId     util.StreamId
	_type        tntypes.StreamType
	_deployer    []byte
	_owner       []byte
	DBID         string
	_client      client.Client
	_initialized bool
	_deployed    bool
}

var _ tntypes.IStream = (*Stream)(nil)

type NewStreamOptions struct {
	Client   client.Client
	StreamId util.StreamId
	Deployer []byte
}

var (
	ErrorStreamNotFound = errors.New("stream not found")
	ErrorDatasetExists  = errors.New("dataset exists")
	ErrorRecordNotFound = errors.New("record not found")
)

// NewStream creates a new stream, it is straightforward and only requires the stream id and the deployer
func NewStream(options NewStreamOptions) (*Stream, error) {
	optClient := options.Client
	streamId := options.StreamId
	deployer := options.Deployer

	// if there's no deployer, let's throw an error
	if len(deployer) == 0 {
		return nil, errors.New("contract owner is required")
	}

	dbid := kwilUtils.GenerateDBID(streamId.String(), deployer)
	// check if the stream is found
	if _, err := optClient.GetSchema(context.Background(), dbid); err == nil {
		// if there's no error, it means the stream is already deployed
		return nil, ErrorDatasetExists
	}

	return &Stream{
		StreamId:  streamId,
		_deployer: deployer,
		DBID:      dbid,
		_client:   optClient,
	}, nil
}

// LoadStream loads an existing stream, so it also checks if the stream is deployed
func LoadStream(options NewStreamOptions) (*Stream, error) {
	streamId := options.StreamId
	deployer := options.Deployer
	optClient := options.Client

	if len(deployer) == 0 {
		return nil, errors.New("contract owner is required")
	}

	dbid := kwilUtils.GenerateDBID(streamId.String(), deployer)
	// check if the stream is found
	if _, err := optClient.GetSchema(context.Background(), dbid); err != nil {
		// if err contains "dataset not found", it means the stream is not deployed, then we return our error
		if strings.Contains(err.Error(), "dataset not found") {
			return nil, ErrorStreamNotFound
		}

		return nil, errors.WithStack(err)
	}

	return &Stream{
		StreamId:  streamId,
		_deployer: options.Deployer,
		DBID:      dbid,
		_client:   optClient,
	}, nil
}

func (s *Stream) ToComposedStream() (*ComposedStream, error) {
	return ComposedStreamFromStream(*s)
}

func (s *Stream) ToPrimitiveStream() (*PrimitiveStream, error) {
	return PrimitiveStreamFromStream(*s)
}

func (s *Stream) GetSchema(ctx context.Context) (*types.Schema, error) {
	return s._client.GetSchema(ctx, s.DBID)
}

func (s *Stream) GetType(ctx context.Context) (tntypes.StreamType, error) {
	if s._type != "" {
		return s._type, nil
	}

	values, err := s.getMetadata(ctx, getMetadataParams{
		Key:        "type",
		OnlyLatest: true,
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	if len(values) == 0 {
		// type can't ever be disabled
		return "", errors.New("no type found, check if the stream is initialized")
	}

	switch values[0].ValueS {
	case "composed":
		s._type = tntypes.StreamTypeComposed
	case "primitive":
		s._type = tntypes.StreamTypePrimitive
	default:
		return "", errors.New(fmt.Sprintf("unknown stream type: %s", values[0].ValueS))
	}

	if s._type == "" {
		return "", errors.New("stream type is not set")
	}

	return s._type, nil
}

func (s *Stream) GetStreamOwner(ctx context.Context) ([]byte, error) {
	if s._owner != nil {
		return s._owner, nil
	}

	values, err := s.getMetadata(ctx, getMetadataParams{
		Key:        "stream_owner",
		OnlyLatest: true,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(values) == 0 {
		// owner can't ever be disabled
		return nil, errors.New("no owner found (is the stream initialized?)")
	}

	s._owner, err = hex.DecodeString(values[0].ValueRef)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return s._owner, nil
}

func (s *Stream) checkInitialized(ctx context.Context) error {
	if s._initialized {
		return nil
	}

	// check if is deployed
	err := s.checkDeployed(ctx)

	if err != nil {
		return errors.WithStack(err)
	}

	// check if is initialized by trying to get its type
	_, err = s.GetType(ctx)
	if err != nil {
		return errors.Wrap(err, "check if the stream is initialized")
	}

	s._initialized = true

	return nil
}

func (s *Stream) checkDeployed(ctx context.Context) error {
	if s._deployed {
		return nil
	}

	_, err := s.GetSchema(ctx)
	if err != nil {
		return errors.Wrap(err, "check if the stream is deployed")
	}

	s._deployed = true

	return nil
}

func (s *Stream) call(ctx context.Context, method string, args []any) (*client.Records, error) {
	result, err := s._client.Call(ctx, s.DBID, method, args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return result.Records, nil
}

func (s *Stream) execute(ctx context.Context, method string, args [][]any) (transactions.TxHash, error) {
	return s._client.Execute(ctx, s.DBID, method, args)
}

// except for init, all write methods should be checked for initialization
// this prevents unknown errors when trying to execute a method on a stream that is not initialized
func (s *Stream) checkedExecute(ctx context.Context, method string, args [][]any) (transactions.TxHash, error) {
	err := s.checkInitialized(ctx)
	if err != nil {
		return transactions.TxHash{}, errors.WithStack(err)
	}

	return s.execute(ctx, method, args)
}
