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
	MaxFee       int64  // Maximum fee willing to pay
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
	if r.MaxFee < 0 {
		return fmt.Errorf("max_fee must be non-negative, got %d", r.MaxFee)
	}
	return nil
}
