package types

import (
	"context"
	"fmt"
)

// TransactionEvent represents a single transaction from the ledger
type TransactionEvent struct {
	TxID             string            // 0x-prefixed transaction hash
	BlockHeight      int64             // Block height when transaction was included
	Method           string            // Method name (e.g., "deployStream", "insertRecords")
	Caller           string            // Ethereum address of caller (lowercase, 0x-prefixed)
	FeeAmount        string            // Fee amount as string (NUMERIC(78,0) - big number)
	FeeRecipient     *string           // Primary fee recipient (nullable)
	Metadata         *string           // Optional metadata JSON (nullable)
	FeeDistributions []FeeDistribution // Parsed fee distributions
}

// FeeDistribution represents a single fee payment to a recipient
type FeeDistribution struct {
	Recipient string // Ethereum address (lowercase, 0x-prefixed)
	Amount    string // Amount as string (NUMERIC(78,0))
}

// GetTransactionEventInput is input for GetTransactionEvent
type GetTransactionEventInput struct {
	TxID string `validate:"required"` // Transaction hash (with or without 0x prefix)
}

// ListTransactionFeesInput is input for ListTransactionFees
type ListTransactionFeesInput struct {
	Wallet string             `validate:"required"` // Ethereum address to query
	Mode   TransactionFeeMode `validate:"required"` // Filter mode: paid, received, or both
	Limit  *int               // Optional limit (default 20, max 1000)
	Offset *int               // Optional offset for pagination (default 0)
}

// TransactionFeeMode specifies which transactions to return
type TransactionFeeMode string

const (
	TransactionFeeModePaid     TransactionFeeMode = "paid"     // Fees paid by wallet
	TransactionFeeModeReceived TransactionFeeMode = "received" // Fees received by wallet
	TransactionFeeModeBoth     TransactionFeeMode = "both"     // Both paid and received
)

// TransactionFeeEntry represents a transaction with fee distribution details
// Note: list_transaction_fees returns one row per fee distribution,
// so multiple rows may have the same TxID with different distribution details
type TransactionFeeEntry struct {
	TxID                  string  // Transaction hash
	BlockHeight           int64   // Block height
	Method                string  // Method name
	Caller                string  // Transaction caller
	TotalFee              string  // Total fee amount
	FeeRecipient          *string // Primary fee recipient (nullable)
	Metadata              *string // Optional metadata JSON (nullable)
	DistributionSequence  int     // Sequence number of this distribution
	DistributionRecipient string  // Recipient of this specific distribution
	DistributionAmount    string  // Amount for this specific distribution
}

// ITransactionAction defines transaction ledger query methods
type ITransactionAction interface {
	// GetTransactionEvent fetches detailed transaction information by tx hash
	GetTransactionEvent(ctx context.Context, input GetTransactionEventInput) (*TransactionEvent, error)

	// ListTransactionFees returns transactions filtered by wallet and mode
	// Returns raw fee distribution entries (one row per distribution)
	ListTransactionFees(ctx context.Context, input ListTransactionFeesInput) ([]TransactionFeeEntry, error)
}

// Validate validates GetTransactionEventInput
func (g *GetTransactionEventInput) Validate() error {
	if g.TxID == "" {
		return fmt.Errorf("tx_id is required")
	}
	return nil
}

// Validate validates ListTransactionFeesInput
func (l *ListTransactionFeesInput) Validate() error {
	if l.Wallet == "" {
		return fmt.Errorf("wallet is required")
	}

	// Validate mode
	validModes := map[TransactionFeeMode]bool{
		TransactionFeeModePaid:     true,
		TransactionFeeModeReceived: true,
		TransactionFeeModeBoth:     true,
	}
	if !validModes[l.Mode] {
		return fmt.Errorf("mode must be one of: paid, received, both")
	}

	// Validate limit if provided
	if l.Limit != nil {
		if *l.Limit <= 0 {
			return fmt.Errorf("limit must be positive")
		}
		if *l.Limit > 1000 {
			return fmt.Errorf("limit cannot exceed 1000")
		}
	}

	// Validate offset if provided
	if l.Offset != nil && *l.Offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}

	return nil
}
