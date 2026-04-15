package contractsapi

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/trufnetwork/kwil-db/core/gatewayclient"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
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

	// Parse MaxFee as NUMERIC(78,0)
	// Use empty string as default (NULL) if not provided
	var maxFeeValue any = nil
	if input.MaxFee != "" {
		maxFeeNumeric, err := kwiltypes.ParseDecimalExplicit(input.MaxFee, 78, 0)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse max_fee as NUMERIC(78,0)")
		}
		maxFeeValue = maxFeeNumeric
	}

	// Prepare execute arguments
	// The request_attestation action expects:
	// ($data_provider TEXT, $stream_id TEXT, $action_name TEXT, $args_bytes BYTEA, $encrypt_sig BOOLEAN, $max_fee NUMERIC(78,0))
	args := [][]any{
		{
			input.DataProvider,
			input.StreamID,
			input.ActionName,
			argsBytes,
			input.EncryptSig,
			maxFeeValue,
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
		// Gateway always returns TEXT as string
		requestTxID, ok := row[0].(string)
		if !ok {
			return nil, fmt.Errorf("row %d: expected request_tx_id to be string, got %T", i, row[0])
		}
		metadata.RequestTxID = requestTxID

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
		// Gateway always returns BOOLEAN as bool
		encryptSig, ok := row[5].(bool)
		if !ok {
			return nil, fmt.Errorf("row %d: expected encrypt_sig to be bool, got %T", i, row[5])
		}
		metadata.EncryptSig = encryptSig

		results = append(results, metadata)
	}

	return results, nil
}

// Helper function to extract bytes from a column
// The Kwil gateway returns BYTEA columns as base64-encoded strings in JSON responses.
// However, for certain fields (like wallet addresses), it returns hex strings (prefixed with 0x).
func extractBytesColumn(value any, dest *[]byte, rowIdx int, colName string) error {
	if value == nil {
		*dest = nil
		return nil
	}

	// Gateway always returns BYTEA as string (base64-encoded or hex)
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("row %d: expected %s to be string, got %T", rowIdx, colName, value)
	}

	if len(str) == 0 {
		*dest = nil
		return nil
	}

	// Encoding logic:
	// 1. If it starts with 0x, try HEX first.
	// 2. Try Base64 on the original string — "0" and "x" are valid base64
	//    characters (indices 52 and 49), so a base64-encoded hash can
	//    legitimately start with "0x" (e.g. any SHA-256 whose first two
	//    bytes are 0xD3 0x1X). Stripping the prefix would truncate the
	//    base64 payload and break decoding.
	// 3. If the string had a 0x prefix and both hex and full-string base64
	//    failed, try base64 on the stripped suffix — this handles the
	//    "hybrid" case where the gateway prepends 0x to a base64 value.

	has0xPrefix := len(str) >= 2 && str[:2] == "0x"

	if has0xPrefix {
		hexData := str[2:]
		decoded, err := hex.DecodeString(hexData)
		if err == nil {
			*dest = decoded
			return nil
		}
	}

	// Try base64 on the original string (handles pure base64 starting with "0x")
	if decoded, err := tryBase64Decode(str); err == nil {
		*dest = decoded
		return nil
	}

	// If it had a 0x prefix, try base64 on the stripped suffix (hybrid case)
	if has0xPrefix {
		if decoded, err := tryBase64Decode(str[2:]); err == nil {
			*dest = decoded
			return nil
		}
	}

	return fmt.Errorf("row %d: failed to decode %s as hex or base64 (len=%d, data=%q)", rowIdx, colName, len(str), str)
}

// tryBase64Decode attempts to decode s using multiple base64 variants
// (standard, raw, URL-safe, raw URL-safe). Returns the decoded bytes on
// success or an error if none of the variants work.
func tryBase64Decode(s string) ([]byte, error) {
	for _, enc := range []interface{ DecodeString(string) ([]byte, error) }{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if decoded, err := enc.DecodeString(s); err == nil {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("no base64 variant could decode input (len=%d)", len(s))
}

// Helper function to extract int64 from a column
// The Kwil gateway typically returns INT8 columns as strings to preserve precision.
// However, in some contexts (e.g. non-DECIMAL numeric types), standard JSON number serialization (float64) is used.
func extractInt64Column(value any, dest *int64, rowIdx int, colName string) error {
	switch v := value.(type) {
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("row %d: failed to parse %s as int64 (value=%q): %w", rowIdx, colName, v, err)
		}
		*dest = n
	case float64:
		*dest = int64(v)
	case int:
		*dest = int64(v)
	case int64:
		*dest = v
	default:
		return fmt.Errorf("row %d: expected %s to be string or number, got %T", rowIdx, colName, value)
	}

	return nil
}
