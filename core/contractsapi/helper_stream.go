package contractsapi

import (
	"context"
	"github.com/kwilteam/kwil-db/core/types/transactions"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"strings"
)

// HelperStream implements the IAdminContract interface
type HelperStream struct {
	Stream
}

var _ types.IHelperStream = (*HelperStream)(nil)

func HelperStreamFromStream(stream Stream) (*HelperStream, error) {
	return &HelperStream{
		Stream: stream,
	}, nil
}

func LoadHelperStream(options NewStreamOptions) (*HelperStream, error) {
	stream, err := LoadStream(options)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return HelperStreamFromStream(*stream)
}

// CheckDeployed checks if the contract is deployed
func (c *HelperStream) CheckDeployed(ctx context.Context) (bool, error) {
	_, err := c.Stream._client.GetSchema(ctx, c.Stream.DBID)
	// if the error message CONTAINS "not found", the contract is not deployed
	if err != nil && strings.Contains(err.Error(), "dataset not found") {
		return false, nil
	} else if err != nil {
		return false, errors.WithStack(err)
	} else {
		return true, nil
	}
}

// InsertRecords inserts records into the stream
func (c *HelperStream) InsertRecords(ctx context.Context, inputs types.TnRecordBatch) (transactions.TxHash, error) {
	dataProviderStr := make([]string, 0)
	streamIdStr := make([]string, 0)
	dateValueStr := make([]string, 0)
	valueStr := make([]string, 0)

	for _, instruction := range inputs.Rows {
		dataProviderStr = append(dataProviderStr, instruction.DataProvider)
		streamIdStr = append(streamIdStr, instruction.StreamID)
		dateValueStr = append(dateValueStr, instruction.DateValue)
		valueStr = append(valueStr, instruction.Value)
	}

	inputArgs, err := util.StructAsArgs(types.RawInsertRecordsInput{
		DataProvider: dataProviderStr,
		StreamID:     streamIdStr,
		DateValue:    dateValueStr,
		Value:        valueStr,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert struct to args")
	}

	return c._client.Execute(ctx, c.DBID, "insert_records", [][]any{inputArgs})
}

// InsertRecordsUnix inserts records into the stream
func (c *HelperStream) InsertRecordsUnix(ctx context.Context, inputs types.TnRecordUnixBatch) (transactions.TxHash, error) {
	dataProviderStr := make([]string, 0)
	streamIdStr := make([]string, 0)
	dateValueStr := make([]string, 0)
	valueStr := make([]string, 0)

	for _, instruction := range inputs.Rows {
		dataProviderStr = append(dataProviderStr, instruction.DataProvider)
		streamIdStr = append(streamIdStr, instruction.StreamID)
		dateValueStr = append(dateValueStr, instruction.DateValue)
		valueStr = append(valueStr, instruction.Value)
	}

	inputArgs, err := util.StructAsArgs(types.RawInsertRecordsUnixInput{
		DataProvider: dataProviderStr,
		StreamID:     streamIdStr,
		DateValue:    dateValueStr,
		Value:        valueStr,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return c._client.Execute(ctx, c.DBID, "insert_records_unix", [][]any{inputArgs})
}
