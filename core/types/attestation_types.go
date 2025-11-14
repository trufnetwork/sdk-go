package types

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
)

// RequestAttestationInput represents the parameters for requesting an attestation
type RequestAttestationInput struct {
	DataProvider string // 0x-prefixed hex address (42 chars)
	StreamID     string // 32-character stream ID
	ActionName   string // Action to attest (e.g., "get_record")
	Args         []any  // Action arguments (will be encoded)
	EncryptSig   bool   // Must be false in MVP
	MaxFee       string // Maximum fee willing to pay (NUMERIC(78,0) as string)
}

// RequestAttestationResult contains the response from request_attestation action
type RequestAttestationResult struct {
	RequestTxID string // Transaction ID for this attestation request
}

// GetSignedAttestationInput specifies which attestation to retrieve
type GetSignedAttestationInput struct {
	RequestTxID string // Transaction ID from request
}

// SignedAttestationResult contains the complete signed attestation payload
type SignedAttestationResult struct {
	Payload []byte // Canonical payload + signature (9 fields)
}

// ListAttestationsInput specifies filters for listing attestations
type ListAttestationsInput struct {
	Requester []byte  // Optional: filter by requester address (20 bytes)
	Limit     *int    // Optional: max results (default/max 5000)
	Offset    *int    // Optional: pagination offset
	OrderBy   *string // Optional: "created_height asc" or "created_height desc"
}

// AttestationMetadata represents a single attestation in a list
type AttestationMetadata struct {
	RequestTxID     string
	AttestationHash []byte
	Requester       []byte
	CreatedHeight   int64
	SignedHeight    *int64 // Nil if not yet signed
	EncryptSig      bool
}

// IAttestationAction provides methods for requesting and retrieving attestations
type IAttestationAction interface {
	// RequestAttestation submits a request for a signed attestation of query results
	RequestAttestation(ctx context.Context, input RequestAttestationInput) (*RequestAttestationResult, error)

	// GetSignedAttestation retrieves a complete signed attestation payload
	GetSignedAttestation(ctx context.Context, input GetSignedAttestationInput) (*SignedAttestationResult, error)

	// ListAttestations returns metadata for attestations, optionally filtered
	ListAttestations(ctx context.Context, input ListAttestationsInput) ([]AttestationMetadata, error)
}

// DecodedRow represents a decoded row from attestation query results
type DecodedRow struct {
	Values []any `json:"values"`
}

// ParsedAttestationPayload contains the decoded attestation payload
type ParsedAttestationPayload struct {
	Version      uint8        `json:"version"`
	Algorithm    uint8        `json:"algorithm"`     // 0 = secp256k1
	BlockHeight  uint64       `json:"blockHeight"`
	DataProvider string       `json:"dataProvider"`  // 0x-prefixed hex address
	StreamID     string       `json:"streamId"`
	ActionID     uint16       `json:"actionId"`
	Arguments    []any        `json:"arguments"`
	Result       []DecodedRow `json:"result"`
}

// Validate validates the request attestation input
func (r *RequestAttestationInput) Validate() error {
	if len(r.DataProvider) != 42 {
		return fmt.Errorf("data_provider must be 0x-prefixed 40 hex characters, got %d chars", len(r.DataProvider))
	}
	if !strings.HasPrefix(r.DataProvider, "0x") {
		return fmt.Errorf("data_provider must start with 0x prefix")
	}
	// Validate that the hex characters after 0x are valid
	if _, err := hex.DecodeString(r.DataProvider[2:]); err != nil {
		return fmt.Errorf("data_provider must contain valid hex characters after 0x prefix: %w", err)
	}
	if len(r.StreamID) != 32 {
		return fmt.Errorf("stream_id must be 32 characters, got %d", len(r.StreamID))
	}
	if r.ActionName == "" {
		return fmt.Errorf("action_name cannot be empty")
	}
	if r.EncryptSig {
		return fmt.Errorf("encryption not implemented in MVP")
	}
	// MaxFee is optional (can be empty string for no limit)
	// If provided, it should be a valid non-negative integer string
	if r.MaxFee != "" {
		// Basic validation - just check it's a valid number format
		for _, ch := range r.MaxFee {
			if ch < '0' || ch > '9' {
				return fmt.Errorf("max_fee must be a numeric string, got: %s", r.MaxFee)
			}
		}
	}
	return nil
}
