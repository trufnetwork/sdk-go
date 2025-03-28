package contractsapi

import (
	"context"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"strconv"
)

func (s *Action) AllowReadWallet(ctx context.Context, input types.ReadWalletInput) (kwiltypes.Hash, error) {
	return s.insertMetadata(ctx, InsertMetadataInput{
		Stream: input.Stream,
		Key:    types.AllowReadWalletKey,
		Value:  types.NewMetadataValue(input.Wallet.Address()),
	})
}

func (s *Action) DisableReadWallet(ctx context.Context, input types.ReadWalletInput) (kwiltypes.Hash, error) {
	return s.disableMetadataByRef(ctx, DisableMetadataByRefInput{
		Stream: input.Stream,
		Key:    types.AllowReadWalletKey,
		Ref:    input.Wallet.Address(),
	})
}

func (s *Action) AllowComposeStream(ctx context.Context, locator types.StreamLocator) (kwiltypes.Hash, error) {
	return s.insertMetadata(ctx, InsertMetadataInput{
		Stream: locator,
		Key:    types.AllowComposeStreamKey,
		Value:  types.NewMetadataValue(locator.StreamId.String()),
	})
}

func (s *Action) DisableComposeStream(ctx context.Context, locator types.StreamLocator) (kwiltypes.Hash, error) {
	return s.disableMetadataByRef(ctx, DisableMetadataByRefInput{
		Stream: locator,
		Key:    types.AllowComposeStreamKey,
		Ref:    locator.StreamId.String(),
	})
}

func (s *Action) GetComposeVisibility(ctx context.Context, locator types.StreamLocator) (*util.VisibilityEnum, error) {
	results, err := s.getMetadata(ctx, getMetadataParams{
		Stream:  locator,
		Key:     types.ComposeVisibilityKey,
		Limit:   1,
		Offset:  0,
		OrderBy: "created_at DESC",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// there can be no visibility set if
	// - it's not initialized
	// - all values are disabled
	if len(results) == 0 {
		return nil, nil
	}

	value, err := results[0].getValueByKey(types.ComposeVisibilityKey)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// convert string into int
	valueInt, err := strconv.Atoi(value)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	visibility, err := util.NewVisibilityEnum(valueInt)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &visibility, nil
}

func (s *Action) SetComposeVisibility(ctx context.Context, input types.VisibilityInput) (kwiltypes.Hash, error) {
	return s.insertMetadata(ctx, InsertMetadataInput{
		Stream: input.Stream,
		Key:    types.ComposeVisibilityKey,
		Value:  types.NewMetadataValue(int(input.Visibility)),
	})
}

func (s *Action) GetReadVisibility(ctx context.Context, locator types.StreamLocator) (*util.VisibilityEnum, error) {
	results, err := s.getMetadata(ctx, getMetadataParams{
		Stream:  locator,
		Key:     types.ReadVisibilityKey,
		Limit:   1,
		Offset:  0,
		OrderBy: "created_at DESC",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// there can be no visibility set if
	// - it's not initialized
	// - all values are disabled
	if len(results) == 0 {
		return nil, nil
	}

	value, err := results[0].getValueByKey(types.ReadVisibilityKey)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// convert string into int
	valueInt, err := strconv.Atoi(value)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	visibility, err := util.NewVisibilityEnum(valueInt)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &visibility, nil
}

func (s *Action) GetAllowedReadWallets(ctx context.Context, locator types.StreamLocator) ([]util.EthereumAddress, error) {
	results, err := s.getMetadata(ctx, getMetadataParams{
		Stream:  locator,
		Key:     types.AllowReadWalletKey,
		OrderBy: "created_at DESC",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	wallets := make([]util.EthereumAddress, len(results))

	for i, result := range results {
		value, err := result.getValueByKey(types.AllowReadWalletKey)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		address, err := util.NewEthereumAddressFromString(value)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		wallets[i] = address
	}

	return wallets, nil
}

func (s *Action) GetAllowedComposeStreams(ctx context.Context, locator types.StreamLocator) ([]types.StreamLocator, error) {
	results, err := s.getMetadata(ctx, getMetadataParams{
		Stream:  locator,
		Key:     types.AllowComposeStreamKey,
		OrderBy: "created_at DESC",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	streams := make([]types.StreamLocator, len(results))

	for i, result := range results {
		value, err := result.getValueByKey(types.AllowComposeStreamKey)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		streamId, err := util.NewStreamId(value)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		streams[i] = types.StreamLocator{
			StreamId:     *streamId,
			DataProvider: locator.DataProvider,
		}
	}

	return streams, nil
}

func (s *Action) SetReadVisibility(ctx context.Context, input types.VisibilityInput) (kwiltypes.Hash, error) {
	return s.insertMetadata(ctx, InsertMetadataInput{
		Stream: input.Stream,
		Key:    types.ReadVisibilityKey,
		Value:  types.NewMetadataValue(int(input.Visibility)),
	})
}

func (s *Action) SetDefaultBaseTime(ctx context.Context, input types.DefaultBaseTimeInput) (kwiltypes.Hash, error) {
	return s.insertMetadata(ctx, InsertMetadataInput{
		Stream: input.Stream,
		Key:    types.DefaultBaseTimeKey,
		Value:  types.NewMetadataValue(input.BaseTime),
	})
}

var MetadataValueNotFound = errors.New("metadata value not found")

type DisableMetadataByRefInput struct {
	Stream types.StreamLocator
	Key    types.MetadataKey
	Ref    string
}

func (s *Action) disableMetadataByRef(ctx context.Context, input DisableMetadataByRefInput) (kwiltypes.Hash, error) {
	metadataList, err := s.getMetadata(ctx, getMetadataParams{
		Stream:  input.Stream,
		Key:     input.Key,
		Ref:     input.Ref,
		Limit:   1,
		Offset:  0,
		OrderBy: "created_at DESC",
	})
	if err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	if len(metadataList) == 0 {
		return kwiltypes.Hash{}, MetadataValueNotFound
	}

	rowIdUUID, err := kwiltypes.ParseUUID(metadataList[0].RowId)
	if err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return s.disableMetadata(ctx, DisableMetadataInput{
		Stream: input.Stream,
		RowId:  rowIdUUID,
	})
}
