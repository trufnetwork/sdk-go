package contractsapi

import (
	"context"

	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
)

// GetHistory retrieves the transaction history for a wallet on a specific bridge
func (s *Action) GetHistory(ctx context.Context, input types.GetHistoryInput) ([]types.BridgeHistory, error) {
	if input.BridgeIdentifier == "" {
		return nil, errors.New("bridge identifier is required")
	}
	if input.Wallet == "" {
		return nil, errors.New("wallet address is required")
	}

	limit := 20
	if input.Limit != nil {
		limit = *input.Limit
	}
	offset := 0
	if input.Offset != nil {
		offset = *input.Offset
	}

	actionName := input.BridgeIdentifier + "_get_history"

	// Arguments for the action: $wallet_address, $limit, $offset
	args := []any{input.Wallet, limit, offset}

	res, err := s.call(ctx, actionName, args)
	if err != nil {
		return nil, err
	}

	return DecodeCallResult[types.BridgeHistory](res)
}
