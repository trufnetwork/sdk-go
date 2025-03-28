package contractsapi

import (
	"context"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
)

// func (s *Action) AllowReadWallet(ctx context.Context, wallet util.EthereumAddress) (kwiltypes.Hash, error) {
func (s *Action) AllowReadWallet(ctx context.Context, input types.ReadWalletInput) (kwiltypes.Hash, error) {
	return s.insertMetadata(ctx, InsertMetadataInput{
		Stream: input.Stream,
		Key:    types.AllowReadWalletKey,
		Value:  types.NewMetadataValue(input.Wallet.Address()),
	})
}

// func (s *Action) DisableReadWallet(ctx context.Context, wallet util.EthereumAddress) (kwiltypes.Hash, error) {
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

//func (s *Action) GetComposeVisibility(ctx context.Context) (*util.VisibilityEnum, error) {
//	results, err := s.getMetadata(ctx, getMetadataParams{
//		Key:        types.ComposeVisibilityKey,
//		OnlyLatest: true,
//	})
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	// there can be no visibility set if
//	// - it's not initialized
//	// - all values are disabled
//	if len(results) == 0 {
//		return nil, nil
//	}
//
//	value, err := results[0].GetValueByKey(types.ComposeVisibilityKey)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	visibility, err := util.NewVisibilityEnum(value.(int))
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	return &visibility, nil
//}

func (s *Action) SetComposeVisibility(ctx context.Context, input types.VisibilityInput) (kwiltypes.Hash, error) {
	return s.insertMetadata(ctx, InsertMetadataInput{
		Stream: input.Stream,
		Key:    types.ComposeVisibilityKey,
		Value:  types.NewMetadataValue(int(input.Visibility)),
	})
}

//func (s *Action) GetReadVisibility(ctx context.Context) (*util.VisibilityEnum, error) {
//	values, err := s.getMetadata(ctx, getMetadataParams{
//		Key:        types.ReadVisibilityKey,
//		OnlyLatest: true,
//	})
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	// there can be no visibility set if
//	// - it's not initialized
//	// - all values are disabled
//	if len(values) == 0 {
//		return nil, nil
//	}
//
//	visibility, err := util.NewVisibilityEnum(values[0].ValueI)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	return &visibility, nil
//}

//func (s *Action) GetAllowedReadWallets(ctx context.Context) ([]util.EthereumAddress, error) {
//	results, err := s.getMetadata(ctx, getMetadataParams{
//		Key: types.AllowReadWalletKey,
//	})
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	wallets := make([]util.EthereumAddress, len(results))
//
//	for i, result := range results {
//		value, err := result.GetValueByKey(types.AllowReadWalletKey)
//		if err != nil {
//			return nil, errors.WithStack(err)
//		}
//
//		address, err := util.NewEthereumAddressFromString(value.(string))
//		if err != nil {
//			return nil, errors.WithStack(err)
//		}
//
//		wallets[i] = address
//	}
//
//	return wallets, nil
//}

//func (s *Action) GetAllowedComposeStreams(ctx context.Context) ([]types.StreamLocator, error) {
//	results, err := s.getMetadata(ctx, getMetadataParams{
//		Key: types.AllowComposeStreamKey,
//	})
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	streams := make([]types.StreamLocator, len(results))
//
//	for i, result := range results {
//		value, err := result.GetValueByKey(types.AllowComposeStreamKey)
//		if err != nil {
//			return nil, errors.WithStack(err)
//		}
//
//		// dbids are stored, not streamIds and data providers
//		// so we get this, then later we query the schema
//		dbid, ok := value.(string)
//		if !ok {
//			return nil, errors.New("invalid value type")
//		}
//
//		loc, err := s._client.GetSchema(ctx, dbid)
//		if err != nil {
//			return nil, errors.WithStack(err)
//		}
//
//		streamId, err := util.NewStreamId(loc.Name)
//		if err != nil {
//			return nil, errors.WithStack(err)
//		}
//
//		owner, err := util.NewEthereumAddressFromString(loc.Owner.String())
//		if err != nil {
//			return nil, errors.WithStack(err)
//		}
//
//		streams[i] = types.StreamLocator{
//			StreamId:     *streamId,
//			DataProvider: owner,
//		}
//	}
//
//	return streams, nil
//}

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
	return kwiltypes.Hash{}, errors.New("not implemented")
	metadataList, err := s.getMetadata(ctx, getMetadataParams{
		Key:        input.Key,
		OnlyLatest: true,
		Ref:        input.Ref,
	})

	if err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	if len(metadataList) == 0 {
		return kwiltypes.Hash{}, MetadataValueNotFound
	}

	//return s.disableMetadata(ctx, metadataList[0].RowId)
	rowIdUUID, err := kwiltypes.ParseUUID(metadataList[0].RowId)
	if err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return s.disableMetadata(ctx, DisableMetadataInput{
		Stream: input.Stream,
		RowId:  rowIdUUID,
	})
}
