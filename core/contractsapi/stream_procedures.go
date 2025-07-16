package contractsapi

import (
	"context"
	"reflect"
	"strconv"
	"strings"

	"github.com/cockroachdb/apd/v3"
	"github.com/pkg/errors"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
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
	args = append(args, transformOrNil(input.UseCache, func(cache bool) any { return cache }))

	prefix := ""
	if input.Prefix != nil {
		prefix = *input.Prefix
	}

	results, err := s.call(ctx, prefix+"get_record", args)
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

func (s *Action) GetRecordWithMetadata(ctx context.Context, input types.GetRecordInput) (types.StreamRecordWithMetadata, error) {
	var args []any
	args = append(args, input.DataProvider)
	args = append(args, input.StreamId)
	args = append(args, transformOrNil(input.From, func(date int) any { return date }))
	args = append(args, transformOrNil(input.To, func(date int) any { return date }))
	args = append(args, transformOrNil(input.FrozenAt, func(date int) any { return date }))
	args = append(args, transformOrNil(input.UseCache, func(cache bool) any { return cache }))

	prefix := ""
	if input.Prefix != nil {
		prefix = *input.Prefix
	}

	callResult, err := s.callWithLogs(ctx, prefix+"get_record", args)
	if err != nil {
		return types.StreamRecordWithMetadata{}, errors.WithStack(err)
	}

	rawOutputs, err := DecodeCallResult[GetRecordRawOutput](callResult.QueryResult)
	if err != nil {
		return types.StreamRecordWithMetadata{}, errors.WithStack(err)
	}

	var outputs []types.StreamRecord
	for _, rawOutput := range rawOutputs {
		value, _, err := apd.NewFromString(rawOutput.Value)
		if err != nil {
			return types.StreamRecordWithMetadata{}, errors.WithStack(err)
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

	// Parse logs string into individual log lines for cache metadata extraction
	var logs []string
	if callResult.Logs != "" {
		lines := strings.Split(callResult.Logs, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				logs = append(logs, line)
			}
		}
	}

	// Parse cache metadata from logs
	metadata, err := types.ParseCacheMetadata(logs)
	if err != nil {
		return types.StreamRecordWithMetadata{}, errors.WithStack(err)
	}

	// Enhance metadata with query context
	metadata.StreamId = input.StreamId
	metadata.DataProvider = input.DataProvider
	if input.From != nil {
		from := int64(*input.From)
		metadata.From = &from
	}
	if input.To != nil {
		to := int64(*input.To)
		metadata.To = &to
	}
	if input.FrozenAt != nil {
		frozenAt := int64(*input.FrozenAt)
		metadata.FrozenAt = &frozenAt
	}
	metadata.RowsServed = len(outputs)

	return types.StreamRecordWithMetadata{
		Records:  outputs,
		Metadata: metadata,
	}, nil
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
	args = append(args, transformOrNil(input.UseCache, func(cache bool) any { return cache }))

	prefix := ""
	if input.Prefix != nil {
		prefix = *input.Prefix
	}

	results, err := s.call(ctx, prefix+"get_index", args)
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

func (s *Action) GetIndexWithMetadata(ctx context.Context, input types.GetIndexInput) (types.StreamIndexWithMetadata, error) {
	var args []any
	args = append(args, input.DataProvider)
	args = append(args, input.StreamId)
	args = append(args, transformOrNil(input.From, func(date int) any { return date }))
	args = append(args, transformOrNil(input.To, func(date int) any { return date }))
	args = append(args, transformOrNil(input.FrozenAt, func(date int) any { return date }))
	args = append(args, transformOrNil(input.BaseDate, func(date int) any { return date }))
	args = append(args, transformOrNil(input.UseCache, func(cache bool) any { return cache }))

	prefix := ""
	if input.Prefix != nil {
		prefix = *input.Prefix
	}

	callResult, err := s.callWithLogs(ctx, prefix+"get_index", args)
	if err != nil {
		return types.StreamIndexWithMetadata{}, errors.WithStack(err)
	}

	rawOutputs, err := DecodeCallResult[GetIndexRawOutput](callResult.QueryResult)
	if err != nil {
		return types.StreamIndexWithMetadata{}, errors.WithStack(err)
	}

	var outputs []types.StreamIndex
	for _, rawOutput := range rawOutputs {
		value, _, err := apd.NewFromString(rawOutput.Value)
		if err != nil {
			return types.StreamIndexWithMetadata{}, errors.WithStack(err)
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

	// Parse logs string into individual log lines for cache metadata extraction
	var logs []string
	if callResult.Logs != "" {
		lines := strings.Split(callResult.Logs, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				logs = append(logs, line)
			}
		}
	}

	// Parse cache metadata from logs
	metadata, err := types.ParseCacheMetadata(logs)
	if err != nil {
		return types.StreamIndexWithMetadata{}, errors.WithStack(err)
	}

	// Enhance metadata with query context
	metadata.StreamId = input.StreamId
	metadata.DataProvider = input.DataProvider
	metadata.RowsServed = len(outputs)
	
	if input.From != nil {
		from := int64(*input.From)
		metadata.From = &from
	}
	if input.To != nil {
		to := int64(*input.To)
		metadata.To = &to
	}
	if input.FrozenAt != nil {
		frozenAt := int64(*input.FrozenAt)
		metadata.FrozenAt = &frozenAt
	}

	return types.StreamIndexWithMetadata{
		Indices:  outputs,
		Metadata: metadata,
	}, nil
}

type GetIndexChangeRawOutput = GetRecordRawOutput

func (s *Action) GetIndexChange(ctx context.Context, input types.GetIndexChangeInput) ([]types.StreamIndexChange, error) {
	var args []any
	args = append(args, input.DataProvider)
	args = append(args, input.StreamId)
	args = append(args, transformOrNil(input.From, func(date int) any { return date }))
	args = append(args, transformOrNil(input.To, func(date int) any { return date }))
	args = append(args, transformOrNil(input.FrozenAt, func(date int) any { return date }))
	args = append(args, transformOrNil(input.BaseDate, func(date int) any { return date }))
	args = append(args, input.TimeInterval)

	prefix := ""
	if input.Prefix != nil {
		prefix = *input.Prefix
	}

	results, err := s.call(ctx, prefix+"get_index_change", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rawOutputs, err := DecodeCallResult[GetIndexChangeRawOutput](results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var outputs []types.StreamIndexChange
	for _, rawOutput := range rawOutputs {
		value, _, err := apd.NewFromString(rawOutput.Value)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		outputs = append(outputs, types.StreamIndexChange{
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

// streamExistsResult is used to decode the output of the stream_exists_batch procedure.
// Note: The exact JSON tags will depend on the actual output of the SQL procedure.
// Assuming it returns columns named data_provider, stream_id, and exists.
type streamExistsResult struct {
	DataProvider string `json:"data_provider"`
	StreamId     string `json:"stream_id"`
	Exists       bool   `json:"stream_exists"`
}

// BatchStreamExists checks for the existence of multiple streams using the stream_exists_batch SQL action.
func (s *Action) BatchStreamExists(ctx context.Context, streamsInput []types.StreamLocator) ([]types.StreamExistsResult, error) {
	if len(streamsInput) == 0 {
		return []types.StreamExistsResult{}, nil
	}

	dataProviders := make([]string, len(streamsInput))
	streamIds := make([]string, len(streamsInput))

	for i, si := range streamsInput {
		dataProviders[i] = si.DataProvider.Address()
		streamIds[i] = si.StreamId.String()
	}

	// The procedure stream_exists_batch expects two array arguments: $data_providers TEXT[], $stream_ids TEXT[]
	args := []any{dataProviders, streamIds}

	queryResult, err := s.call(ctx, "stream_exists_batch", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	decodedResults, err := DecodeCallResult[streamExistsResult](queryResult)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resultsMap := make([]types.StreamExistsResult, len(decodedResults))
	for i, res := range decodedResults {
		dataProviderAddr, err := util.NewEthereumAddressFromString(res.DataProvider)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse data provider address '%s' from stream_exists_batch result", res.DataProvider)
		}
		streamIdObj, err := util.NewStreamId(res.StreamId)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse stream id '%s' from stream_exists_batch result", res.StreamId)
		}
		resultsMap[i] = types.StreamExistsResult{
			StreamLocator: types.StreamLocator{
				DataProvider: dataProviderAddr,
				StreamId:     *streamIdObj,
			},
			Exists: res.Exists,
		}
	}

	return resultsMap, nil
}

// filteredStreamResult is used to decode the output of the filter_streams_by_existence procedure.
// It expects columns data_provider and stream_id.
type filteredStreamResult struct {
	DataProvider string `json:"data_provider"`
	StreamId     string `json:"stream_id"`
}

// BatchFilterStreamsByExistence filters a list of streams based on their existence in the database.
// The existingOnly flag determines whether to return streams that exist or streams that do not exist.
func (s *Action) BatchFilterStreamsByExistence(ctx context.Context, streamsInput []types.StreamLocator, returnExisting bool) ([]types.StreamLocator, error) {
	if len(streamsInput) == 0 {
		return []types.StreamLocator{}, nil
	}

	dataProviders := make([]string, len(streamsInput))
	streamIds := make([]string, len(streamsInput))

	for i, sl := range streamsInput {
		dataProviders[i] = sl.DataProvider.Address()
		streamIds[i] = sl.StreamId.String()
	}

	args := []any{dataProviders, streamIds, returnExisting}

	queryResult, err := s.call(ctx, "filter_streams_by_existence", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	decodedResults, err := DecodeCallResult[filteredStreamResult](queryResult)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	filteredLocators := make([]types.StreamLocator, 0, len(decodedResults))
	for _, res := range decodedResults {
		dataProviderAddr, err := util.NewEthereumAddressFromString(res.DataProvider)
		if err != nil {
			// This might happen if the SQL procedure returns an invalid address string.
			// Consider how to handle this - skip, error out, or log.
			// For now, wrapping the error and returning.
			return nil, errors.Wrapf(err, "failed to parse data provider address '%s' from filter_streams_by_existence result", res.DataProvider)
		}
		streamIdObj, err := util.NewStreamId(res.StreamId)
		if err != nil {
			// Similar handling for invalid stream ID string.
			return nil, errors.Wrapf(err, "failed to parse stream id '%s' from filter_streams_by_existence result", res.StreamId)
		}

		filteredLocators = append(filteredLocators, types.StreamLocator{
			DataProvider: dataProviderAddr,
			StreamId:     *streamIdObj,
		})
	}

	return filteredLocators, nil
}

func (s *Action) GetFirstRecord(ctx context.Context, input types.GetFirstRecordInput) (*types.StreamRecord, error) {
	var args []any
	args = append(args, input.DataProvider)
	args = append(args, input.StreamId)
	args = addArgOrNull(args, input.After, true)
	args = addArgOrNull(args, input.FrozenAt, true)
	args = append(args, transformOrNil(input.UseCache, func(cache bool) any { return cache }))

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
		Value:     *value,
	}, nil
}

func (s *Action) GetFirstRecordWithMetadata(ctx context.Context, input types.GetFirstRecordInput) (*types.StreamRecord, *types.CacheMetadata, error) {
	var args []any
	args = append(args, input.DataProvider)
	args = append(args, input.StreamId)
	args = addArgOrNull(args, input.After, true)
	args = addArgOrNull(args, input.FrozenAt, true)
	args = append(args, transformOrNil(input.UseCache, func(cache bool) any { return cache }))

	callResult, err := s.callWithLogs(ctx, "get_first_record", args)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	rawOutputs, err := DecodeCallResult[GetRecordRawOutput](callResult.QueryResult)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	if len(rawOutputs) == 0 {
		return nil, nil, nil
	}

	rawOutput := rawOutputs[0]
	value, _, err := apd.NewFromString(rawOutput.Value)
	if err != nil {
		return nil, nil, errors.WithStack(err)
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
		return nil, nil, err
	}

	// Parse logs string into individual log lines for cache metadata extraction
	var logs []string
	if callResult.Logs != "" {
		lines := strings.Split(callResult.Logs, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				logs = append(logs, line)
			}
		}
	}

	// Parse cache metadata from logs
	metadata, err := types.ParseCacheMetadata(logs)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	// Enhance metadata with query context
	metadata.StreamId = input.StreamId
	metadata.DataProvider = input.DataProvider
	metadata.RowsServed = 1
	if input.After != nil {
		after := int64(*input.After)
		metadata.From = &after
	}
	if input.FrozenAt != nil {
		frozenAt := int64(*input.FrozenAt)
		metadata.FrozenAt = &frozenAt
	}

	record := &types.StreamRecord{
		EventTime: eventTime,
		Value:     *value,
	}

	return record, &metadata, nil
}
