package contractsapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/trufnetwork/kwil-db/core/gatewayclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

// TransactionAction implements transaction ledger query methods
type TransactionAction struct {
	_client *gatewayclient.GatewayClient
}

var _ types.ITransactionAction = (*TransactionAction)(nil)

// TransactionActionOptions contains options for creating a TransactionAction
type TransactionActionOptions struct {
	Client *gatewayclient.GatewayClient
}

// LoadTransactionActions creates a new transaction action handler
func LoadTransactionActions(opts TransactionActionOptions) (types.ITransactionAction, error) {
	if opts.Client == nil {
		return nil, fmt.Errorf("kwil client is required")
	}
	return &TransactionAction{
		_client: opts.Client,
	}, nil
}

// GetTransactionEvent fetches detailed transaction information by tx hash
//
// This method queries the transaction ledger for a specific transaction and returns
// comprehensive information including fee details and distributions.
//
// Example:
//
//	txEvent, err := txActions.GetTransactionEvent(ctx, types.GetTransactionEventInput{
//	    TxID: "0xabcdef123456...",
//	})
func (t *TransactionAction) GetTransactionEvent(
	ctx context.Context,
	input types.GetTransactionEventInput,
) (*types.TransactionEvent, error) {
	// Validate input
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	// Call the view action
	// Action signature: get_transaction_event($tx_id TEXT)
	// Returns: tx_id, block_height, method, caller, fee_amount, fee_recipient, metadata, fee_distributions
	args := []any{input.TxID}
	callResult, err := t._client.Call(ctx, "", "get_transaction_event", args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call get_transaction_event")
	}

	if callResult == nil {
		return nil, errors.New("get_transaction_event returned nil response")
	}

	if callResult.Error != nil {
		return nil, errors.Errorf("get_transaction_event returned error: %s", *callResult.Error)
	}

	if callResult.QueryResult == nil {
		return nil, errors.New("get_transaction_event returned nil QueryResult")
	}

	// Check if transaction exists
	if len(callResult.QueryResult.Values) == 0 {
		return nil, fmt.Errorf("transaction not found: %s", input.TxID)
	}

	row := callResult.QueryResult.Values[0]
	if len(row) < 8 {
		return nil, fmt.Errorf("invalid result: expected 8 columns, got %d", len(row))
	}

	// Parse the result row
	// Columns: tx_id, block_height, method, caller, fee_amount, fee_recipient, metadata, fee_distributions
	event := &types.TransactionEvent{}

	// Column 0: tx_id (TEXT)
	if txID, ok := row[0].(string); ok {
		event.TxID = txID
	} else {
		return nil, fmt.Errorf("invalid tx_id type: %T", row[0])
	}

	// Column 1: block_height (INT8)
	// Gateway returns INT8 as string in JSON
	if err := extractInt64Column(row[1], &event.BlockHeight, 0, "block_height"); err != nil {
		return nil, err
	}

	// Column 2: method (TEXT)
	if method, ok := row[2].(string); ok {
		event.Method = method
	} else {
		return nil, fmt.Errorf("invalid method type: %T", row[2])
	}

	// Column 3: caller (TEXT)
	if caller, ok := row[3].(string); ok {
		event.Caller = caller
	} else {
		return nil, fmt.Errorf("invalid caller type: %T", row[3])
	}

	// Column 4: fee_amount (NUMERIC(78,0) as string)
	// The gateway returns NUMERIC as string to preserve precision
	if feeAmount, ok := row[4].(string); ok {
		event.FeeAmount = feeAmount
	} else {
		return nil, fmt.Errorf("invalid fee_amount type: %T", row[4])
	}

	// Column 5: fee_recipient (TEXT, nullable)
	if row[5] != nil {
		if recipient, ok := row[5].(string); ok {
			event.FeeRecipient = &recipient
		} else {
			return nil, fmt.Errorf("invalid fee_recipient type: %T", row[5])
		}
	}

	// Column 6: metadata (TEXT, nullable)
	if row[6] != nil {
		if metadata, ok := row[6].(string); ok {
			event.Metadata = &metadata
		} else {
			return nil, fmt.Errorf("invalid metadata type: %T", row[6])
		}
	}

	// Column 7: fee_distributions (TEXT, comma-separated "recipient:amount")
	if row[7] != nil {
		if distStr, ok := row[7].(string); ok && distStr != "" {
			event.FeeDistributions = parseFeeDistributions(distStr)
		}
	}

	// If no distributions parsed, initialize empty slice
	if event.FeeDistributions == nil {
		event.FeeDistributions = []types.FeeDistribution{}
	}

	return event, nil
}

// ListTransactionFees returns transactions filtered by wallet and mode
//
// This method queries the transaction ledger for transactions involving a specific wallet,
// either as the payer or receiver of fees. Returns one row per fee distribution.
//
// Example:
//
//	limit := 20
//	entries, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
//	    Wallet: "0x1234...",
//	    Mode:   types.TransactionFeeModeBoth,
//	    Limit:  &limit,
//	})
func (t *TransactionAction) ListTransactionFees(
	ctx context.Context,
	input types.ListTransactionFeesInput,
) ([]types.TransactionFeeEntry, error) {
	// Validate input
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	// Set defaults
	limit := 20
	if input.Limit != nil {
		limit = *input.Limit
	}

	offset := 0
	if input.Offset != nil {
		offset = *input.Offset
	}

	// Call the view action
	// Action signature: list_transaction_fees($wallet TEXT, $mode TEXT, $limit INT, $offset INT)
	// Returns: tx_id, block_height, method, caller, total_fee, fee_recipient, metadata,
	//          distribution_sequence, distribution_recipient, distribution_amount
	args := []any{
		input.Wallet,
		string(input.Mode),
		limit,
		offset,
	}

	callResult, err := t._client.Call(ctx, "", "list_transaction_fees", args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call list_transaction_fees")
	}

	if callResult == nil {
		return nil, errors.New("list_transaction_fees returned nil response")
	}

	if callResult.Error != nil {
		return nil, errors.Errorf("list_transaction_fees returned error: %s", *callResult.Error)
	}

	if callResult.QueryResult == nil {
		return nil, errors.New("list_transaction_fees returned nil QueryResult")
	}

	// Parse result rows
	// Columns: tx_id, block_height, method, caller, total_fee, fee_recipient, metadata,
	//          distribution_sequence, distribution_recipient, distribution_amount
	entries := make([]types.TransactionFeeEntry, 0, len(callResult.QueryResult.Values))

	for i, row := range callResult.QueryResult.Values {
		if len(row) < 10 {
			return nil, fmt.Errorf("row %d has insufficient columns: expected 10, got %d", i, len(row))
		}

		entry := types.TransactionFeeEntry{}

		// Column 0: tx_id (TEXT)
		if txID, ok := row[0].(string); ok {
			entry.TxID = txID
		} else {
			return nil, fmt.Errorf("row %d: invalid tx_id type: %T", i, row[0])
		}

		// Column 1: block_height (INT8)
		if err := extractInt64Column(row[1], &entry.BlockHeight, i, "block_height"); err != nil {
			return nil, err
		}

		// Column 2: method (TEXT)
		if method, ok := row[2].(string); ok {
			entry.Method = method
		} else {
			return nil, fmt.Errorf("row %d: invalid method type: %T", i, row[2])
		}

		// Column 3: caller (TEXT)
		if caller, ok := row[3].(string); ok {
			entry.Caller = caller
		} else {
			return nil, fmt.Errorf("row %d: invalid caller type: %T", i, row[3])
		}

		// Column 4: total_fee (NUMERIC(78,0) as string)
		if totalFee, ok := row[4].(string); ok {
			entry.TotalFee = totalFee
		} else {
			return nil, fmt.Errorf("row %d: invalid total_fee type: %T", i, row[4])
		}

		// Column 5: fee_recipient (TEXT, nullable)
		if row[5] != nil {
			if recipient, ok := row[5].(string); ok {
				entry.FeeRecipient = &recipient
			} else {
				return nil, fmt.Errorf("row %d: invalid fee_recipient type: %T", i, row[5])
			}
		}

		// Column 6: metadata (TEXT, nullable)
		if row[6] != nil {
			if metadata, ok := row[6].(string); ok {
				entry.Metadata = &metadata
			} else {
				return nil, fmt.Errorf("row %d: invalid metadata type: %T", i, row[6])
			}
		}

		// Column 7: distribution_sequence (INT)
		if seqInt64, ok := row[7].(int64); ok {
			entry.DistributionSequence = int(seqInt64)
		} else if _, ok := row[7].(string); ok {
			// Sometimes gateway returns INT as string, try parsing
			var parsedSeq int64
			if err := extractInt64Column(row[7], &parsedSeq, i, "distribution_sequence"); err != nil {
				return nil, err
			}
			entry.DistributionSequence = int(parsedSeq)
		} else {
			return nil, fmt.Errorf("row %d: invalid distribution_sequence type: %T", i, row[7])
		}

		// Column 8: distribution_recipient (TEXT, nullable)
		if row[8] != nil {
			distRecipient, ok := row[8].(string)
			if !ok {
				return nil, fmt.Errorf("row %d: invalid distribution_recipient type: %T", i, row[8])
			}
			entry.DistributionRecipient = &distRecipient
		}

		// Column 9: distribution_amount (NUMERIC(78,0) as string, nullable)
		if row[9] != nil {
			distAmount, ok := row[9].(string)
			if !ok {
				return nil, fmt.Errorf("row %d: invalid distribution_amount type: %T", i, row[9])
			}
			entry.DistributionAmount = &distAmount
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// parseFeeDistributions parses "recipient1:amount1,recipient2:amount2" format
// Returns empty slice if input is empty
func parseFeeDistributions(distStr string) []types.FeeDistribution {
	if distStr == "" {
		return []types.FeeDistribution{}
	}

	parts := strings.Split(distStr, ",")
	distributions := make([]types.FeeDistribution, 0, len(parts))

	for _, part := range parts {
		// Trim whitespace
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		// Split only on first colon (in case addresses somehow have colons, though unlikely)
		colonIndex := strings.Index(trimmed, ":")
		if colonIndex == -1 {
			// Invalid format, skip this entry
			continue
		}

		recipient := trimmed[:colonIndex]
		amount := trimmed[colonIndex+1:]

		// Validate both parts are non-empty
		if recipient != "" && amount != "" {
			distributions = append(distributions, types.FeeDistribution{
				Recipient: recipient,
				Amount:    amount,
			})
		}
	}

	return distributions
}
