//go:build wasip1

package tnclient

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			name:      "Multi-word message with code",
			err:       fmt.Errorf("JSON-RPC error: transaction not found in mempool or ledger (code: -202)"),
			want:      true,
			reasoning: "Regex should handle multi-word messages (fixed from %*s limitation)",
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

// -----------------------------------------------------------------------------
// New tests for CRE HTTP cache settings
// -----------------------------------------------------------------------------

func TestCRETransport_CacheSettingsForJSONRPC(t *testing.T) {
	t.Run("broadcast_is_cached", func(t *testing.T) {
		tr := &CRETransport{}
		cs := tr.cacheSettingsForJSONRPC("user.broadcast", []byte(`{"tx":"abc"}`))
		require.NotNil(t, cs, "user.broadcast should return non-nil CacheSettings")
		assert.True(t, cs.Store, "user.broadcast should set Store=true")
		require.NotNil(t, cs.MaxAge, "user.broadcast should set MaxAge")
		assert.Equal(t, defaultBroadcastCacheMaxAge, cs.MaxAge.AsDuration(), "MaxAge should match defaultBroadcastCacheMaxAge")
	})

	t.Run("non_broadcast_is_not_cached", func(t *testing.T) {
		tr := &CRETransport{}
		assert.Nil(t, tr.cacheSettingsForJSONRPC("user.call", []byte(`{}`)))
		assert.Nil(t, tr.cacheSettingsForJSONRPC("user.tx_query", []byte(`{}`)))
		assert.Nil(t, tr.cacheSettingsForJSONRPC("kgw.authn", []byte(`{}`)))
	})

	t.Run("paramsJSON_is_accepted", func(t *testing.T) {
		// Policy currently ignores paramsJSON, but this test ensures we can pass
		// arbitrary JSON without panicking and still get the expected outcome.
		tr := &CRETransport{}
		cs := tr.cacheSettingsForJSONRPC("user.broadcast", []byte(`{"nested":{"a":[1,2,3]}}`))
		require.NotNil(t, cs)
		assert.Equal(t, defaultBroadcastCacheMaxAge, cs.MaxAge.AsDuration())
	})
}

// Test that the caching change is actually wired into HTTP request construction.
//
// We cannot execute the CRE HTTP client outside a CRE runtime, so this test parses
// transport_cre.go and verifies that:
//   - doJSONRPC, doJSONRPCWithResponse, and executeOnce each set CacheSettings in
//     their &http.Request literals
//   - the CacheSettings value is the local variable named `cacheSettings`
//   - `cacheSettings` is assigned from t.cacheSettingsForJSONRPC(..., paramsJSON)
func TestCRETransport_HTTPRequestsIncludeCacheSettings(t *testing.T) {
	src, err := os.ReadFile("transport_cre.go")
	require.NoError(t, err, "failed to read transport_cre.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "transport_cre.go", src, 0)
	require.NoError(t, err, "failed to parse transport_cre.go")

	targets := []struct {
		funcName               string
		expectFirstArgIsMethod bool   // if true, expects first arg ident "method"
		expectFirstArgString   string // if non-empty, expects string literal
	}{
		{funcName: "doJSONRPC", expectFirstArgIsMethod: true},
		{funcName: "doJSONRPCWithResponse", expectFirstArgIsMethod: true},
		{funcName: "executeOnce", expectFirstArgString: "user.broadcast"},
	}

	for _, tc := range targets {
		t.Run(tc.funcName, func(t *testing.T) {
			fd := findFuncDecl(t, f, tc.funcName)
			require.NotNil(t, fd, "function %s not found", tc.funcName)

			// 1) Ensure we assign: cacheSettings := t.cacheSettingsForJSONRPC(<method>, paramsJSON)
			assert.True(t, hasCacheSettingsAssignment(t, fd, tc.expectFirstArgIsMethod, tc.expectFirstArgString),
				"%s should assign cacheSettings := t.cacheSettingsForJSONRPC(..., paramsJSON)", tc.funcName)

			// 2) Ensure &http.Request{..., CacheSettings: cacheSettings, ...} exists
			assert.True(t, hasHttpRequestWithCacheSettings(t, fd),
				"%s should set CacheSettings on http.Request literal", tc.funcName)
		})
	}
}

func findFuncDecl(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name != nil && fd.Name.Name == name {
			return fd
		}
	}
	return nil
}

func hasCacheSettingsAssignment(t *testing.T, fd *ast.FuncDecl, firstArgIsMethod bool, firstArgString string) bool {
	t.Helper()

	found := false
	ast.Inspect(fd, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}

		lhs, ok := as.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name != "cacheSettings" {
			return true
		}

		// RHS must be t.cacheSettingsForJSONRPC(...)
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != "t" || sel.Sel == nil || sel.Sel.Name != "cacheSettingsForJSONRPC" {
			return true
		}

		if len(call.Args) != 2 {
			return true
		}

		// Second arg must be paramsJSON ident
		if id2, ok := call.Args[1].(*ast.Ident); !ok || id2.Name != "paramsJSON" {
			return true
		}

		// First arg check
		if firstArgIsMethod {
			id1, ok := call.Args[0].(*ast.Ident)
			if !ok || id1.Name != "method" {
				return true
			}
		}
		if firstArgString != "" {
			bl, ok := call.Args[0].(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				return true
			}
			unquoted, err := strconv.Unquote(bl.Value)
			if err != nil || unquoted != firstArgString {
				return true
			}
		}

		found = true
		return true
	})
	return found
}

func hasHttpRequestWithCacheSettings(t *testing.T, fd *ast.FuncDecl) bool {
	t.Helper()

	found := false
	ast.Inspect(fd, func(n ast.Node) bool {
		// Look for: &http.Request{ ... CacheSettings: cacheSettings ... }
		ue, ok := n.(*ast.UnaryExpr)
		if !ok || ue.Op != token.AND {
			return true
		}
		cl, ok := ue.X.(*ast.CompositeLit)
		if !ok {
			return true
		}
		// Type must be http.Request
		se, ok := cl.Type.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := se.X.(*ast.Ident)
		if !ok || pkg.Name != "http" || se.Sel == nil || se.Sel.Name != "Request" {
			return true
		}

		// Must include CacheSettings: cacheSettings
		for _, elt := range cl.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "CacheSettings" {
				continue
			}
			val, ok := kv.Value.(*ast.Ident)
			if !ok || val.Name != "cacheSettings" {
				continue
			}
			found = true
			return true
		}

		return true
	})
	return found
}

func TestDefaultBroadcastCacheMaxAge(t *testing.T) {
	// Defensive check: ensure the constant remains what the transport expects.
	// If this is intentionally changed, update tests accordingly.
	assert.Equal(t, 2*time.Minute, defaultBroadcastCacheMaxAge)
}
