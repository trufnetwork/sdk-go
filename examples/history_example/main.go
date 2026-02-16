package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

func main() {
	ctx := context.Background()

	// 1. Initialize Client
	// Use environment variable for private key to avoid hardcoding
	privateKeyHex := os.Getenv("TN_PRIVATE_KEY")
	if privateKeyHex == "" {
		log.Fatal("TN_PRIVATE_KEY environment variable is required")
	}

	pk, err := crypto.Secp256k1PrivateKeyFromHex(privateKeyHex)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}
	signer := &auth.EthPersonalSigner{Key: *pk}

	// Use testnet gateway or local node
	endpoint := os.Getenv("TN_GATEWAY_URL")
	if endpoint == "" {
		endpoint = "https://gateway.testnet.truf.network" // Default to local node
	}

	client, err := tnclient.NewClient(ctx, endpoint, tnclient.WithSigner(signer))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Printf("🔄 Transaction History Demo\n")
	fmt.Printf("===========================\n")
	fmt.Printf("Endpoint: %s\n", endpoint)
	addr := client.Address()
	fmt.Printf("Wallet:   %s\n\n", addr.Address())

	// 2. Define History Query Parameters
	// We want history for the "hoodi_tt2" bridge
	bridgeID := "hoodi_tt2"
	walletAddress := "0xc11Ff6d3cC60823EcDCAB1089F1A4336053851EF" // Example address from issue
	limit := 10
	offset := 0

	fmt.Printf("📋 Fetching history for bridge '%s'...\n", bridgeID)
	fmt.Printf("   Wallet: %s\n", walletAddress)
	fmt.Printf("   Limit:  %d\n", limit)
	fmt.Printf("   Offset: %d\n", offset)
	fmt.Println("-------------------------------------------------------")

	// 3. Call GetHistory
	history, err := client.GetHistory(ctx, types.GetHistoryInput{
		BridgeIdentifier: bridgeID,
		Wallet:           walletAddress,
		Limit:            &limit,
		Offset:           &offset,
	})
	if err != nil {
		log.Fatalf("❌ Failed to fetch history: %v", err)
	}

	// 4. Display Results
	if len(history) == 0 {
		fmt.Println("No history records found.")
		return
	}

	// Use tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tAMOUNT\tFROM\tTO\tINTERNAL TX\tEXTERNAL TX\tSTATUS\tBLOCK\tEXT BLOCK\tTIMESTAMP")
	fmt.Fprintln(w, "----\t------\t----\t--\t-----------\t-----------\t------\t-----\t---------\t---------")

	for _, rec := range history {
		// Format timestamp
		tm := time.Unix(rec.BlockTimestamp, 0)
		timeStr := tm.Format(time.RFC3339)

		// Shorten hashes for display if needed, but printing full for now as per request
		// Or shortening to keep table readable? The CLI output truncated them.
		// "0x%x..." logic was used before. I will use a helper to optionally shorten.
		formatHexShort := func(b []byte) string {
			if len(b) == 0 {
				return "null"
			}
			if len(b) > 4 {
				return fmt.Sprintf("0x%x...", b[:4])
			}
			return fmt.Sprintf("0x%x", b)
		}

		// Handle nullable external block height
		extBlock := "null"
		if rec.ExternalBlockHeight != nil {
			extBlock = fmt.Sprintf("%d", *rec.ExternalBlockHeight)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			rec.Type,
			rec.Amount,
			formatHexShort(rec.FromAddress),
			formatHexShort(rec.ToAddress),
			formatHexShort(rec.InternalTxHash),
			formatHexShort(rec.ExternalTxHash),
			rec.Status,
			rec.BlockHeight,
			extBlock,
			timeStr,
		)
	}
	w.Flush()

	fmt.Printf("\n✅ Successfully retrieved %d records.\n", len(history))
	fmt.Println("\nNote: 'completed' means credited (deposits) or ready to claim (withdrawals). 'claimed' means withdrawn on Ethereum.")
}
