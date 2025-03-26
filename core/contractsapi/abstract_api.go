package contractsapi

//
//import (
//	"context"
//	kwiltypes "github.com/kwilteam/kwil-db/core/types"
//
//	"github.com/pkg/errors"
//	"github.com/trufnetwork/sdk-go/core/types"
//	"github.com/trufnetwork/sdk-go/core/util"
//)
//
//func (s *Action) AllowReadWallet(ctx context.Context, wallet util.EthereumAddress) (kwiltypes.Hash, error) {
//	return s.insertMetadata(ctx, types.AllowReadWalletKey, types.NewMetadataValue(wallet.Address()))
//}
//
//func (s *Action) DisableReadWallet(ctx context.Context, wallet util.EthereumAddress) (kwiltypes.Hash, error) {
//	return s.disableMetadataByRef(ctx, types.AllowReadWalletKey, wallet.Address())
//}
//
////func (s *Action) AllowComposeStream(ctx context.Context, locator types.StreamLocator) (kwiltypes.Hash, error) {
////	streamId := locator.StreamId
////	dbid := utils.GenerateDBID(streamId.String(), locator.DataProvider.Bytes())
////	return s.insertMetadata(ctx, types.AllowComposeStreamKey, types.NewMetadataValue(dbid))
////}
//
////func (s *Action) DisableComposeStream(ctx context.Context, locator types.StreamLocator) (kwiltypes.Hash, error) {
////	dbid := utils.GenerateDBID(locator.StreamId.String(), locator.DataProvider.Bytes())
////	return s.disableMetadataByRef(ctx, types.AllowComposeStreamKey, dbid)
////}
//
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
//
//func (s *Action) SetComposeVisibility(ctx context.Context, visibility util.VisibilityEnum) (kwiltypes.Hash, error) {
//	return s.insertMetadata(ctx, types.ComposeVisibilityKey, types.NewMetadataValue(int(visibility)))
//}
//
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
//
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
//
////func (s *Action) GetAllowedComposeStreams(ctx context.Context) ([]types.StreamLocator, error) {
////	results, err := s.getMetadata(ctx, getMetadataParams{
////		Key: types.AllowComposeStreamKey,
////	})
////	if err != nil {
////		return nil, errors.WithStack(err)
////	}
////
////	streams := make([]types.StreamLocator, len(results))
////
////	for i, result := range results {
////		value, err := result.GetValueByKey(types.AllowComposeStreamKey)
////		if err != nil {
////			return nil, errors.WithStack(err)
////		}
////
////		// dbids are stored, not streamIds and data providers
////		// so we get this, then later we query the schema
////		dbid, ok := value.(string)
////		if !ok {
////			return nil, errors.New("invalid value type")
////		}
////
////		loc, err := s._client.GetSchema(ctx, dbid)
////		if err != nil {
////			return nil, errors.WithStack(err)
////		}
////
////		streamId, err := util.NewStreamId(loc.Name)
////		if err != nil {
////			return nil, errors.WithStack(err)
////		}
////
////		owner, err := util.NewEthereumAddressFromString(loc.Owner.String())
////		if err != nil {
////			return nil, errors.WithStack(err)
////		}
////
////		streams[i] = types.StreamLocator{
////			StreamId:     *streamId,
////			DataProvider: owner,
////		}
////	}
////
////	return streams, nil
////}
//
//func (s *Action) SetReadVisibility(ctx context.Context, visibility util.VisibilityEnum) (kwiltypes.Hash, error) {
//	return s.insertMetadata(ctx, types.ReadVisibilityKey, types.NewMetadataValue(int(visibility)))
//}
//
//func (s *Action) SetDefaultBaseDate(ctx context.Context, baseDate string) (kwiltypes.Hash, error) {
//	return s.insertMetadata(ctx, types.DefaultBaseDateKey, types.NewMetadataValue(baseDate))
//}
//
//var MetadataValueNotFound = errors.New("metadata value not found")
//
//func (s *Action) disableMetadataByRef(ctx context.Context, key types.MetadataKey, ref string) (kwiltypes.Hash, error) {
//	metadataList, err := s.getMetadata(ctx, getMetadataParams{
//		Key:        key,
//		OnlyLatest: true,
//		Ref:        ref,
//	})
//
//	if err != nil {
//		return kwiltypes.Hash{}, errors.WithStack(err)
//	}
//
//	if len(metadataList) == 0 {
//		return kwiltypes.Hash{}, MetadataValueNotFound
//	}
//
//	return s.disableMetadata(ctx, metadataList[0].RowId)
//}
