package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

func TestAttestationE2E(t *testing.T) {
	ctx := context.Background()

	// Setup test environment (starts Docker containers + runs migrations)
	fixture := NewServerFixture(t)
	err := fixture.Setup()
	require.NoError(t, err, "Failed to setup server fixture")

	// Teardown when test completes
	t.Cleanup(func() {
		fixture.Teardown()
	})

	// Create SDK client with test wallet
	wallet, err := kwilcrypto.Secp256k1PrivateKeyFromHex(AnonWalletPK)
	require.NoError(t, err, "failed to parse test wallet private key")

	client, err := tnclient.NewClient(ctx, TestKwilProvider,
		tnclient.WithSigner(auth.GetUserSigner(wallet)))
	require.NoError(t, err, "failed to create TN client")

	// Load attestation actions (just creates the struct, doesn't make network calls)
	attestations, err := client.LoadAttestationActions()
	require.NoError(t, err, "failed to load attestation actions")

	// Test 1: Request Attestation
	t.Run("RequestAttestation", func(t *testing.T) {
		// Prepare test arguments for get_record action
		// Args: [data_provider, stream_id, date_from, date_to, composition_ids, use_cache]
		args := []any{
			"0x4710a8d8f0d845da110086812a32de6d90d7ff5c", // data_provider
			"stai0000000000000000000000000000",           // stream_id
			int64(1609459200),                             // date_from (2021-01-01)
			int64(1640995200),                             // date_to (2022-01-01)
			nil,                                           // composition_ids (nil = all)
			false,                                         // use_cache (must be false for attestations)
		}

		result, err := attestations.RequestAttestation(ctx, types.RequestAttestationInput{
			DataProvider: "0x4710a8d8f0d845da110086812a32de6d90d7ff5c",
			StreamID:     "stai0000000000000000000000000000",
			ActionName:   "get_record",
			Args:         args,
			EncryptSig:   false,
			MaxFee:       1000000,
		})

		require.NoError(t, err, "failed to request attestation")
		assert.NotEmpty(t, result.RequestTxID, "request_tx_id should not be empty")

		// Wait for transaction to be mined so INSERT completes
		// Parse the TX hash (remove 0x prefix if present)
		txHashStr := strings.TrimPrefix(result.RequestTxID, "0x")
		txHash, err := kwiltypes.NewHashFromString(txHashStr)
		require.NoError(t, err, "failed to parse request_tx_id as hash")
		waitTxToBeMinedWithSuccess(t, ctx, client, txHash)

		// Test 2: Get Signed Attestation (nested to use result from Test 1)
		t.Run("GetSignedAttestation", func(t *testing.T) {
			// Wait for attestation to be signed (up to 30 seconds)
			// In production, you'd implement proper polling with exponential backoff
			var signedResult *types.SignedAttestationResult
			var lastErr error

			maxAttempts := 30
			for i := 0; i < maxAttempts; i++ {
				signedResult, lastErr = attestations.GetSignedAttestation(ctx,
					types.GetSignedAttestationInput{
						RequestTxID: result.RequestTxID,
					})

				// If we got the signed attestation, break
				if lastErr == nil && signedResult != nil && len(signedResult.Payload) > 0 {
					break
				}

				// If it's not a "not found" or "not yet signed" error, fail immediately
				if lastErr != nil &&
					!strings.Contains(lastErr.Error(), "not found") &&
					!strings.Contains(lastErr.Error(), "not yet signed") {
					break
				}

				// Wait 1 second before retrying
				time.Sleep(1 * time.Second)
			}

			require.NoError(t, lastErr, "failed to get signed attestation after 30s")
			require.NotNil(t, signedResult, "signed result should not be nil")
			assert.NotEmpty(t, signedResult.Payload, "payload should not be empty")

			// Payload should contain: canonical (8 fields) + signature (65 bytes)
			// Minimum size check (actual size depends on args/result encoding)
			assert.Greater(t, len(signedResult.Payload), 65,
				"payload should contain canonical data + signature (>65 bytes)")

			t.Logf("Successfully retrieved signed attestation. Payload size: %d bytes", len(signedResult.Payload))
		})
	})

	// Test 3: List Attestations
	t.Run("ListAttestations", func(t *testing.T) {
		limit := 10
		list, err := attestations.ListAttestations(ctx, types.ListAttestationsInput{
			Limit: &limit,
		})

		require.NoError(t, err, "failed to list attestations")
		require.NotEmpty(t, list, "should have at least one attestation")

		// Verify metadata fields on first attestation
		att := list[0]
		assert.NotEmpty(t, att.RequestTxID, "request_tx_id should not be empty")
		assert.NotEmpty(t, att.AttestationHash, "attestation_hash should not be empty")
		assert.Greater(t, att.CreatedHeight, int64(0), "created_height should be positive")

		if att.SignedHeight != nil {
			assert.Greater(t, *att.SignedHeight, int64(0), "signed_height should be positive")
			assert.GreaterOrEqual(t, *att.SignedHeight, att.CreatedHeight,
				"signed_height should be >= created_height")
		}

		t.Logf("Found %d attestations. First attestation: TX=%s, Height=%d",
			len(list), att.RequestTxID, att.CreatedHeight)
	})

	// Test 4: List Attestations with Filters
	t.Run("ListAttestationsWithFilters", func(t *testing.T) {
		limit := 5
		orderBy := "created_height desc"

		list, err := attestations.ListAttestations(ctx, types.ListAttestationsInput{
			Limit:   &limit,
			OrderBy: &orderBy,
		})

		require.NoError(t, err, "failed to list attestations with filters")
		require.NotEmpty(t, list, "should have at least one attestation")

		assert.LessOrEqual(t, len(list), limit, "should respect limit")

		if len(list) > 1 {
			for i := 1; i < len(list); i++ {
				assert.GreaterOrEqual(t, list[i-1].CreatedHeight, list[i].CreatedHeight,
					"attestations should be ordered by created_height desc")
			}
		}
	})
}
