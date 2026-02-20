package contractsapi

import (
	"context"

	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
)

// GetWithdrawalProof retrieves the proofs and signatures needed to claim a withdrawal on EVM.
func (s *Action) GetWithdrawalProof(ctx context.Context, input types.GetWithdrawalProofInput) ([]types.WithdrawalProof, error) {
	if input.BridgeIdentifier == "" {
		return nil, errors.New("bridge identifier is required")
	}
	if input.Wallet == "" {
		return nil, errors.New("wallet address is required")
	}

	actionName := input.BridgeIdentifier + "_get_withdrawal_proof"

	// Arguments for the action: $wallet_address
	args := []any{input.Wallet}

	res, err := s.call(ctx, actionName, args)
	if err != nil {
		return nil, err
	}

	return DecodeCallResult[types.WithdrawalProof](res)
}
