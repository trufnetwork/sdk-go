package contractsapi

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/trufnetwork/kwil-db/core/gatewayclient"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	kwilClientType "github.com/trufnetwork/kwil-db/core/client/types"
	"github.com/trufnetwork/sdk-go/core/types"
)

// OrderBook provides methods for interacting with the prediction market order book
type OrderBook struct {
	_client *gatewayclient.GatewayClient
}

// Compile-time check that OrderBook implements IOrderBook
var _ types.IOrderBook = (*OrderBook)(nil)

// NewOrderBookOptions contains options for creating an OrderBook instance
type NewOrderBookOptions struct {
	Client *gatewayclient.GatewayClient
}

// LoadOrderBook creates a new OrderBook instance with the given options
func LoadOrderBook(options NewOrderBookOptions) (types.IOrderBook, error) {
	if options.Client == nil {
		return nil, errors.New("kwil client is required")
	}
	return &OrderBook{
		_client: options.Client,
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// HELPER METHODS
// ═══════════════════════════════════════════════════════════════

// call wraps _client.Call for read operations
func (o *OrderBook) call(ctx context.Context, action string, args []any) (*kwiltypes.QueryResult, error) {
	callResult, err := o._client.Call(ctx, "", action, args)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to call %s", action)
	}
	if callResult == nil {
		return nil, fmt.Errorf("action %s returned nil result", action)
	}
	if callResult.Error != nil {
		return nil, fmt.Errorf("action %s returned error: %v", action, callResult.Error)
	}
	if callResult.QueryResult == nil {
		return nil, fmt.Errorf("action %s returned nil QueryResult", action)
	}
	return callResult.QueryResult, nil
}

// execute wraps _client.Execute for write operations
func (o *OrderBook) execute(ctx context.Context, action string, args [][]any,
	opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	return o._client.Execute(ctx, "", action, args, opts...)
}

// extractIntColumn extracts an int value from a query result column
// Handles int, int64, and string representations
func extractIntColumn(val any, target *int, colIndex int, colName string) error {
	switch v := val.(type) {
	case int:
		*target = v
	case int64:
		*target = int(v)
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid %s (column %d): cannot parse string to int: %w", colName, colIndex, err)
		}
		*target = parsed
	default:
		return fmt.Errorf("invalid %s type (column %d): %T", colName, colIndex, val)
	}
	return nil
}

// extractBoolColumn extracts a bool value from a query result column
func extractBoolColumn(val any, target *bool, colIndex int, colName string) error {
	switch v := val.(type) {
	case bool:
		*target = v
	case string:
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("invalid %s (column %d): cannot parse string to bool: %w", colName, colIndex, err)
		}
		*target = parsed
	default:
		return fmt.Errorf("invalid %s type (column %d): %T", colName, colIndex, val)
	}
	return nil
}

// extractStringColumn extracts a string value from a query result column
func extractStringColumn(val any, target *string, colIndex int, colName string) error {
	str, ok := val.(string)
	if !ok {
		return fmt.Errorf("invalid %s type (column %d): expected string, got %T", colName, colIndex, val)
	}
	*target = str
	return nil
}

// Note: extractInt64Column and extractBytesColumn are defined in attestation_actions.go
// and are available to all files in the contractsapi package
