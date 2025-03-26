package contractsapi

import (
	"context"
	client "github.com/kwilteam/kwil-db/core/client/types"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
	"strconv"
)

type PrimitiveAction struct {
	Action
}

var _ types.IPrimitiveAction = (*PrimitiveAction)(nil)

var (
	ErrorStreamNotPrimitive = errors.New("stream is not a primitive stream")
)

func PrimitiveStreamFromStream(stream Action) (*PrimitiveAction, error) {
	return &PrimitiveAction{
		Action: stream,
	}, nil
}

func LoadPrimitiveActions(options NewActionOptions) (*PrimitiveAction, error) {
	stream, err := LoadAction(options)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return PrimitiveStreamFromStream(*stream)
}

// checkValidPrimitiveStream checks if the stream is a valid primitive stream
// and returns an error if it is not. Valid means:
// - the stream is initialized
// - the stream is a primitive stream
func (p *PrimitiveAction) checkValidPrimitiveStream(ctx context.Context) error {
	// first check if is initialized
	//err := p.checkInitialized(ctx)
	//if err != nil {
	//	return errors.WithStack(err)
	//}
	//
	//// then check if is primitive
	//streamType, err := p.GetType(ctx)
	//if err != nil {
	//	return errors.WithStack(err)
	//}
	//
	//if streamType != types.StreamTypePrimitive {
	//	return ErrorStreamNotPrimitive
	//}

	return nil
}

func (p *PrimitiveAction) checkedExecute(ctx context.Context, method string, args [][]any, opts ...client.TxOpt) (kwiltypes.Hash, error) {
	err := p.checkValidPrimitiveStream(ctx)
	if err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return p._client.Execute(ctx, "", method, args, opts...)
}

func (p *PrimitiveAction) InsertRecord(ctx context.Context, input types.InsertRecordInput, opts ...client.TxOpt) (kwiltypes.Hash, error) {
	valueNumeric, err := kwiltypes.ParseDecimalExplicit(strconv.FormatFloat(input.Value, 'f', -1, 64), 36, 18)
	if err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return p.checkedExecute(ctx, "insert_record", [][]any{{
		input.DataProvider,
		input.StreamId,
		input.EventTime,
		valueNumeric,
	}}, opts...)
}

//func (p *PrimitiveAction) InsertRecords(ctx context.Context, inputs []types.InsertRecordsInput, opts ...client.TxOpt) (kwiltypes.Hash, error) {
//	var args [][]any
//	for _, input := range inputs {
//		dateStr := input.EventTime
//		args = append(args, []any{
//			dateStr,
//			fmt.Sprintf("%f", input.Value),
//		})
//	}
//
//	return p.checkedExecute(ctx, "insert_records", args, opts...)
//}

//func (p *PrimitiveAction) GetFirstRecordUnix(ctx context.Context, input types.GetFirstRecordUnixInput) (*types.StreamRecordUnix, error) {
//	err := p.checkValidPrimitiveStream(ctx)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	var args []any
//	args = append(args, transformOrNil(input.AfterDate, func(date int) any { return date }))
//	args = append(args, transformOrNil(input.FrozenAt, func(date time.Time) any { return date.UTC().Format(time.RFC3339) }))
//
//	results, err := p.call(ctx, "get_first_record", args)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	rawOutputs, err := DecodeCallResult[GetRecordUnixRawOutput](results)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	if len(rawOutputs) == 0 {
//		return nil, nil
//	}
//
//	rawOutput := rawOutputs[0]
//	value, _, err := apd.NewFromString(rawOutput.Value)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	return &types.StreamRecordUnix{
//		EventTime: rawOutput.EventTime,
//		Value:     *value,
//	}, nil
//}
