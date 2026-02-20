package contractsapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

// GetWalletBalance retrieves the wallet balance for a specific bridge instance
func (s *Action) GetWalletBalance(bridgeIdentifier string, walletAddress string) (string, error) {
	if bridgeIdentifier == "" {
		return "", errors.New("bridge identifier is required")
	}
	if walletAddress == "" {
		return "", errors.New("wallet address is required")
	}

	ctx := context.Background()
	actionName := bridgeIdentifier + "_wallet_balance"

	res, err := s.call(ctx, actionName, []any{walletAddress})
	if err != nil {
		return "", err
	}

	if len(res.Values) == 0 {
		return "0", nil
	}

	return fmt.Sprint(res.Values[0][0]), nil
}

// Withdraw performs a withdrawal operation by bridging tokens from TN to a destination chain
func (s *Action) Withdraw(bridgeIdentifier string, amount string, recipient string) (string, error) {
	if bridgeIdentifier == "" {
		return "", errors.New("bridge identifier is required")
	}
	if amount == "" {
		return "", errors.New("amount is required")
	}
	// Validate amount is a valid decimal
	if _, _, err := apd.NewFromString(amount); err != nil {
		return "", fmt.Errorf("invalid amount format: %w", err)
	}

	if recipient == "" {
		return "", errors.New("recipient address is required")
	}

	ctx := context.Background()
	actionName := bridgeIdentifier + "_bridge_tokens"

	// Arguments for the action: $recipient, $amount
	args := []any{recipient, amount}

	txHash, err := s.execute(ctx, actionName, [][]any{args})
	if err != nil {
		return "", err
	}

	return txHash.String(), nil
}
