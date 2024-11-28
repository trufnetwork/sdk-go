package contractsapi

import (
	"context"
	"reflect"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/golang-sql/civil"
	"github.com/kwilteam/kwil-db/core/types/client"
	"github.com/kwilteam/kwil-db/core/types/transactions"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ## View only procedures

type getMetadataParams struct {
	Key        types.MetadataKey
	OnlyLatest bool
	// optional. Gets metadata with ref value equal to the given value
	Ref string
}

type getMetadataResult struct {
	RowId  string `json:"row_id"`
	ValueI int    `json:"value_i"`
	ValueB bool   `json:"value_b"`
	// TODO: uncomment when supported
	// ValueF    float64 `json:"value_f"`
	ValueS    string `json:"value_s"`
	ValueRef  string `json:"value_ref"`
	CreatedAt int    `json:"created_at"`
}

// GetValueByKey returns the value of the metadata by its key
// I.e. if we expect an int from `ComposeVisibility`, we can call this function
// to get `valueI` from the result
func (g getMetadataResult) GetValueByKey(t types.MetadataKey) (any, error) {
	metadataType := t.GetType()

	switch metadataType {
	case types.MetadataTypeInt:
		return g.ValueI, nil
	case types.MetadataTypeBool:
		return g.ValueB, nil
	case types.MetadataTypeString:
		return g.ValueS, nil
	case types.MetadataTypeRef:
		return g.ValueRef, nil
	default:
		return types.MetadataValue{}, errors.New("unsupported metadata type")
	}
}

// addArgOrNull adds a new argument to the list of arguments
// this helps us making it NULL if it's equal to its zero value
// The caveat is that we won't be able to pass the zero value of the type. Issues with this?
func addArgOrNull(oldArgs []any, newArg any, nullIfZero bool) []any {
	if nullIfZero && reflect.ValueOf(newArg).IsZero() {
		return append(oldArgs, nil)
	}

	return append(oldArgs, newArg)
}

func (s *Stream) getMetadata(ctx context.Context, params getMetadataParams) ([]getMetadataResult, error) {

	var args []any

	args = addArgOrNull(args, params.Key.String(), false)
	args = addArgOrNull(args, params.OnlyLatest, false)
	// just add null if ref is empty, because it's optional
	args = addArgOrNull(args, params.Ref, true)

	res, err := s.call(ctx, "get_metadata", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return DecodeCallResult[getMetadataResult](res)
}

// ## Write procedures

type metadataInput struct {
	Key   types.MetadataKey
	Value types.MetadataValue
}

func (s *Stream) batchInsertMetadata(ctx context.Context, inputs []metadataInput) (transactions.TxHash, error) {
	var tuples [][]any
	for _, input := range inputs {
		valType := input.Key.GetType()
		valStr, err := valType.StringFromValue(input.Value)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		tuples = append(tuples, []any{input.Key.String(), valStr, string(valType)})
	}

	return s.checkedExecute(ctx, "insert_metadata", tuples)
}

func (s *Stream) insertMetadata(ctx context.Context, key types.MetadataKey, value types.MetadataValue) (transactions.TxHash, error) {
	return s.batchInsertMetadata(ctx, []metadataInput{{key, value}})
}

func (s *Stream) disableMetadata(ctx context.Context, rowId string) (transactions.TxHash, error) {
	return s.checkedExecute(ctx, "disable_metadata", [][]any{{rowId}})
}

func (s *Stream) InitializeStream(ctx context.Context) (transactions.TxHash, error) {
	return s.execute(ctx, "init", nil)
}

// ExecuteProcedure is a wrapper around the execute function, just to be explicit that users can execute arbitrary procedures
func (s *Stream) ExecuteProcedure(ctx context.Context, procedure string, args [][]any) (transactions.TxHash, error) {
	return s.execute(ctx, procedure, args)
}

type GetRecordRawOutput struct {
	DateValue string `json:"date_value"`
	Value     string `json:"value"`
}

type GetRecordUnixRawOutput struct {
	DateValue int    `json:"date_value"`
	Value     string `json:"value"`
}

// transformOrNil returns nil if the value is nil, otherwise it applies the transform function to the value.
func transformOrNil[T any](value *T, transform func(T) any) any {
	if value == nil {
		return nil
	}
	return transform(*value)
}

// CallProcedure is a wrapper around the call function, just to be explicit that users can call arbitrary procedures
func (s *Stream) CallProcedure(ctx context.Context, procedure string, args []any) (*client.Records, error) {
	return s.call(ctx, procedure, args)
}

func (s *Stream) GetRecord(ctx context.Context, input types.GetRecordInput) ([]types.StreamRecord, error) {
	var args []any
	args = append(args, transformOrNil(input.DateFrom, func(date civil.Date) any { return date.String() }))
	args = append(args, transformOrNil(input.DateTo, func(date civil.Date) any { return date.String() }))
	args = append(args, transformOrNil(input.FrozenAt, func(date time.Time) any { return date.UTC().Format(time.RFC3339) }))

	results, err := s.call(ctx, "get_record", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rawOutputs, err := DecodeCallResult[GetRecordRawOutput](results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var outputs []types.StreamRecord
	for _, rawOutput := range rawOutputs {
		value, _, err := apd.NewFromString(rawOutput.Value)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		dateValue, err := civil.ParseDate(rawOutput.DateValue)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		outputs = append(outputs, types.StreamRecord{
			DateValue: dateValue,
			Value:     *value,
		})
	}

	return outputs, nil
}

func (s *Stream) GetRecordUnix(ctx context.Context, input types.GetRecordUnixInput) ([]types.StreamRecordUnix, error) {
	var args []any
	args = append(args, transformOrNil(input.DateFrom, func(date int) any { return date }))
	args = append(args, transformOrNil(input.DateTo, func(date int) any { return date }))
	args = append(args, transformOrNil(input.FrozenAt, func(date time.Time) any { return date.UTC().Format(time.RFC3339) }))

	results, err := s.call(ctx, "get_record", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rawOutputs, err := DecodeCallResult[GetRecordUnixRawOutput](results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var outputs []types.StreamRecordUnix
	for _, rawOutput := range rawOutputs {
		value, _, err := apd.NewFromString(rawOutput.Value)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		outputs = append(outputs, types.StreamRecordUnix{
			DateValue: rawOutput.DateValue,
			Value:     *value,
		})
	}

	return outputs, nil
}

type GetIndexRawOutput = GetRecordRawOutput
type GetIndexUnixRawOutput = GetRecordUnixRawOutput

func (s *Stream) GetIndex(ctx context.Context, input types.GetIndexInput) ([]types.StreamIndex, error) {
	var args []any
	args = append(args, transformOrNil(input.DateFrom, func(date civil.Date) any { return date.String() }))
	args = append(args, transformOrNil(input.DateTo, func(date civil.Date) any { return date.String() }))
	args = append(args, transformOrNil(input.FrozenAt, func(date time.Time) any { return date.UTC().Format(time.RFC3339) }))
	args = append(args, transformOrNil(input.BaseDate, func(date civil.Date) any { return date.String() }))

	results, err := s.call(ctx, "get_index", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rawOutputs, err := DecodeCallResult[GetIndexRawOutput](results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var outputs []types.StreamIndex
	for _, rawOutput := range rawOutputs {
		value, _, err := apd.NewFromString(rawOutput.Value)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		dateValue, err := civil.ParseDate(rawOutput.DateValue)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		outputs = append(outputs, types.StreamIndex{
			DateValue: dateValue,
			Value:     *value,
		})
	}

	return outputs, nil
}

func (s *Stream) GetIndexUnix(ctx context.Context, input types.GetIndexUnixInput) ([]types.StreamIndexUnix, error) {
	var args []any
	args = append(args, transformOrNil(input.DateFrom, func(date int) any { return date }))
	args = append(args, transformOrNil(input.DateTo, func(date int) any { return date }))
	args = append(args, transformOrNil(input.FrozenAt, func(date time.Time) any { return date.UTC().Format(time.RFC3339) }))
	args = append(args, transformOrNil(input.BaseDate, func(date int) any { return date }))

	results, err := s.call(ctx, "get_index", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rawOutputs, err := DecodeCallResult[GetIndexUnixRawOutput](results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var outputs []types.StreamIndexUnix
	for _, rawOutput := range rawOutputs {
		value, _, err := apd.NewFromString(rawOutput.Value)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		outputs = append(outputs, types.StreamIndexUnix{
			DateValue: rawOutput.DateValue,
			Value:     *value,
		})
	}

	return outputs, nil
}

// GetFirstRecord(ctx context.Context, input GetFirstRecordInput) (*StreamRecord, error)
func (s *Stream) GetFirstRecord(ctx context.Context, input types.GetFirstRecordInput) (*types.StreamRecord, error) {
	var args []any
	args = append(args, transformOrNil(input.AfterDate, func(date civil.Date) any { return date.String() }))
	args = append(args, transformOrNil(input.FrozenAt, func(date time.Time) any { return date.UTC().Format(time.RFC3339) }))

	results, err := s.call(ctx, "get_first_record", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rawOutputs, err := DecodeCallResult[GetRecordRawOutput](results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(rawOutputs) == 0 {
		return nil, ErrorRecordNotFound
	}

	rawOutput := rawOutputs[0]
	value, _, err := apd.NewFromString(rawOutput.Value)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	dateValue, err := civil.ParseDate(rawOutput.DateValue)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &types.StreamRecord{
		DateValue: dateValue,
		Value:     *value,
	}, nil
}
