package main

import (
	"context"
	"fmt"
	"log"

	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/contractsapi"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

func main() {
	// Configuration
	endpoint := "https://gateway.testnet.truf.network"
	// Use a dummy private key for read-only operation
	privateKeyHex := "0000000000000000000000000000000000000000000000000000000000000001"

	fmt.Println("--- Prediction Market Decoding Example (Real Data) ---")
	fmt.Printf("Endpoint: %s\n\n", endpoint)

	// 1. Initialize Client
	ctx := context.Background()
	pk, _ := crypto.Secp256k1PrivateKeyFromHex(privateKeyHex)
	signer := &auth.EthPersonalSigner{Key: *pk}

	client, err := tnclient.NewClient(ctx, endpoint, tnclient.WithSigner(signer))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// 2. Load Order Book
	orderBook, err := client.LoadOrderBook()
	if err != nil {
		log.Fatalf("Failed to load order book: %v", err)
	}

	// 3. List and Decode Markets (Both Active and Settled)
	settledStates := []bool{false, true}
	limit := 2

	for _, isSettled := range settledStates {
		stateStr := "ACTIVE"
		if isSettled {
			stateStr = "SETTLED"
		}
		fmt.Printf("--- Fetching Latest %s Markets ---\n", stateStr)

		markets, err := orderBook.ListMarkets(ctx, types.ListMarketsInput{
			Limit:         &limit,
			SettledFilter: &isSettled,
		})
		if err != nil {
			log.Fatalf("Failed to list %s markets: %v", stateStr, err)
		}

		fmt.Printf("Found %d %s markets. Decoding details...\n", len(markets), stateStr)

		// 4. Fetch and Decode each market
		for _, m := range markets {
			fmt.Printf("  Processing Market ID: %d\n", m.ID)

			// Fetch full info (including queryComponents)
			marketInfo, err := orderBook.GetMarketInfo(ctx, types.GetMarketInfoInput{
				QueryID: m.ID,
			})
			if err != nil {
				fmt.Printf("    Error fetching info: %v\n", err)
				continue
			}

			// Decode components
			data, err := contractsapi.DecodeMarketData(marketInfo.QueryComponents)
			if err != nil {
				fmt.Printf("    Error decoding data: %v\n", err)
				continue
			}

			fmt.Printf("    Market Type:   %s\n", data.Type)
			fmt.Printf("    Thresholds:    %v\n", data.Thresholds)
			fmt.Printf("    Action:        %s\n", data.ActionID)
			fmt.Printf("    Stream:        %s\n", data.StreamID)
		}
		fmt.Println()
	}
}
