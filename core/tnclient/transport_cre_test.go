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
	t.Run("constructor_exists", func(t *testing.T) {
		assert.NotNil(t, NewCRETransport)
	})

	t.Run("defaults_and_endpoint_normalization", func(t *testing.T) {
		tr, err := NewCRETransport(nil, "https://example.com", nil)
		require.NoError(t, err)

		// Endpoint should have /rpc/v1 suffix
		assert.Equal(t, "https://example.com/rpc/v1", tr.endpoint)

		// Cache defaults should be applied
		assert.Equal(t, defaultHTTPCacheStore, tr.httpCacheStore)
		assert.Equal(t, defaultHTTPCacheMaxAge, tr.httpCacheMaxAge)

		// Sanity: cache settings should be non-nil for any method
		cs := tr.cacheSettingsForJSONRPC("user.call", []byte(`{}`))
		require.NotNil(t, cs)
		assert.Equal(t, defaultHTTPCacheStore, cs.Store)
		require.NotNil(t, cs.MaxAge)
		assert.Equal(t, defaultHTTPCacheMaxAge, cs.MaxAge.AsDuration())
	})
}

func TestCRETransport_Implements_Transport_Interface(t *testing.T) {
	t.Run("implements_interface", func(t *testing.T) {
		var _ Transport = (*CRETransport)(nil)
	})
}

func TestWithCRETransport(t *testing.T) {
	t.Run("option_exists", func(t *testing.T) {
		assert.NotNil(t, WithCRETransport)
	})

	t.Run("option_signature", func(t *testing.T) {
		var _ Option = WithCRETransport(nil, "http://example.com")
	})
}

func TestWithCRETransportAndSigner(t *testing.T) {
	t.Run("option_exists", func(t *testing.T) {
		assert.NotNil(t, WithCRETransportAndSigner)
	})

	t.Run("option_signature", func(t *testing.T) {
		var _ Option = WithCRETransportAndSigner(nil, "http://example.com", nil)
	})
}

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
			reasoning: "Regex should handle multi-word messages",
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

func TestCRETransport_ApplyHTTPCacheConfig(t *testing.T) {
	t.Run("nil_config_is_noop", func(t *testing.T) {
		tr, err := NewCRETransport(nil, "https://example.com", nil)
		require.NoError(t, err)

		beforeStore := tr.httpCacheStore
		beforeAge := tr.httpCacheMaxAge

		tr.ApplyHTTPCacheConfig(nil)

		assert.Equal(t, beforeStore, tr.httpCacheStore)
		assert.Equal(t, beforeAge, tr.httpCacheMaxAge)
	})

	t.Run("overrides_are_applied", func(t *testing.T) {
		tr, err := NewCRETransport(nil, "https://example.com", nil)
		require.NoError(t, err)

		store := false
		secs := int64(30)
		cfg := &CREHTTPCacheConfig{Store: &store, MaxAgeSeconds: &secs}

		tr.ApplyHTTPCacheConfig(cfg)

		assert.False(t, tr.httpCacheStore)
		assert.Equal(t, 30*time.Second, tr.httpCacheMaxAge)

		cs := tr.cacheSettingsForJSONRPC("user.call", []byte(`{}`))
		require.NotNil(t, cs)
		assert.False(t, cs.Store)
		require.NotNil(t, cs.MaxAge)
		assert.Equal(t, 30*time.Second, cs.MaxAge.AsDuration())
	})

	t.Run("negative_max_age_is_clamped_to_zero", func(t *testing.T) {
		tr, err := NewCRETransport(nil, "https://example.com", nil)
		require.NoError(t, err)

		secs := int64(-5)
		cfg := &CREHTTPCacheConfig{MaxAgeSeconds: &secs}
		tr.ApplyHTTPCacheConfig(cfg)

		assert.Equal(t, 0*time.Second, tr.httpCacheMaxAge)

		cs := tr.cacheSettingsForJSONRPC("user.call", []byte(`{}`))
		require.NotNil(t, cs)
		require.NotNil(t, cs.MaxAge)
		assert.Equal(t, 0*time.Second, cs.MaxAge.AsDuration())
	})

	t.Run("max_age_is_clamped_to_cre_max", func(t *testing.T) {
		tr, err := NewCRETransport(nil, "https://example.com", nil)
		require.NoError(t, err)

		secs := int64(999999)
		cfg := &CREHTTPCacheConfig{MaxAgeSeconds: &secs}
		tr.ApplyHTTPCacheConfig(cfg)

		assert.Equal(t, maxHTTPCacheMaxAge, tr.httpCacheMaxAge)

		cs := tr.cacheSettingsForJSONRPC("user.call", []byte(`{}`))
		require.NotNil(t, cs)
		require.NotNil(t, cs.MaxAge)
		assert.Equal(t, maxHTTPCacheMaxAge, cs.MaxAge.AsDuration())
	})
}

func TestCRETransport_CacheSettingsForJSONRPC(t *testing.T) {
	t.Run("all_methods_return_cache_settings", func(t *testing.T) {
		tr, err := NewCRETransport(nil, "https://example.com", nil)
		require.NoError(t, err)

		methods := []string{
			"user.call",
			"user.tx_query",
			"user.account",
			"user.chain_info",
			"kgw.authn_param",
			"kgw.authn",
			"user.broadcast",
		}

		for _, m := range methods {
			cs := tr.cacheSettingsForJSONRPC(m, []byte(`{}`))
			require.NotNil(t, cs, "method %s should return non-nil CacheSettings", m)
			assert.Equal(t, defaultHTTPCacheStore, cs.Store, "method %s Store should match transport default", m)
			require.NotNil(t, cs.MaxAge, "method %s should set MaxAge", m)
			assert.Equal(t, defaultHTTPCacheMaxAge, cs.MaxAge.AsDuration(), "method %s MaxAge should match transport default", m)
		}
	})
}

func TestCRETransport_NextReqID(t *testing.T) {
	t.Run("deterministic_when_caching_active", func(t *testing.T) {
		tr, err := NewCRETransport(nil, "https://example.com", nil)
		require.NoError(t, err)

		params := []byte(`{"a":1}`)
		id1 := tr.nextReqID("user.call", params)
		id2 := tr.nextReqID("user.call", params)

		assert.Equal(t, id1, id2)
		assert.True(t, stringsHasPrefix(id1, "tn:"), "expected deterministic id to have tn: prefix")
		assert.Equal(t, 19, len(id1), "expected tn: + 16 hex chars")

		id3 := tr.nextReqID("user.call", []byte(`{"a":2}`))
		assert.NotEqual(t, id1, id3)

		// Different method -> different id
		id4 := tr.nextReqID("user.tx_query", params)
		assert.NotEqual(t, id1, id4)
	})

	t.Run("monotonic_sequence_when_caching_disabled", func(t *testing.T) {
		// Construct minimal transport with caching disabled and fresh reqID counter.
		tr := &CRETransport{
			httpCacheStore:  false,
			httpCacheMaxAge: 0,
		}

		id1 := tr.nextReqID("user.call", []byte(`{}`))
		id2 := tr.nextReqID("user.call", []byte(`{}`))

		assert.Equal(t, "1", id1)
		assert.Equal(t, "2", id2)
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
		expectFirstArgIsMethod bool
		expectFirstArgString   string
	}{
		{funcName: "doJSONRPC", expectFirstArgIsMethod: true},
		{funcName: "doJSONRPCWithResponse", expectFirstArgIsMethod: true},
		{funcName: "executeOnce", expectFirstArgString: "user.broadcast"},
	}

	for _, tc := range targets {
		t.Run(tc.funcName, func(t *testing.T) {
			fd := findFuncDecl(t, f, tc.funcName)
			require.NotNil(t, fd, "function %s not found", tc.funcName)

			assert.True(t, hasCacheSettingsAssignment(t, fd, tc.expectFirstArgIsMethod, tc.expectFirstArgString),
				"%s should assign cacheSettings := t.cacheSettingsForJSONRPC(..., paramsJSON)", tc.funcName)

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

		if id2, ok := call.Args[1].(*ast.Ident); !ok || id2.Name != "paramsJSON" {
			return true
		}

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
		ue, ok := n.(*ast.UnaryExpr)
		if !ok || ue.Op != token.AND {
			return true
		}
		cl, ok := ue.X.(*ast.CompositeLit)
		if !ok {
			return true
		}

		se, ok := cl.Type.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := se.X.(*ast.Ident)
		if !ok || pkg.Name != "http" || se.Sel == nil || se.Sel.Name != "Request" {
			return true
		}

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

func stringsHasPrefix(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	return s[:len(prefix)] == prefix
}
