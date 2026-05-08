package contractsapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/trufnetwork/sdk-go/core/util"
)

// GetWalletBalance retrieves the wallet balance for a specific bridge instance
func (s *Action) GetWalletBalance(ctx context.Context, bridgeIdentifier string, walletAddress string) (string, error) {
	if bridgeIdentifier == "" {
		return "", errors.New("bridge identifier is required")
	}
	if walletAddress == "" {
		return "", errors.New("wallet address is required")
	}

	actionName := bridgeIdentifier + "_wallet_balance"

	res, err := s.call(ctx, actionName, []any{walletAddress})
	if err != nil {
		return "", err
	}

	if len(res.Values) == 0 || len(res.Values[0]) == 0 {
		return "0", nil
	}

	val := res.Values[0][0]
	if val == nil {
		return "0", nil
	}

	return fmt.Sprint(val), nil
}

// Withdraw performs a withdrawal operation by bridging tokens from TN to a destination chain
func (s *Action) Withdraw(ctx context.Context, bridgeIdentifier string, amount string, recipient string) (string, error) {
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

	actionName := bridgeIdentifier + "_bridge_tokens"

	// Arguments for the action: $recipient, $amount
	args := []any{recipient, amount}

	txHash, err := s.execute(ctx, actionName, [][]any{args})
	if err != nil {
		return "", err
	}

	return txHash.String(), nil
}

// Transfer sends TRUF (or USDC, on bridges that support it) from the caller
// to another in-network wallet. Binds to the on-chain action
// "<bridgeIdentifier>_transfer" — e.g. "eth_truf" → eth_truf_transfer (mainnet),
// "ethereum" → ethereum_transfer / "sepolia" → sepolia_transfer (dev/test).
//
// The caller pays a 1-token action fee on top of `amount`, in the same token
// as the bridge instance. Reverts if the caller balance is below
// (amount + 1 token).
func (s *Action) Transfer(ctx context.Context, bridgeIdentifier string, recipient string, amount string) (string, error) {
	if bridgeIdentifier == "" {
		return "", errors.New("bridge identifier is required")
	}
	if recipient == "" {
		return "", errors.New("recipient address is required")
	}
	if _, err := util.NewEthereumAddressFromString(recipient); err != nil {
		return "", fmt.Errorf("invalid recipient address format: %w", err)
	}
	if amount == "" {
		return "", errors.New("amount is required")
	}
	parsed, _, err := apd.NewFromString(amount)
	if err != nil {
		return "", fmt.Errorf("invalid amount format: %w", err)
	}
	// On-chain action enforces amount > 0; mirror it here to fail fast and
	// avoid spending a tx submission round-trip on guaranteed reverts.
	if parsed.Sign() <= 0 {
		return "", fmt.Errorf("amount must be > 0, got %s", amount)
	}

	actionName := bridgeIdentifier + "_transfer"

	// Action signature: ($to_address TEXT, $amount TEXT)
	args := []any{recipient, amount}

	txHash, err := s.execute(ctx, actionName, [][]any{args})
	if err != nil {
		return "", err
	}

	return txHash.String(), nil
}
