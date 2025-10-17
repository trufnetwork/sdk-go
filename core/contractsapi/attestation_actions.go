package contractsapi

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/trufnetwork/kwil-db/core/gatewayclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

// AttestationAction implements attestation-related actions
type AttestationAction struct {
	_client *gatewayclient.GatewayClient
}

var _ types.IAttestationAction = (*AttestationAction)(nil)

// AttestationActionOptions contains options for creating an AttestationAction
type AttestationActionOptions struct {
	Client *gatewayclient.GatewayClient
}

// LoadAttestationActions creates a new attestation action handler
func LoadAttestationActions(opts AttestationActionOptions) (types.IAttestationAction, error) {
	if opts.Client == nil {
		return nil, fmt.Errorf("kwil client is required")
	}
	return &AttestationAction{
		_client: opts.Client,
	}, nil
}

// RequestAttestation submits a request for a signed attestation of query results
func (a *AttestationAction) RequestAttestation(
	ctx context.Context,
	input types.RequestAttestationInput,
) (*types.RequestAttestationResult, error) {
	// Validate inputs
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	// Encode arguments using the canonical format
	argsBytes, err := EncodeActionArgs(input.Args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode action args")
	}

	// Prepare execute arguments
	// The request_attestation action expects:
	// ($data_provider TEXT, $stream_id TEXT, $action_name TEXT, $args_bytes BYTEA, $encrypt_sig BOOLEAN, $max_fee INT8)
	args := [][]any{
		{
			input.DataProvider,
			input.StreamID,
			input.ActionName,
			argsBytes,
			input.EncryptSig,
			input.MaxFee,
		},
	}

	// Execute the action
	txHash, err := a._client.Execute(ctx, "", "request_attestation", args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request_attestation")
	}

	result := &types.RequestAttestationResult{
		RequestTxID: txHash.String(),
	}

	return result, nil
}

// GetSignedAttestation retrieves a complete signed attestation payload
func (a *AttestationAction) GetSignedAttestation(
	ctx context.Context,
	input types.GetSignedAttestationInput,
) (*types.SignedAttestationResult, error) {
	if input.RequestTxID == "" {
		return nil, fmt.Errorf("request_tx_id cannot be empty")
	}

	// Call the get_signed_attestation view action
	// The action expects: ($request_tx_id TEXT)
	args := []any{input.RequestTxID}

	callResult, err := a._client.Call(ctx, "", "get_signed_attestation", args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call get_signed_attestation")
	}

	if callResult == nil {
		return nil, errors.New("get_signed_attestation returned nil response")
	}

	if callResult.Error != nil {
		return nil, errors.Errorf("get_signed_attestation returned error: %s", *callResult.Error)
	}

	if callResult.QueryResult == nil {
		return nil, errors.New("get_signed_attestation returned nil QueryResult")
	}

	// Extract the payload from the result
	// The action returns a single row with a single column (payload BYTEA)
	if len(callResult.QueryResult.Values) == 0 {
		return nil, fmt.Errorf("no attestation found for request_tx_id: %s", input.RequestTxID)
	}

	row := callResult.QueryResult.Values[0]
	if len(row) == 0 {
		return nil, fmt.Errorf("empty result row")
	}

	// Extract the payload bytes using the helper function (handles PostgreSQL BYTEA format)
	var payload []byte
	if err := extractBytesColumn(row[0], &payload, 0, "payload"); err != nil {
		return nil, err
	}

	return &types.SignedAttestationResult{
		Payload: payload,
	}, nil
}

// ListAttestations returns metadata for attestations, optionally filtered
func (a *AttestationAction) ListAttestations(
	ctx context.Context,
	input types.ListAttestationsInput,
) ([]types.AttestationMetadata, error) {
	// Set defaults
	limit := 5000
	if input.Limit != nil {
		if *input.Limit <= 0 || *input.Limit > 5000 {
			return nil, fmt.Errorf("limit must be between 1 and 5000")
		}
		limit = *input.Limit
	}

	offset := 0
	if input.Offset != nil {
		if *input.Offset < 0 {
			return nil, fmt.Errorf("offset must be non-negative")
		}
		offset = *input.Offset
	}

	// Validate requester length (must be 20 bytes max)
	if input.Requester != nil && len(input.Requester) > 20 {
		return nil, fmt.Errorf("requester must be at most 20 bytes, got %d bytes", len(input.Requester))
	}

	// Whitelist allowed OrderBy values to prevent SQL injection
	var orderBy *string
	if input.OrderBy != nil {
		allowedOrderBy := map[string]bool{
			"created_height ASC":  true,
			"created_height DESC": true,
			"created_height asc":  true,
			"created_height desc": true,
			"signed_height ASC":   true,
			"signed_height DESC":  true,
			"signed_height asc":   true,
			"signed_height desc":  true,
		}
		if !allowedOrderBy[*input.OrderBy] {
			return nil, fmt.Errorf("order_by must be one of: created_height ASC/DESC, signed_height ASC/DESC")
		}
		orderBy = input.OrderBy
	}

	// Prepare call arguments
	// The action expects: ($requester BYTEA, $limit INT, $offset INT, $order_by TEXT)
	args := []any{
		input.Requester, // Can be nil
		limit,
		offset,
		orderBy, // Use validated orderBy or nil for default
	}

	// Call the view action
	callResult, err := a._client.Call(ctx, "", "list_attestations", args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call list_attestations")
	}

	if callResult == nil {
		return nil, errors.New("list_attestations returned nil response")
	}

	if callResult.Error != nil {
		return nil, errors.Errorf("list_attestations returned error: %s", *callResult.Error)
	}

	if callResult.QueryResult == nil {
		return nil, errors.New("list_attestations returned nil QueryResult")
	}

	// Parse the result rows
	// Expected columns: request_tx_id, attestation_hash, requester, created_height, signed_height, encrypt_sig
	results := make([]types.AttestationMetadata, 0, len(callResult.QueryResult.Values))

	for i, row := range callResult.QueryResult.Values {
		if len(row) < 6 {
			return nil, fmt.Errorf("row %d has insufficient columns: expected 6, got %d", i, len(row))
		}

		metadata := types.AttestationMetadata{}

		// Column 0: request_tx_id (TEXT)
		if requestTxID, ok := row[0].(string); ok {
			metadata.RequestTxID = requestTxID
		} else if requestTxIDPtr, ok := row[0].(*string); ok && requestTxIDPtr != nil {
			metadata.RequestTxID = *requestTxIDPtr
		} else {
			return nil, fmt.Errorf("row %d: unexpected request_tx_id type: %T", i, row[0])
		}

		// Column 1: attestation_hash (BYTEA)
		if err := extractBytesColumn(row[1], &metadata.AttestationHash, i, "attestation_hash"); err != nil {
			return nil, err
		}

		// Column 2: requester (BYTEA)
		if err := extractBytesColumn(row[2], &metadata.Requester, i, "requester"); err != nil {
			return nil, err
		}

		// Column 3: created_height (INT8)
		if err := extractInt64Column(row[3], &metadata.CreatedHeight, i, "created_height"); err != nil {
			return nil, err
		}

		// Column 4: signed_height (INT8, nullable)
		if row[4] != nil {
			var signedHeight int64
			if err := extractInt64Column(row[4], &signedHeight, i, "signed_height"); err != nil {
				return nil, err
			}
			metadata.SignedHeight = &signedHeight
		}

		// Column 5: encrypt_sig (BOOLEAN)
		if encryptSig, ok := row[5].(bool); ok {
			metadata.EncryptSig = encryptSig
		} else if encryptSigPtr, ok := row[5].(*bool); ok && encryptSigPtr != nil {
			metadata.EncryptSig = *encryptSigPtr
		} else {
			return nil, fmt.Errorf("row %d: unexpected encrypt_sig type: %T", i, row[5])
		}

		results = append(results, metadata)
	}

	return results, nil
}

// Helper function to extract bytes from a column
func extractBytesColumn(value any, dest *[]byte, rowIdx int, colName string) error {
	if value == nil {
		*dest = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		// Copy to avoid aliasing driver-backed buffers
		*dest = append([]byte(nil), v...)
	case *[]byte:
		if v != nil {
			// Copy to avoid aliasing driver-backed buffers
			*dest = append([]byte(nil), *v...)
		} else {
			*dest = nil
		}
	case string:
		// Handle PostgreSQL BYTEA hex format (\xHHHH...)
		if len(v) >= 2 && v[0:2] == "\\x" {
			// Remove \x prefix and decode hex
			decoded, err := hex.DecodeString(v[2:])
			if err != nil {
				return fmt.Errorf("row %d: failed to decode %s as hex (\\x format): %w", rowIdx, colName, err)
			}
			*dest = decoded
		} else if decoded, err := hex.DecodeString(v); err == nil {
			// Plain hex decode succeeded
			*dest = decoded
		} else {
			// Not valid hex - fail fast
			return fmt.Errorf("row %d: %s not valid hex", rowIdx, colName)
		}
	case *string:
		if v != nil {
			// Handle PostgreSQL BYTEA hex format (\xHHHH...)
			if len(*v) >= 2 && (*v)[0:2] == "\\x" {
				// Remove \x prefix and decode hex
				decoded, err := hex.DecodeString((*v)[2:])
				if err != nil {
					return fmt.Errorf("row %d: failed to decode %s as hex (\\x format): %w", rowIdx, colName, err)
				}
				*dest = decoded
			} else if decoded, err := hex.DecodeString(*v); err == nil {
				// Plain hex decode succeeded
				*dest = decoded
			} else {
				// Not valid hex - fail fast
				return fmt.Errorf("row %d: %s not valid hex", rowIdx, colName)
			}
		} else {
			*dest = nil
		}
	default:
		return fmt.Errorf("row %d: unexpected %s type: %T", rowIdx, colName, value)
	}

	return nil
}

// Helper function to extract int64 from a column
func extractInt64Column(value any, dest *int64, rowIdx int, colName string) error {
	switch v := value.(type) {
	case int64:
		*dest = v
	case *int64:
		if v != nil {
			*dest = *v
		} else {
			return fmt.Errorf("row %d: %s is null", rowIdx, colName)
		}
	case int:
		*dest = int64(v)
	case *int:
		if v != nil {
			*dest = int64(*v)
		} else {
			return fmt.Errorf("row %d: %s is null", rowIdx, colName)
		}
	case int32:
		*dest = int64(v)
	case *int32:
		if v != nil {
			*dest = int64(*v)
		} else {
			return fmt.Errorf("row %d: %s is null", rowIdx, colName)
		}
	case string:
		// Parse string as int64
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("row %d: failed to parse %s as int64: %w", rowIdx, colName, err)
		}
		*dest = n
	case *string:
		if v != nil {
			n, err := strconv.ParseInt(*v, 10, 64)
			if err != nil {
				return fmt.Errorf("row %d: failed to parse %s as int64: %w", rowIdx, colName, err)
			}
			*dest = n
		} else {
			return fmt.Errorf("row %d: %s is null", rowIdx, colName)
		}
	default:
		return fmt.Errorf("row %d: unexpected %s type: %T", rowIdx, colName, value)
	}

	return nil
}
