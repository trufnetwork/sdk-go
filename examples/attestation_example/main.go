package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/contractsapi"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

func main() {
	// Load environment variables (try CWD first, then parent)
	if err := godotenv.Load(".env", "../../.env"); err != nil {
		log.Printf("Info: no .env loaded (%v) — proceeding with environment only", err)
	}

	ctx := context.Background()

	// Setup client with private key
	privateKeyHex := os.Getenv("PRIVATE_KEY")
	if privateKeyHex == "" {
		log.Fatal("PRIVATE_KEY environment variable is required")
	}

	pk, err := crypto.Secp256k1PrivateKeyFromHex(privateKeyHex)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}
	signer := &auth.EthPersonalSigner{Key: *pk}

	// Get provider URL from environment or use default
	providerURL := os.Getenv("PROVIDER_URL")
	if providerURL == "" {
		providerURL = "https://gateway.mainnet.truf.network"
	}

	// Create TN client
	tnClient, err := tnclient.NewClient(
		ctx,
		providerURL,
		tnclient.WithSigner(signer),
	)
	if err != nil {
		log.Fatalf("Failed to create TN client: %v", err)
	}

	log.Printf("Connected to TN network at %s", providerURL)
	myAddr := tnClient.Address()
	log.Printf("Using address: %s", myAddr.Address())

	// Load attestation actions
	attestationActions, err := tnClient.LoadAttestationActions()
	if err != nil {
		log.Fatalf("Failed to load attestation actions: %v", err)
	}

	// Example attestation parameters
	dataProvider := "0x4710a8d8f0d845da110086812a32de6d90d7ff5c" // AI Index data provider
	streamID := "stai0000000000000000000000000000"               // AI Index stream

	// Prepare query parameters (last 7 days)
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)

	// Arguments for get_record action
	args := []any{
		dataProvider,
		streamID,
		int64(weekAgo.Unix()),
		int64(now.Unix()),
		nil,   // frozen_at (optional)
		false, // use_cache (will be forced to false by node for attestation)
	}

	fmt.Println("\n=== Requesting Attestation ===")
	fmt.Printf("Data Provider: %s\n", dataProvider)
	fmt.Printf("Stream ID: %s\n", streamID)
	fmt.Printf("Time Range: %s to %s\n", weekAgo.Format(time.RFC3339), now.Format(time.RFC3339))

	// Request attestation
	result, err := attestationActions.RequestAttestation(ctx, types.RequestAttestationInput{
		DataProvider: dataProvider,
		StreamID:     streamID,
		ActionName:   "get_record",
		Args:         args,
		EncryptSig:   false,
		MaxFee:       "100000000000000000000", // Maximum fee: 100 TRUF (100 * 10^18)
	})
	if err != nil {
		log.Fatalf("Failed to request attestation: %v", err)
	}

	fmt.Printf("\n✓ Attestation requested successfully!\n")
	fmt.Printf("Request TX ID: %s\n", result.RequestTxID)

	// Wait for the attestation to be signed with bounded polling
	fmt.Println("\n=== Retrieving Signed Attestation ===")
	fmt.Println("Polling for signed attestation (max 30 seconds)...")

	ctxPoll, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var signedResult *types.SignedAttestationResult
	for {
		select {
		case <-ctxPoll.Done():
			log.Println("Warning: Timed out waiting for signature")
			log.Println("The attestation may still be processing. Try checking again later.")
			goto afterPoll
		case <-ticker.C:
			signed, err := attestationActions.GetSignedAttestation(ctxPoll, types.GetSignedAttestationInput{
				RequestTxID: result.RequestTxID,
			})
			if err == nil && signed != nil && len(signed.Payload) > 0 {
				signedResult = signed
				goto afterPoll
			}
			// Continue polling on error (attestation likely not ready)
		}
	}

afterPoll:
	if signedResult != nil {
		fmt.Printf("✓ Retrieved signed attestation!\n")
		fmt.Printf("Payload size: %d bytes\n", len(signedResult.Payload))
		fmt.Printf("Payload (hex): %x...\n", signedResult.Payload[:min(64, len(signedResult.Payload))])

		// ===== Extract Validator Public Key from Payload =====
		fmt.Println("\n===== Verifying Signature =====")

		// Validate payload has minimum length (at least 1 byte data + 65 bytes signature)
		if len(signedResult.Payload) < 66 {
			log.Printf("⚠️  Payload too short (%d bytes), expected at least 66 bytes\n", len(signedResult.Payload))
		} else {
			signatureOffset := len(signedResult.Payload) - 65
			canonicalPayload := signedResult.Payload[:signatureOffset]
			signature := signedResult.Payload[signatureOffset:]

			// Hash the canonical payload with SHA256 (as per attestation spec)
			hash := sha256.Sum256(canonicalPayload)

			// Adjust signature format for recovery
			// The attestation signature uses Ethereum format with V=27/28
			// But kwil-db's RecoverSecp256k1KeyFromSigHash expects V=0-3 (it adds 27 internally)
			adjustedSig := make([]byte, 65)
			copy(adjustedSig, signature)
			if signature[64] >= 27 {
				adjustedSig[64] = signature[64] - 27
			}

			// Recover validator public key from signature
			pubKey, err := crypto.RecoverSecp256k1KeyFromSigHash(hash[:], adjustedSig)
			if err != nil {
				log.Printf("⚠️  Failed to recover public key: %v\n", err)
			} else {
				// Derive Ethereum address from public key
				validatorAddr := crypto.EthereumAddressFromPubKey(pubKey)
				fmt.Printf("✅ Validator Address: 0x%x\n", validatorAddr)
				fmt.Println("   This is the address you should use in your EVM smart contract's verify() function")
			}

			// ===== Parse Attestation Payload =====
			fmt.Println("\n===== Parsing Attestation Payload =====")
			parsed, err := contractsapi.ParseAttestationPayload(canonicalPayload)
			if err != nil {
				log.Printf("⚠️  Failed to parse payload: %v\n", err)
			} else {
				fmt.Printf("\n📋 Attestation Details:\n")
				fmt.Printf("   Version: %d\n", parsed.Version)
				fmt.Printf("   Algorithm: %d (0 = secp256k1)\n", parsed.Algorithm)
				fmt.Printf("   Block Height: %d\n", parsed.BlockHeight)
				fmt.Printf("   Data Provider: %s\n", parsed.DataProvider)
				fmt.Printf("   Stream ID: %s\n", parsed.StreamID)
				fmt.Printf("   Action ID: %d\n\n", parsed.ActionID)

				fmt.Printf("📊 Attested Query Result (from get_record):\n")
				if len(parsed.Result) == 0 {
					fmt.Println("   No records found")
				} else {
					fmt.Printf("   Found %d row(s):\n\n", len(parsed.Result))
					for i, row := range parsed.Result {
						if len(row.Values) >= 2 {
							fmt.Printf("   Row %d: Timestamp=%v, Value=%v\n", i+1, row.Values[0], row.Values[1])
						} else {
							fmt.Printf("   Row %d: %v\n", i+1, row.Values)
						}
					}
				}

				fmt.Println("\n   💡 How to use this payload:")
				fmt.Println("   1. Send this hex payload to your EVM smart contract")
				fmt.Println("   2. The contract can verify the signature using ecrecover")
				fmt.Println("   3. Parse the payload to extract the attested query results")
				fmt.Println("   4. Use the verified data in your on-chain logic (e.g., settle bets, trigger payments)")
			}
		}
	}

	// List recent attestations
	fmt.Println("\n=== Listing My Recent Attestations ===")
	myAddress := tnClient.Address()
	addr := myAddress.Address()

	// Guard address slicing - validate format before decoding
	if len(addr) != 42 || addr[:2] != "0x" {
		log.Printf("Warning: Unexpected address format: %q", addr)
	} else {
		addressBytes, err := hex.DecodeString(addr[2:]) // Remove 0x prefix
		if err != nil {
			log.Printf("Warning: Failed to decode address: %v", err)
		} else {
			limit := 10
			attestations, err := attestationActions.ListAttestations(ctx, types.ListAttestationsInput{
				Requester: addressBytes,
				Limit:     &limit,
				OrderBy:   strPtr("created_height desc"),
			})
			if err != nil {
				log.Printf("Warning: Failed to list attestations: %v", err)
			} else {
				fmt.Printf("Found %d recent attestations:\n", len(attestations))
				for i, att := range attestations {
					status := "unsigned"
					if att.SignedHeight != nil {
						status = fmt.Sprintf("signed at height %d", *att.SignedHeight)
					}
					fmt.Printf("%d. TX: %s, Created: height %d, Status: %s\n",
						i+1, att.RequestTxID, att.CreatedHeight, status)
				}
			}
		}
	}

	fmt.Println("\n=== Example Complete ===")
	fmt.Println("✓ Successfully demonstrated attestation workflow")
}

func strPtr(s string) *string {
	return &s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
