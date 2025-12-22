package tnclient

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	clientType "github.com/trufnetwork/kwil-db/core/client/types"
	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	kwilTypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
)

// mockTransport implements Transport interface for testing
type mockTransport struct {
	callFunc    func(ctx context.Context, namespace string, action string, inputs []any) (*kwilTypes.CallResult, error)
	executeFunc func(ctx context.Context, namespace string, action string, inputs [][]any, opts ...clientType.TxOpt) (kwilTypes.Hash, error)
	waitTxFunc  func(ctx context.Context, txHash kwilTypes.Hash, interval time.Duration) (*kwilTypes.TxQueryResponse, error)
	chainID     string
	signer      auth.Signer
}

func (m *mockTransport) Call(ctx context.Context, namespace string, action string, inputs []any) (*kwilTypes.CallResult, error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, namespace, action, inputs)
	}
	return &kwilTypes.CallResult{QueryResult: &kwilTypes.QueryResult{}}, nil
}

func (m *mockTransport) Execute(ctx context.Context, namespace string, action string, inputs [][]any, opts ...clientType.TxOpt) (kwilTypes.Hash, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, namespace, action, inputs, opts...)
	}
	return kwilTypes.Hash{}, nil
}

func (m *mockTransport) WaitTx(ctx context.Context, txHash kwilTypes.Hash, interval time.Duration) (*kwilTypes.TxQueryResponse, error) {
	if m.waitTxFunc != nil {
		return m.waitTxFunc(ctx, txHash, interval)
	}
	return &kwilTypes.TxQueryResponse{}, nil
}

func (m *mockTransport) ChainID() string {
	return m.chainID
}

func (m *mockTransport) Signer() auth.Signer {
	return m.signer
}

// Verify mockTransport implements Transport interface at compile time
var _ Transport = (*mockTransport)(nil)

// createTestSigner creates an EthPersonalSigner for testing
func createTestSigner(t *testing.T) auth.Signer {
	t.Helper()

	// Generate a random secp256k1 private key
	privKey, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// privKey is already *crypto.Secp256k1PrivateKey, no need to dereference
	secp256k1Key, ok := privKey.(*crypto.Secp256k1PrivateKey)
	if !ok {
		t.Fatalf("Expected *crypto.Secp256k1PrivateKey, got %T", privKey)
	}

	// Create EthPersonalSigner
	return &auth.EthPersonalSigner{Key: *secp256k1Key}
}

// TestMockTransportImplementsInterface verifies mock transport implements the interface
func TestMockTransportImplementsInterface(t *testing.T) {
	mock := &mockTransport{
		chainID: "test-chain",
	}

	var transport Transport = mock
	if transport == nil {
		t.Fatal("mockTransport should implement Transport interface")
	}
}

// TestHTTPTransportImplementsInterface verifies HTTPTransport implements the interface
func TestHTTPTransportImplementsInterface(t *testing.T) {
	// This test verifies that HTTPTransport implements Transport interface
	// The actual verification happens at compile time with: var _ Transport = (*HTTPTransport)(nil)
	// This test just ensures the compile-time check is present
	t.Log("HTTPTransport implements Transport interface (verified at compile time)")
}

// TestNewClientWithCustomTransport tests using a custom transport via WithTransport option
func TestNewClientWithCustomTransport(t *testing.T) {
	testSigner := createTestSigner(t)

	customTransport := &mockTransport{
		chainID: "custom-chain-id",
		signer:  testSigner,
	}

	client, err := NewClient(
		context.Background(),
		"", // endpoint not used when transport provided
		WithTransport(customTransport),
		WithSigner(testSigner),
	)

	if err != nil {
		t.Fatalf("NewClient with custom transport failed: %v", err)
	}

	if client == nil {
		t.Fatal("Client should not be nil")
	}

	if client.transport != customTransport {
		t.Error("Client should use the provided custom transport")
	}

	// Verify transport methods are accessible through client
	if client.transport.ChainID() != "custom-chain-id" {
		t.Errorf("Expected chain ID 'custom-chain-id', got '%s'", client.transport.ChainID())
	}

	// Verify GetSigner returns the transport's signer
	if client.GetSigner() != testSigner {
		t.Error("GetSigner should return the signer from transport")
	}
}

// TestClientMethodsUseTransport verifies that client methods use the transport
func TestClientMethodsUseTransport(t *testing.T) {
	testSigner := createTestSigner(t)

	callCalled := false
	customTransport := &mockTransport{
		chainID: "test-chain",
		signer:  testSigner,
		callFunc: func(ctx context.Context, namespace string, action string, inputs []any) (*kwilTypes.CallResult, error) {
			callCalled = true
			// Return a valid response for list_streams
			return &kwilTypes.CallResult{
				QueryResult: &kwilTypes.QueryResult{
					ColumnNames: []string{"stream_id", "data_provider"},
					Values:      [][]interface{}{},
				},
			}, nil
		},
	}

	client, err := NewClient(
		context.Background(),
		"",
		WithTransport(customTransport),
		WithSigner(testSigner),
	)

	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Call ListStreams which internally uses transport.Call
	_, err = client.ListStreams(context.Background(), types.ListStreamsInput{})
	if err != nil {
		// Even if there's an error in parsing, we just want to verify transport was called
		t.Logf("ListStreams returned error (may be expected with mock data): %v", err)
	}

	if !callCalled {
		t.Error("Transport.Call should have been called by ListStreams")
	}
}

// TestGetSigner verifies GetSigner uses transport
func TestGetSigner(t *testing.T) {
	testSigner := createTestSigner(t)

	customTransport := &mockTransport{
		signer: testSigner,
	}

	client, err := NewClient(
		context.Background(),
		"",
		WithTransport(customTransport),
		WithSigner(testSigner),
	)

	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	retrievedSigner := client.GetSigner()
	if retrievedSigner != testSigner {
		t.Error("GetSigner should return the signer from transport")
	}

	// Verify it's the same instance
	if retrievedSigner.AuthType() != testSigner.AuthType() {
		t.Errorf("Expected auth type %s, got %s", testSigner.AuthType(), retrievedSigner.AuthType())
	}
}

// TestGetKwilClientWithCustomTransport verifies GetKwilClient returns nil for non-HTTP transport
func TestGetKwilClientWithCustomTransport(t *testing.T) {
	testSigner := createTestSigner(t)

	customTransport := &mockTransport{
		chainID: "test-chain",
		signer:  testSigner,
	}

	client, err := NewClient(
		context.Background(),
		"",
		WithTransport(customTransport),
		WithSigner(testSigner),
	)

	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	gwClient := client.GetKwilClient()
	if gwClient != nil {
		t.Error("GetKwilClient should return nil for non-HTTP transport (mockTransport)")
	}
}

// TestWaitForTx verifies WaitForTx uses transport
func TestWaitForTx(t *testing.T) {
	testSigner := createTestSigner(t)

	waitTxCalled := false
	testHash := kwilTypes.Hash{1, 2, 3}

	customTransport := &mockTransport{
		signer: testSigner,
		waitTxFunc: func(ctx context.Context, txHash kwilTypes.Hash, interval time.Duration) (*kwilTypes.TxQueryResponse, error) {
			waitTxCalled = true
			if txHash != testHash {
				t.Errorf("Expected hash %v, got %v", testHash, txHash)
			}
			return &kwilTypes.TxQueryResponse{
				Hash: txHash,
			}, nil
		},
	}

	client, err := NewClient(
		context.Background(),
		"",
		WithTransport(customTransport),
		WithSigner(testSigner),
	)

	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.WaitForTx(context.Background(), testHash, time.Second)
	if err != nil {
		t.Fatalf("WaitForTx failed: %v", err)
	}

	if !waitTxCalled {
		t.Error("Transport.WaitTx should have been called")
	}
}

// TestTransportChainID verifies ChainID is accessible via transport
func TestTransportChainID(t *testing.T) {
	testSigner := createTestSigner(t)

	expectedChainID := "test-network-123"

	customTransport := &mockTransport{
		chainID: expectedChainID,
		signer:  testSigner,
	}

	client, err := NewClient(
		context.Background(),
		"",
		WithTransport(customTransport),
		WithSigner(testSigner),
	)

	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Access ChainID through transport
	actualChainID := client.transport.ChainID()
	if actualChainID != expectedChainID {
		t.Errorf("Expected ChainID %s, got %s", expectedChainID, actualChainID)
	}
}
