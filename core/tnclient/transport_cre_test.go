//go:build wasip1

package tnclient

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Note: These are basic structural tests for CRE transport.
// Full integration tests require running in actual CRE environment.
// See the examples/cre_integration/ directory for complete working examples.

func TestNewCRETransport(t *testing.T) {
	// Note: We cannot create a real NodeRuntime outside of CRE environment,
	// so this test just verifies the function signature and basic structure.

	t.Run("constructor_exists", func(t *testing.T) {
		// This test just verifies that the NewCRETransport function exists
		// and has the expected signature.
		// Actual testing requires CRE simulation environment.

		// Verify the function is not nil
		assert.NotNil(t, NewCRETransport)
	})
}

func TestCRETransport_Implements_Transport_Interface(t *testing.T) {
	// This compile-time check verifies that CRETransport implements Transport
	// The var _ Transport = (*CRETransport)(nil) line in transport_cre.go
	// ensures this at compile time, but we include this test for documentation.

	t.Run("implements_interface", func(t *testing.T) {
		// If this compiles, the interface is implemented
		var _ Transport = (*CRETransport)(nil)
	})
}

func TestWithCRETransport(t *testing.T) {
	t.Run("option_exists", func(t *testing.T) {
		// Verify the WithCRETransport option function exists
		assert.NotNil(t, WithCRETransport)
	})

	t.Run("option_signature", func(t *testing.T) {
		// Verify the function returns an Option
		// This test documents the expected signature
		var _ Option = WithCRETransport(nil, "http://example.com")
	})
}

func TestWithCRETransportAndSigner(t *testing.T) {
	t.Run("option_exists", func(t *testing.T) {
		// Verify the WithCRETransportAndSigner option function exists
		assert.NotNil(t, WithCRETransportAndSigner)
	})

	t.Run("option_signature", func(t *testing.T) {
		// Verify the function returns an Option
		var _ Option = WithCRETransportAndSigner(nil, "http://example.com", nil)
	})
}

// Unit tests for error classification

func TestIsTransientTxError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		want      bool
		reasoning string
	}{
		{
			name:      "nil error",
			err:       nil,
			want:      false,
			reasoning: "nil errors are not transient",
		},
		{
			name:      "ErrorTxNotFound code",
			err:       fmt.Errorf("JSON-RPC error: transaction not found (code: -202)"),
			want:      true,
			reasoning: "ErrorTxNotFound (-202) is transient - tx not indexed yet",
		},
		{
			name:      "ErrorTimeout code",
			err:       fmt.Errorf("JSON-RPC error: request timeout (code: -32001)"),
			want:      true,
			reasoning: "ErrorTimeout (-32001) is transient",
		},
		{
			name:      "ErrorInvalidParams code",
			err:       fmt.Errorf("JSON-RPC error: invalid parameters (code: -32602)"),
			want:      false,
			reasoning: "ErrorInvalidParams is permanent - malformed request",
		},
		{
			name:      "ErrorInternal code",
			err:       fmt.Errorf("JSON-RPC error: internal error (code: -32603)"),
			want:      false,
			reasoning: "ErrorInternal is permanent - server issue",
		},
		{
			name:      "Fallback: not found message",
			err:       fmt.Errorf("transaction not found in mempool"),
			want:      true,
			reasoning: "Contains 'not found' pattern",
		},
		{
			name:      "Fallback: not indexed message",
			err:       fmt.Errorf("transaction not indexed yet"),
			want:      true,
			reasoning: "Contains 'not indexed' pattern",
		},
		{
			name:      "Fallback: pending message",
			err:       fmt.Errorf("transaction is pending"),
			want:      true,
			reasoning: "Contains 'pending' pattern",
		},
		{
			name:      "Fallback: timeout message",
			err:       fmt.Errorf("connection timeout"),
			want:      true,
			reasoning: "Contains 'timeout' pattern",
		},
		{
			name:      "Permanent: authentication error",
			err:       fmt.Errorf("authentication failed"),
			want:      false,
			reasoning: "Does not match transient patterns",
		},
		{
			name:      "Permanent: network error",
			err:       fmt.Errorf("network unreachable"),
			want:      false,
			reasoning: "Network errors should be handled by caller retries",
		},
		{
			name:      "Case insensitive: NOT FOUND",
			err:       fmt.Errorf("Transaction NOT FOUND"),
			want:      true,
			reasoning: "Should match case-insensitively",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientTxError(tt.err)
			if got != tt.want {
				t.Errorf("isTransientTxError() = %v, want %v\nReasoning: %s\nError: %v",
					got, tt.want, tt.reasoning, tt.err)
			}
		})
	}
}
