package tnclient

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cockroachdb/apd/v3"
	"github.com/pkg/errors"
	kwilClientType "github.com/trufnetwork/kwil-db/core/client/types"
	kwilType "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/kwil-db/node/types"
	tn_api "github.com/trufnetwork/sdk-go/core/contractsapi"
	clientType "github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// TransportAction implements IAction interface using the Transport abstraction.
// This allows actions to work with any transport (HTTP, CRE, etc.).
//
// MINIMAL IMPLEMENTATION: Only GetRecord is fully implemented.
// Other methods return "not implemented" errors since they're not needed by QuantAMM.
type TransportAction struct {
	transport Transport
}

var _ clientType.IAction = (*TransportAction)(nil)

// GetRecord reads the records of the stream within the given date range.
// This is a core method needed by QuantAMM for reading stream data.
func (a *TransportAction) GetRecord(ctx context.Context, input clientType.GetRecordInput) (clientType.ActionResult, error) {
	var args []any
	args = append(args, input.DataProvider)
	args = append(args, input.StreamId)
	args = append(args, util.TransformOrNil(input.From, func(date int) any { return date }))
	args = append(args, util.TransformOrNil(input.To, func(date int) any { return date }))
	args = append(args, util.TransformOrNil(input.FrozenAt, func(date int) any { return date }))
	args = append(args, util.TransformOrNil(input.BaseDate, func(date int) any { return date }))
	if input.UseCache != nil {
		args = append(args, *input.UseCache)
	}

	prefix := ""
	if input.Prefix != nil {
		prefix = *input.Prefix
	}

	result, err := a.transport.Call(ctx, "", prefix+"get_record", args)
	if err != nil {
		return clientType.ActionResult{}, errors.WithStack(err)
	}

	// Decode raw SQL output using shared type
	rawOutputs, err := tn_api.DecodeCallResult[clientType.GetRecordRawOutput](result.QueryResult)
	if err != nil {
		return clientType.ActionResult{}, errors.WithStack(err)
	}

	// Parse strings to proper types
	var outputs []clientType.StreamResult
	for _, rawOutput := range rawOutputs {
		value, _, err := apd.NewFromString(rawOutput.Value)
		if err != nil {
			return clientType.ActionResult{}, errors.WithStack(err)
		}

		eventTime := 0
		if rawOutput.EventTime != "" {
			eventTime, err = strconv.Atoi(rawOutput.EventTime)
			if err != nil {
				return clientType.ActionResult{}, errors.WithStack(err)
			}
		}

		outputs = append(outputs, clientType.StreamResult{
			EventTime: eventTime,
			Value:     *value,
		})
	}

	// Note: Cache metadata parsing is not implemented in this minimal version
	// as noted in TRANSPORT_IMPLEMENTATION_NOTES.md
	return clientType.ActionResult{Results: outputs}, nil
}

// Stub implementations for IAction methods not needed by QuantAMM.
// These return errors indicating they're not implemented for custom transports.

func (a *TransportAction) ExecuteProcedure(ctx context.Context, procedure string, args [][]any) (types.Hash, error) {
	return a.transport.Execute(ctx, "", procedure, args)
}

func (a *TransportAction) CallProcedure(ctx context.Context, procedure string, args []any) (*kwilType.QueryResult, error) {
	result, err := a.transport.Call(ctx, "", procedure, args)
	if err != nil {
		return nil, err
	}
	return result.QueryResult, nil
}

func (a *TransportAction) GetIndex(ctx context.Context, input clientType.GetIndexInput) (clientType.ActionResult, error) {
	return clientType.ActionResult{}, fmt.Errorf("GetIndex not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetIndexChange(ctx context.Context, input clientType.GetIndexChangeInput) (clientType.ActionResult, error) {
	return clientType.ActionResult{}, fmt.Errorf("GetIndexChange not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetType(ctx context.Context, locator clientType.StreamLocator) (clientType.StreamType, error) {
	return "", fmt.Errorf("GetType not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetFirstRecord(ctx context.Context, input clientType.GetFirstRecordInput) (clientType.ActionResult, error) {
	return clientType.ActionResult{}, fmt.Errorf("GetFirstRecord not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) SetReadVisibility(ctx context.Context, input clientType.VisibilityInput) (types.Hash, error) {
	return types.Hash{}, fmt.Errorf("SetReadVisibility not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetReadVisibility(ctx context.Context, locator clientType.StreamLocator) (*util.VisibilityEnum, error) {
	return nil, fmt.Errorf("GetReadVisibility not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) SetComposeVisibility(ctx context.Context, input clientType.VisibilityInput) (types.Hash, error) {
	return types.Hash{}, fmt.Errorf("SetComposeVisibility not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetComposeVisibility(ctx context.Context, locator clientType.StreamLocator) (*util.VisibilityEnum, error) {
	return nil, fmt.Errorf("GetComposeVisibility not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) AllowReadWallet(ctx context.Context, input clientType.ReadWalletInput) (types.Hash, error) {
	return types.Hash{}, fmt.Errorf("AllowReadWallet not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) DisableReadWallet(ctx context.Context, input clientType.ReadWalletInput) (types.Hash, error) {
	return types.Hash{}, fmt.Errorf("DisableReadWallet not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) AllowComposeStream(ctx context.Context, locator clientType.StreamLocator) (types.Hash, error) {
	return types.Hash{}, fmt.Errorf("AllowComposeStream not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) DisableComposeStream(ctx context.Context, locator clientType.StreamLocator) (types.Hash, error) {
	return types.Hash{}, fmt.Errorf("DisableComposeStream not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetStreamOwner(ctx context.Context, locator clientType.StreamLocator) ([]byte, error) {
	return nil, fmt.Errorf("GetStreamOwner not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetAllowedReadWallets(ctx context.Context, locator clientType.StreamLocator) ([]util.EthereumAddress, error) {
	return nil, fmt.Errorf("GetAllowedReadWallets not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetAllowedComposeStreams(ctx context.Context, locator clientType.StreamLocator) ([]clientType.StreamLocator, error) {
	return nil, fmt.Errorf("GetAllowedComposeStreams not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) SetDefaultBaseTime(ctx context.Context, input clientType.DefaultBaseTimeInput) (types.Hash, error) {
	return types.Hash{}, fmt.Errorf("SetDefaultBaseTime not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) BatchStreamExists(ctx context.Context, streams []clientType.StreamLocator) ([]clientType.StreamExistsResult, error) {
	return nil, fmt.Errorf("BatchStreamExists not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) BatchFilterStreamsByExistence(ctx context.Context, streams []clientType.StreamLocator, returnExisting bool) ([]clientType.StreamLocator, error) {
	return nil, fmt.Errorf("BatchFilterStreamsByExistence not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetHistory(ctx context.Context, input clientType.GetHistoryInput) ([]clientType.BridgeHistory, error) {
	return nil, fmt.Errorf("GetHistory not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetWalletBalance(bridgeIdentifier string, walletAddress string) (string, error) {
	return "", fmt.Errorf("GetWalletBalance not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) Withdraw(bridgeIdentifier string, amount string, recipient string) (string, error) {
	return "", fmt.Errorf("Withdraw not implemented for custom transports - use HTTP transport or implement if needed")
}

func (a *TransportAction) GetWithdrawalProof(ctx context.Context, input clientType.GetWithdrawalProofInput) ([]clientType.WithdrawalProof, error) {
	return nil, fmt.Errorf("GetWithdrawalProof not implemented for custom transports - use HTTP transport or implement if needed")
}

// TransportPrimitiveAction implements IPrimitiveAction interface using the Transport abstraction.
// This allows primitive stream actions to work with any transport (HTTP, CRE, etc.).
//
// MINIMAL IMPLEMENTATION: Only InsertRecords is fully implemented.
// Other methods return "not implemented" errors since they're not needed by QuantAMM.
type TransportPrimitiveAction struct {
	TransportAction
}

var _ clientType.IPrimitiveAction = (*TransportPrimitiveAction)(nil)

// InsertRecords inserts multiple records into primitive streams.
// This is a core method needed by QuantAMM for writing stream data.
func (p *TransportPrimitiveAction) InsertRecords(ctx context.Context, inputs []clientType.InsertRecordInput, opts ...kwilClientType.TxOpt) (types.Hash, error) {
	var (
		dataProviders []string
		streamIds     []string
		eventTimes    []int
		values        kwilType.DecimalArray
	)

	for _, input := range inputs {
		// Convert float64 to decimal with 36 precision and 18 scale
		valueNumeric, err := kwilType.ParseDecimalExplicit(strconv.FormatFloat(input.Value, 'f', -1, 64), 36, 18)
		if err != nil {
			return types.Hash{}, errors.WithStack(err)
		}

		dataProviders = append(dataProviders, input.DataProvider)
		streamIds = append(streamIds, input.StreamId)
		eventTimes = append(eventTimes, input.EventTime)
		values = append(values, valueNumeric)
	}

	return p.transport.Execute(ctx, "", "insert_records", [][]any{{
		dataProviders,
		streamIds,
		eventTimes,
		values,
	}}, opts...)
}

// InsertRecord inserts a single record - stub implementation
func (p *TransportPrimitiveAction) InsertRecord(ctx context.Context, input clientType.InsertRecordInput, opts ...kwilClientType.TxOpt) (types.Hash, error) {
	// Just delegate to InsertRecords
	return p.InsertRecords(ctx, []clientType.InsertRecordInput{input}, opts...)
}

// CheckValidPrimitiveStream checks if the stream is a valid primitive stream - stub implementation
func (p *TransportPrimitiveAction) CheckValidPrimitiveStream(ctx context.Context, locator clientType.StreamLocator) error {
	return fmt.Errorf("CheckValidPrimitiveStream not implemented for custom transports - use HTTP transport or implement if needed")
}
