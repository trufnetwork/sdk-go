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

func TestNewLocalClient_Success(t *testing.T) {
	// We don't need to reach the server — just assert that a well-formed
	// admin URL produces a non-nil ILocalActions.
	local, err := NewLocalClient("http://127.0.0.1:8485")
	require.NoError(t, err)
	require.NotNil(t, local)
}
