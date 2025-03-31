package contractsapi

import (
	"context"
	"github.com/cockroachdb/apd/v3"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
	"reflect"
	"strconv"
)

// ## View only procedures

type getMetadataParams struct {
	Stream types.StreamLocator
	Key    types.MetadataKey
	// optional. Gets metadata with ref value equal to the given value
	Ref     string
	Limit   int
	Offset  int
	OrderBy string
}
type getMetadataResult struct {
	RowId     string `json:"row_id"`
	ValueI    string `json:"value_i"`
	ValueF    string `json:"value_f"`
	ValueB    string `json:"value_b"`
	ValueS    string `json:"value_s"`
	ValueRef  string `json:"value_ref"`
	CreatedAt string `json:"created_at"`
}

// getValueByKey returns the value of the metadata by its key
// I.e. if we expect an int from `ComposeVisibility`, we can call this function
// to get `valueI` from the result

func (g getMetadataResult) getValueByKey(t types.MetadataKey) (string, error) {
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
		return "", errors.New("unsupported metadata type")
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

func (s *Action) getMetadata(ctx context.Context, params getMetadataParams) ([]getMetadataResult, error) {
	var args []any

	args = append(args, params.Stream.DataProvider.Address())
	args = append(args, params.Stream.StreamId.String())
	args = addArgOrNull(args, params.Key.String(), false)
	// just add null if ref is empty, because it's optional
	args = addArgOrNull(args, params.Ref, true)
	args = addArgOrNull(args, params.Limit, true)
	args = addArgOrNull(args, params.Offset, true)
	args = addArgOrNull(args, params.OrderBy, true)

	res, err := s.call(ctx, "get_metadata", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return DecodeCallResult[getMetadataResult](res)
}

// ## Write procedures

type InsertMetadataInput struct {
	Stream types.StreamLocator
	Key    types.MetadataKey
	Value  types.MetadataValue
}

func (s *Action) insertMetadata(ctx context.Context, input InsertMetadataInput) (kwiltypes.Hash, error) {
	valType := input.Key.GetType()
	valStr, err := valType.StringFromValue(input.Value)
	if err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return s.execute(ctx, "insert_metadata", [][]any{
		{input.Stream.DataProvider.Address(), input.Stream.StreamId.String(), input.Key.String(), valStr, string(valType)},
	})
}

type DisableMetadataInput struct {
	Stream types.StreamLocator
	RowId  *kwiltypes.UUID
}

func (s *Action) disableMetadata(ctx context.Context, input DisableMetadataInput) (kwiltypes.Hash, error) {
	return s.execute(ctx, "disable_metadata", [][]any{{input.Stream.DataProvider.Address(), input.Stream.StreamId.String(), input.RowId}})
}

// ExecuteProcedure is a wrapper around the execute function, just to be explicit that users can execute arbitrary procedures
func (s *Action) ExecuteProcedure(ctx context.Context, procedure string, args [][]any) (kwiltypes.Hash, error) {
	return s.execute(ctx, procedure, args)
}

type GetRecordRawOutput struct {
	EventTime string `json:"event_time"`
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
func (s *Action) CallProcedure(ctx context.Context, procedure string, args []any) (*kwiltypes.QueryResult, error) {
	return s.call(ctx, procedure, args)
}

func (s *Action) GetRecord(ctx context.Context, input types.GetRecordInput) ([]types.StreamRecord, error) {
	var args []any
	args = append(args, input.DataProvider)
	args = append(args, input.StreamId)
	args = append(args, transformOrNil(input.From, func(date int) any { return date }))
	args = append(args, transformOrNil(input.To, func(date int) any { return date }))
	args = append(args, transformOrNil(input.FrozenAt, func(date int) any { return date }))

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
		outputs = append(outputs, types.StreamRecord{
			EventTime: func() int {
				if rawOutput.EventTime == "" {
					return 0
				}

				eventTime, err := strconv.Atoi(rawOutput.EventTime)
				if err != nil {
					return 0
				}

				return eventTime
			}(),
			Value: *value,
		})
	}

	return outputs, nil
}

type GetIndexRawOutput = GetRecordRawOutput

func (s *Action) GetIndex(ctx context.Context, input types.GetIndexInput) ([]types.StreamIndex, error) {
	var args []any
	args = append(args, input.DataProvider)
	args = append(args, input.StreamId)
	args = append(args, transformOrNil(input.From, func(date int) any { return date }))
	args = append(args, transformOrNil(input.To, func(date int) any { return date }))
	args = append(args, transformOrNil(input.FrozenAt, func(date int) any { return date }))
	args = append(args, transformOrNil(input.BaseDate, func(date int) any { return date }))

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
		outputs = append(outputs, types.StreamIndex{
			EventTime: func() int {
				if rawOutput.EventTime == "" {
					return 0
				}

				eventTime, err := strconv.Atoi(rawOutput.EventTime)
				if err != nil {
					return 0
				}

				return eventTime
			}(),
			Value: *value,
		})
	}

	return outputs, nil
}

func (s *Action) GetFirstRecord(ctx context.Context, input types.GetFirstRecordInput) (*types.StreamRecord, error) {
	var args []any
	args = append(args, input.DataProvider)
	args = append(args, input.StreamId)
	args = addArgOrNull(args, input.After, true)
	args = addArgOrNull(args, input.FrozenAt, true)

	results, err := s.call(ctx, "get_first_record", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rawOutputs, err := DecodeCallResult[GetRecordRawOutput](results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(rawOutputs) == 0 {
		return nil, nil
	}

	rawOutput := rawOutputs[0]
	value, _, err := apd.NewFromString(rawOutput.Value)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	eventTime, err := func() (int, error) {
		if rawOutput.EventTime == "" {
			return 0, nil
		}

		eventTime, err := strconv.Atoi(rawOutput.EventTime)
		if err != nil {
			return 0, errors.WithStack(err)
		}

			return eventTime, nil
	}()
	if err != nil {
		return nil, err
	}

	return &types.StreamRecord{
		EventTime: eventTime,
		Value: *value,
	}, nil
}
