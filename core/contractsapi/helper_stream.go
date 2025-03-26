package contractsapi

//
//import (
//	"context"
//	kwiltypes "github.com/kwilteam/kwil-db/core/types"
//	// "github.com/kwilteam/kwil-db/core/types/transactions"
//	"github.com/pkg/errors"
//	"github.com/trufnetwork/sdk-go/core/types"
//	"github.com/trufnetwork/sdk-go/core/util"
//)
//
//// HelperStream implements the IAdminContract interface
//type HelperStream struct {
//	Action
//}
//
//var _ types.IHelperStream = (*HelperStream)(nil)
//
//func HelperStreamFromStream(stream Action) (*HelperStream, error) {
//	return &HelperStream{
//		Action: stream,
//	}, nil
//}
//
////func LoadHelperStream(options NewActionOptions) (*HelperStream, error) {
////	stream, err := LoadActions(options)
////	if err != nil {
////		return nil, errors.WithStack(err)
////	}
////	return HelperStreamFromStream(*stream)
////}
//
//// CheckDeployed checks if the contract is deployed
////func (c *HelperStream) CheckDeployed(ctx context.Context) (bool, error) {
////	_, err := c.Action._client.GetSchema(ctx, c.Action.DBID)
////	// if the error message CONTAINS "not found", the contract is not deployed
////	if err != nil && strings.Contains(err.Error(), "dataset not found") {
////		return false, nil
////	} else if err != nil {
////		return false, errors.WithStack(err)
////	} else {
////		return true, nil
////	}
////}
//
//// InsertRecords inserts records into the stream
//func (c *HelperStream) InsertRecords(ctx context.Context, inputs types.TnRecordBatch) (kwiltypes.Hash, error) {
//	dataProviderStr := make([]string, 0)
//	streamIdStr := make([]string, 0)
//	dateValueStr := make([]string, 0)
//	valueStr := make([]string, 0)
//
//	for _, instruction := range inputs.Rows {
//		dataProviderStr = append(dataProviderStr, instruction.DataProvider)
//		streamIdStr = append(streamIdStr, instruction.StreamID)
//		dateValueStr = append(dateValueStr, instruction.EventTime)
//		valueStr = append(valueStr, instruction.Value)
//	}
//
//	inputArgs, err := util.StructAsArgs(types.RawInsertRecordsInput{
//		DataProvider: dataProviderStr,
//		StreamID:     streamIdStr,
//		EventTime:    dateValueStr,
//		Value:        valueStr,
//	})
//	if err != nil {
//		return kwiltypes.Hash{}, errors.Wrap(err, "failed to convert struct to args")
//	}
//
//	return c._client.Execute(ctx, c.DBID, "insert_records", [][]any{inputArgs})
//}
//
//// InsertRecordsUnix inserts records into the stream
//func (c *HelperStream) InsertRecordsUnix(ctx context.Context, inputs types.TnRecordUnixBatch) (kwiltypes.Hash, error) {
//	dataProviderStr := make([]string, 0)
//	streamIdStr := make([]string, 0)
//	dateValueStr := make([]string, 0)
//	valueStr := make([]string, 0)
//
//	for _, instruction := range inputs.Rows {
//		dataProviderStr = append(dataProviderStr, instruction.DataProvider)
//		streamIdStr = append(streamIdStr, instruction.StreamID)
//		dateValueStr = append(dateValueStr, instruction.EventTime)
//		valueStr = append(valueStr, instruction.Value)
//	}
//
//	inputArgs, err := util.StructAsArgs(types.RawInsertRecordsUnixInput{
//		DataProvider: dataProviderStr,
//		StreamID:     streamIdStr,
//		EventTime:    dateValueStr,
//		Value:        valueStr,
//	})
//	if err != nil {
//		return kwiltypes.Hash{}, errors.WithStack(err)
//	}
//
//	return c._client.Execute(ctx, c.DBID, "insert_records_unix", [][]any{inputArgs})
//}
//
//// FilterInitialized filters out non-initialized streams
//func (c *HelperStream) FilterInitialized(ctx context.Context, inputs types.FilterInitializedInput) ([]types.FilterInitializedResult, error) {
//	inputArgs, err := util.StructAsArgs(types.RawFilterInitializedInput{
//		DataProvider: inputs.DataProviders,
//		StreamID:     inputs.StreamIDs,
//	})
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	results, err := c.call(ctx, "filter_initialized", inputArgs)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	type rawResult struct {
//		DataProvider string `json:"data_provider"`
//		StreamID     string `json:"stream_id"`
//	}
//
//	rawResults, err := DecodeCallResult[rawResult](results)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	var output []types.FilterInitializedResult
//	for _, result := range rawResults {
//		output = append(output, types.FilterInitializedResult{
//			DataProvider: result.DataProvider,
//			StreamID:     result.StreamID,
//		})
//	}
//
//	return output, nil
//}
