package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
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

	// 2. Generate unique stream IDs
	// We'll create two primitive streams and one composed stream
	primitiveStreamId1 := util.GenerateStreamId("market-sentiment-stream")
	primitiveStreamId2 := util.GenerateStreamId("economic-indicator-stream")
	composedStreamId := util.GenerateStreamId("composite-market-index")

	// Defer stream destruction to clean up resources
	defer func() {
		// Destroy streams in reverse order of creation
		streamIds := []util.StreamId{
			composedStreamId,
			primitiveStreamId1,
			primitiveStreamId2,
		}

		for _, streamId := range streamIds {
			destroyTx, err := tnClient.DestroyStream(ctx, streamId)
			if err != nil {
				log.Printf("Failed to destroy stream %s: %v", streamId, err)
				continue
			}

			// Wait for the destroy transaction to be mined
			_, err = tnClient.WaitForTx(ctx, destroyTx, time.Second*5)
			if err != nil {
				log.Printf("Error waiting for destroy transaction for stream %s: %v", streamId, err)
			} else {
				fmt.Printf("Successfully destroyed stream: %s\n", streamId)
			}
		}
	}()

	// 3. Deploy Primitive Streams
	deployPrimitiveStream := func(streamId util.StreamId) error {
		deployTx, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
		if err != nil {
			return fmt.Errorf("failed to deploy primitive stream %s: %v", streamId, err)
		}
		_, err = tnClient.WaitForTx(ctx, deployTx, time.Second*5)
		return err
	}

	if err := deployPrimitiveStream(primitiveStreamId1); err != nil {
		log.Fatalf("Deployment error: %v", err)
	}
	if err := deployPrimitiveStream(primitiveStreamId2); err != nil {
		log.Fatalf("Deployment error: %v", err)
	}

	// 4. Deploy Composed Stream
	deployTx, err := tnClient.DeployStream(ctx, composedStreamId, types.StreamTypeComposed)
	if err != nil {
		log.Fatalf("Failed to deploy composed stream: %v", err)
	}
	_, err = tnClient.WaitForTx(ctx, deployTx, time.Second*5)
	if err != nil {
		log.Fatalf("Failed to wait for composed stream deployment: %v", err)
	}

	// 5. Load Primitive Actions to Insert Records
	primitiveActions, err := tnClient.LoadPrimitiveActions()
	if err != nil {
		log.Fatalf("Failed to load primitive actions: %v", err)
	}

	// 6. Insert Records into Primitive Streams
	dataProvider := tnClient.Address()
	insertRecords := func(streamId string, records []types.InsertRecordInput) error {
		insertTx, err := primitiveActions.InsertRecords(ctx, records)
		if err != nil {
			return fmt.Errorf("failed to insert records into stream %s: %v", streamId, err)
		}
		_, err = tnClient.WaitForTx(ctx, insertTx, time.Second*5)
		return err
	}

	// Market Sentiment Stream Records
	sentimentRecords := []types.InsertRecordInput{
		{
			DataProvider: dataProvider.Address(),
			StreamId:     primitiveStreamId1.String(),
			EventTime:    int(time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).Unix()),
			Value:        75.5, // Sentiment score
		},
		{
			DataProvider: dataProvider.Address(),
			StreamId:     primitiveStreamId1.String(),
			EventTime:    int(time.Date(2023, 6, 2, 0, 0, 0, 0, time.UTC).Unix()),
			Value:        82.3,
		},
	}

	// Economic Indicator Stream Records
	indicatorRecords := []types.InsertRecordInput{
		{
			DataProvider: dataProvider.Address(),
			StreamId:     primitiveStreamId2.String(),
			EventTime:    int(time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).Unix()),
			Value:        105.7, // Economic indicator value
		},
		{
			DataProvider: dataProvider.Address(),
			StreamId:     primitiveStreamId2.String(),
			EventTime:    int(time.Date(2023, 6, 2, 0, 0, 0, 0, time.UTC).Unix()),
			Value:        108.2,
		},
	}

	if err := insertRecords(primitiveStreamId1.String(), sentimentRecords); err != nil {
		log.Fatalf("Failed to insert market sentiment records: %v", err)
	}
	if err := insertRecords(primitiveStreamId2.String(), indicatorRecords); err != nil {
		log.Fatalf("Failed to insert economic indicator records: %v", err)
	}

	// 7. Set Taxonomy for Composed Stream
	composedActions, err := tnClient.LoadComposedActions()
	if err != nil {
		log.Fatalf("Failed to load composed actions: %v", err)
	}

	// Define taxonomy:
	// - Market Sentiment Stream (weight: 0.6)
	// - Economic Indicator Stream (weight: 0.4)
	startDate := int(time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).Unix())
	taxonomyTx, err := composedActions.InsertTaxonomy(ctx, types.Taxonomy{
		ParentStream: tnClient.OwnStreamLocator(composedStreamId),
		TaxonomyItems: []types.TaxonomyItem{
			{
				ChildStream: tnClient.OwnStreamLocator(primitiveStreamId1),
				Weight:      0.6,
			},
			{
				ChildStream: tnClient.OwnStreamLocator(primitiveStreamId2),
				Weight:      0.4,
			},
		},
		StartDate: &startDate,
	})
	if err != nil {
		log.Fatalf("Failed to set taxonomy: %v", err)
	}
	_, err = tnClient.WaitForTx(ctx, taxonomyTx, time.Second*5)
	if err != nil {
		log.Fatalf("Failed to wait for taxonomy transaction: %v", err)
	}

	// 8. Retrieve Records from Composed Stream
	records, err := composedActions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: dataProvider.Address(),
		StreamId:     composedStreamId.String(),
		From:         &startDate,
	})
	if err != nil {
		log.Fatalf("Failed to retrieve composed stream records: %v", err)
	}

	fmt.Println("Composite Market Index Records:")
	for _, record := range records {
		fmt.Printf("Event Time: %d, Value: %s\n",
			record.EventTime,
			record.Value.String(),
		)
	}

	fmt.Println("\nExample completed successfully. Streams will be automatically destroyed.")
}
