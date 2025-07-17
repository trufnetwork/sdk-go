package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

func main() {
	ctx := context.Background()

	// 1. Set up local node connection
	// Replace with your actual private key
	pk, err := crypto.Secp256k1PrivateKeyFromHex("0000000000000000000000000000000000000000000000000000000000000001")
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}
	signer := &auth.EthPersonalSigner{Key: *pk}

	// Choose endpoint: Use "http://localhost:8484" for local node or "https://gateway.mainnet.truf.network" for mainnet
	endpoint := "https://gateway.mainnet.truf.network" // Change to mainnet URL if needed
	tnClient, err := tnclient.NewClient(
		ctx,
		endpoint,
		tnclient.WithSigner(signer),
	)
	if err != nil {
		log.Fatalf("Failed to create TN client: %v", err)
	}

	// 2. AI Index stream details
	dataProvider := "0x4710a8d8f0d845da110086812a32de6d90d7ff5c"
	streamId := "stai0000000000000000000000000000"

	// 3. Prepare record retrieval parameters (last week)
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	fromTime := int(weekAgo.Unix())
	toTime := int(now.Unix())

	// 4. Load composed actions and retrieve records
	// Note: AI Index is a composed stream that aggregates data from multiple primitive streams
	composedActions, err := tnClient.LoadComposedActions()
	if err != nil {
		log.Fatalf("Failed to load primitive actions: %v", err)
	}

	records, err := composedActions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: dataProvider,
		StreamId:     streamId,
		From:         &fromTime,
		To:           &toTime,
	})
	if err != nil {
		log.Fatalf("Failed to retrieve records: %v", err)
	}

	// 5. Display retrieved records
	fmt.Println("\nAI Index Results:")
	fmt.Println("----------------------------")
	for _, record := range records.Results {
		fmt.Printf("Event Time: %d, Value: %s\n",
			record.EventTime,
			record.Value.String(),
		)
	}

	// 6. Display cache metadata
	fmt.Printf("\nCache Metadata:\n")
	fmt.Printf("Cache Hit: %v\n", records.Metadata.CacheHit)
	fmt.Printf("Rows Served: %d\n", records.Metadata.RowsServed)
}
