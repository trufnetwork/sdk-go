package tnclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewLocalClient_EmptyURL(t *testing.T) {
	_, err := NewLocalClient("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "adminURL is required")
}

func TestNewLocalClient_InvalidURL(t *testing.T) {
	// url.Parse is pretty lenient and accepts most garbage; a control
	// character forces a real failure.
	_, err := NewLocalClient("http://\x00bad")
	require.Error(t, err)
}

func TestNewLocalClient_MissingScheme(t *testing.T) {
	// "127.0.0.1:8485" — url.Parse rejects this because the colon in
	// the first path segment is ambiguous. Either way, the user should
	// get a clear error — they forgot "http://".
	_, err := NewLocalClient("127.0.0.1:8485")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid adminURL")
}

func TestNewLocalClient_UnixPathNotURL(t *testing.T) {
	// "/tmp/admin.sock" parses as a bare path (no scheme, no host). Use
	// "http://unix" + rpcclient.WithHTTPClient() for unix sockets instead.
	_, err := NewLocalClient("/tmp/admin.sock")
	require.Error(t, err)
	require.Contains(t, err.Error(), "absolute URL with scheme and host")
}

func TestNewLocalClient_SchemeNoHost(t *testing.T) {
	// "http://" parses successfully but Host is empty — would produce
	// a request to "http:///rpc/v1". Reject it.
	_, err := NewLocalClient("http://")
	require.Error(t, err)
	require.Contains(t, err.Error(), "absolute URL with scheme and host")
}

func TestNewLocalClient_Success(t *testing.T) {
	// We don't need to reach the server — just assert that a well-formed
	// admin URL produces a non-nil ILocalActions.
	local, err := NewLocalClient("http://127.0.0.1:8485")
	require.NoError(t, err)
	require.NotNil(t, local)
}
