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
	pk, err := crypto.Secp256k1PrivateKeyFromHex("your-private-key")
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}
	signer := &auth.EthPersonalSigner{Key: *pk}

	// Choose endpoint: Use "http://localhost:8484" for local node or "https://gateway.mainnet.truf.network" for mainnet
	endpoint := "https://gateway.mainnet.truf.network"
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

	// 3. Prepare index retrieval parameters
	now := time.Now()
	dayAgo := now.AddDate(0, 0, -1)
	fromTime := int(dayAgo.Unix())
	toTime := int(now.Unix())

	// 4. Load composed actions and retrieve indexes
	// Note: AI Index is a composed stream that aggregates data from multiple primitive streams
	composedActions, err := tnClient.LoadComposedActions()
	if err != nil {
		log.Fatalf("Failed to load primitive actions: %v", err)
	}

	// 5. Add prefix for specific data provider action
	prefix := "truflation_"
	records, err := composedActions.GetIndex(ctx, types.GetIndexInput{
		DataProvider: dataProvider,
		StreamId:     streamId,
		From:         &fromTime,
		To:           &toTime,
		Prefix:       &prefix,
	})
	if err != nil {
		log.Fatalf("Failed to retrieve indexes: %v", err)
	}

	// 5. Display retrieved records
	fmt.Println("\nTruflation AI Index:")
	fmt.Println("----------------------------")
	for _, record := range records {
		fmt.Printf("Event Time: %d, Value: %s\n",
			record.EventTime,
			record.Value.String(),
		)
	}
}
