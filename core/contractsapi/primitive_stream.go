package contractsapi

//
//import (
//	"context"
//	"fmt"
//	client "github.com/kwilteam/kwil-db/core/client/types"
//	kwiltypes "github.com/kwilteam/kwil-db/core/types"
//	"time"
//
//	"github.com/cockroachdb/apd/v3"
//	// "github.com/kwilteam/kwil-db/core/types/client"
//	// "github.com/kwilteam/kwil-db/core/types/transactions"
//	"github.com/pkg/errors"
//	"github.com/trufnetwork/sdk-go/core/types"
//)
//
//type PrimitiveStream struct {
//	Stream
//}
//
//var _ types.IPrimitiveStream = (*PrimitiveStream)(nil)
//
//var (
//	ErrorStreamNotPrimitive = errors.New("stream is not a primitive stream")
//)
//
//func PrimitiveStreamFromStream(stream Stream) (*PrimitiveStream, error) {
//	return &PrimitiveStream{
//		Stream: stream,
//	}, nil
//}
//
//func LoadPrimitiveStream(options NewStreamOptions) (*PrimitiveStream, error) {
//	stream, err := LoadStream(options)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//	return PrimitiveStreamFromStream(*stream)
//}
//
//// checkValidPrimitiveStream checks if the stream is a valid primitive stream
//// and returns an error if it is not. Valid means:
//// - the stream is initialized
//// - the stream is a primitive stream
//func (p *PrimitiveStream) checkValidPrimitiveStream(ctx context.Context) error {
//	// first check if is initialized
//	err := p.checkInitialized(ctx)
//	if err != nil {
//		return errors.WithStack(err)
//	}
//
//	// then check if is primitive
//	streamType, err := p.GetType(ctx)
//	if err != nil {
//		return errors.WithStack(err)
//	}
//
//	if streamType != types.StreamTypePrimitive {
//		return ErrorStreamNotPrimitive
//	}
//
//	return nil
//}
//
//func (p *PrimitiveStream) checkedExecute(ctx context.Context, method string, args [][]any, opts ...client.TxOpt) (kwiltypes.Hash, error) {
//	err := p.checkValidPrimitiveStream(ctx)
//	if err != nil {
//		return kwiltypes.Hash{}, errors.WithStack(err)
//	}
//
//	return p._client.Execute(ctx, p.DBID, method, args, opts...)
//}
//
//func (p *PrimitiveStream) InsertRecords(ctx context.Context, inputs []types.InsertRecordInput, opts ...client.TxOpt) (kwiltypes.Hash, error) {
//	err := p.checkValidPrimitiveStream(ctx)
//	if err != nil {
//		return kwiltypes.Hash{}, errors.WithStack(err)
//	}
//
//	var args [][]any
//	for _, input := range inputs {
//
//		dateStr := input.DateValue.String()
//
//		args = append(args, []any{
//			dateStr,
//			fmt.Sprintf("%f", input.Value),
//		})
//	}
//
//	return p.checkedExecute(ctx, "insert_record", args, opts...)
//}
//
//func (p *PrimitiveStream) InsertRecordsUnix(ctx context.Context, inputs []types.InsertRecordUnixInput, opts ...client.TxOpt) (kwiltypes.Hash, error) {
//	err := p.checkValidPrimitiveStream(ctx)
//	if err != nil {
//		return kwiltypes.Hash{}, errors.WithStack(err)
//	}
//
//	var args [][]any
//	for _, input := range inputs {
//
//		dateStr := input.DateValue
//
//		args = append(args, []any{
//			dateStr,
//			fmt.Sprintf("%f", input.Value),
//		})
//	}
//
//	return p.checkedExecute(ctx, "insert_record", args, opts...)
//}
//
//func (p *PrimitiveStream) GetFirstRecordUnix(ctx context.Context, input types.GetFirstRecordUnixInput) (*types.StreamRecordUnix, error) {
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
//		DateValue: rawOutput.DateValue,
//		Value:     *value,
//	}, nil
//}
