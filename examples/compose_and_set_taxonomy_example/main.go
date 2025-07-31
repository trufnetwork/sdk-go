package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

func main() {
	ctx := context.Background()

	// 1. Set up connection to local or mainnet node
	pk, err := crypto.Secp256k1PrivateKeyFromHex("your-private-key")
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}
	signer := &auth.EthPersonalSigner{Key: *pk}

	// Choose endpoint: local node or mainnet
	endpoint := "http://localhost:8484" // Change to mainnet URL if needed
	tnClient, err := tnclient.NewClient(
		ctx,
		endpoint,
		tnclient.WithSigner(signer),
	)
	if err != nil {
		log.Fatalf("Failed to create TN client: %v", err)
	}

	// 2. Generate unique stream ID for the composed stream
	composedStreamId := util.GenerateStreamId("market-composite-index")

	// Defer stream destruction to clean up resources
	defer func() {
		destroyTx, err := tnClient.DestroyStream(ctx, composedStreamId)
		if err != nil {
			log.Printf("Failed to destroy stream %s: %v", composedStreamId, err)
			return
		}

		// Wait for the destroy transaction to be mined
		txRes, err := tnClient.WaitForTx(ctx, destroyTx, time.Second*10)
		if err != nil {
			log.Printf("Error waiting for destroy transaction for stream %s: %v", composedStreamId, err)
		} else if txRes.Result.Code != uint32(kwiltypes.CodeOk) {
			log.Printf("Error destroying stream %s: %s", composedStreamId, txRes.Result.Log)
		} else {
			fmt.Printf("Successfully destroyed stream: %s\n", composedStreamId)
		}
	}()

	// 3. Create taxonomy with the specified stream IDs and weights
	// Note: These are existing streams in the network
	taxonomy := types.Taxonomy{
		ParentStream: tnClient.OwnStreamLocator(composedStreamId),
		TaxonomyItems: []types.TaxonomyItem{
			{
				ChildStream: types.StreamLocator{
					StreamId:     *util.NewRawStreamId("st96047782bbd4f43be169e58f7d051e"),
					DataProvider: util.Unsafe_NewEthereumAddressFromString("0x7f573e177ee7ec50eb5dee59478285054e4e74e7"),
				},
				Weight: 0.33,
			},
			{
				ChildStream: types.StreamLocator{
					StreamId:     *util.NewRawStreamId("st42ec7fe9e03d7e1369b8161adbde37"),
					DataProvider: util.Unsafe_NewEthereumAddressFromString("0xf3c816dc0576ec011e5d28367d7fa8c17bb8c6b7"),
				},
				Weight: 0.33,
			},
			{
				ChildStream: types.StreamLocator{
					StreamId:     *util.NewRawStreamId("stefa4eff1d1ea28db2b8c41af81e8ef"),
					DataProvider: util.Unsafe_NewEthereumAddressFromString("0xf3c816dc0576ec011e5d28367d7fa8c17bb8c6b7"),
				},
				Weight: 0.34,
			},
		},
	}

	// 4. Deploy composed stream with taxonomy using the DeployComposedStreamWithTaxonomy method
	fmt.Printf("Deploying composed stream with taxonomy: %s\n", composedStreamId)
	err = tnClient.DeployComposedStreamWithTaxonomy(ctx, composedStreamId, taxonomy)
	if err != nil {
		log.Fatalf("Failed to deploy composed stream with taxonomy: %v", err)
	}

	fmt.Printf("Successfully deployed composed stream with taxonomy: %s\n", composedStreamId)

	// 5. Verify the deployment by retrieving the taxonomy
	composedActions, err := tnClient.LoadComposedActions()
	if err != nil {
		log.Fatalf("Failed to load composed actions: %v", err)
	}

	retrievedTaxonomy, err := composedActions.DescribeTaxonomies(ctx, types.DescribeTaxonomiesParams{
		Stream:        tnClient.OwnStreamLocator(composedStreamId),
		LatestVersion: true,
	})
	if err != nil {
		log.Fatalf("Failed to retrieve taxonomy: %v", err)
	}

	fmt.Println("\nRetrieved Taxonomy:")
	fmt.Printf("Parent Stream: %s (Provider: %s)\n",
		retrievedTaxonomy.ParentStream.StreamId,
		retrievedTaxonomy.ParentStream.DataProvider.Address())

	fmt.Printf("Created At: %d\n", retrievedTaxonomy.CreatedAt)
	fmt.Printf("Group Sequence: %d\n", retrievedTaxonomy.GroupSequence)

	fmt.Println("Taxonomy Items:")
	for i, item := range retrievedTaxonomy.TaxonomyItems {
		fmt.Printf("  %d. Stream: %s (Provider: %s) - Weight: %.2f\n",
			i+1,
			item.ChildStream.StreamId,
			item.ChildStream.DataProvider.Address(),
			item.Weight,
		)
	}

	// 6. Optional: Try to retrieve some composed data (if available)
	// This might fail if the child streams don't have data in the expected date range
	fmt.Println("\nAttempting to retrieve composed stream data...")

	// Use a date range that might contain data
	from := int(time.Now().AddDate(0, -1, 0).Unix()) // 1 month ago

	tnClientAddress := tnClient.Address()
	records, err := composedActions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: tnClientAddress.Address(),
		StreamId:     composedStreamId.String(),
		From:         &from,
	})

	if err != nil {
		fmt.Printf("Note: Could not retrieve composed data (this is expected if child streams are empty): %v\n", err)
	} else {
		fmt.Printf("Retrieved %d composed records:\n", len(records.Results))
		for i, record := range records.Results {
			if i >= 5 { // Limit output to first 5 records
				fmt.Printf("... and %d more records\n", len(records.Results)-5)
				break
			}
			fmt.Printf("  Event Time: %d, Value: %s\n",
				record.EventTime,
				record.Value.String(),
			)
		}
	}

	fmt.Println("\nExample completed successfully. Stream will be automatically destroyed.")
}
