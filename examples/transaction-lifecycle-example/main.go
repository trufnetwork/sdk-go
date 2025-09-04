package main

import (
	"context"
	"fmt"
	kwilClientType "github.com/trufnetwork/kwil-db/core/client/types"
	kwilTypes "github.com/trufnetwork/kwil-db/core/types"
	"log"
	"strings"
	"time"

	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// deployStreamSafely demonstrates the proper way to deploy a stream with WaitForTx
func deployStreamSafely(ctx context.Context, client *tnclient.Client, streamId util.StreamId) error {
	fmt.Println("📝 Deploying stream...")

	// Step 1: Submit deployment transaction
	deployTx, err := client.DeployStream(ctx, streamId, types.StreamTypePrimitive)
	if err != nil {
		return fmt.Errorf("failed to submit deployment: %v", err)
	}
	fmt.Printf("   Deployment submitted: %s\n", deployTx.String())

	// Step 2: Wait for deployment to be mined
	fmt.Println("⏳ Waiting for deployment to be mined...")
	txRes, err := client.WaitForTx(ctx, deployTx, time.Second*5)
	if err != nil {
		return fmt.Errorf("failed to wait for deployment: %v", err)
	}

	// Step 3: Check if deployment was successful
	if txRes.Result.Code != uint32(kwilTypes.CodeOk) {
		return fmt.Errorf("deployment failed: %s", txRes.Result.Log)
	}

	fmt.Println("✅ Stream deployed and confirmed on-chain")
	return nil
}

// destroyStreamSafely demonstrates the proper way to destroy a stream with WaitForTx
func destroyStreamSafely(ctx context.Context, client *tnclient.Client, streamId util.StreamId) error {
	fmt.Println("🗑️  Destroying stream...")

	// Step 1: Submit destruction transaction
	destroyTx, err := client.DestroyStream(ctx, streamId)
	if err != nil {
		return fmt.Errorf("failed to submit destruction: %v", err)
	}
	fmt.Printf("   Destruction submitted: %s\n", destroyTx.String())

	// Step 2: Wait for destruction to be mined
	fmt.Println("⏳ Waiting for destruction to be mined...")
	txRes, err := client.WaitForTx(ctx, destroyTx, time.Second*5)
	if err != nil {
		return fmt.Errorf("failed to wait for destruction: %v", err)
	}

	// Step 3: Check if destruction was successful
	if txRes.Result.Code != uint32(kwilTypes.CodeOk) {
		return fmt.Errorf("destruction failed: %s", txRes.Result.Log)
	}

	fmt.Println("✅ Stream destroyed and confirmed on-chain")
	return nil
}

func main() {
	ctx := context.Background()

	// Set up client
	pk, err := crypto.Secp256k1PrivateKeyFromHex("<PRIVATE_KEY_HEX>")
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}
	signer := &auth.EthPersonalSigner{Key: *pk}

	endpoint := "https://gateway.mainnet.truf.network"
	tnClient, err := tnclient.NewClient(ctx, endpoint, tnclient.WithSigner(signer))
	if err != nil {
		log.Fatalf("Failed to create TN client: %v", err)
	}

	streamId := util.GenerateStreamId(fmt.Sprintf("lifecycle-demo-%d", time.Now().Unix()))

	fmt.Printf("🔄 Transaction Lifecycle Best Practices Demo\n")
	fmt.Printf("===========================================\n")
	fmt.Printf("Stream ID: %s\n", streamId)
	fmt.Printf("Endpoint: %s\n\n", endpoint)

	// Example 1: Proper stream deployment with WaitForTx
	fmt.Println("📋 EXAMPLE 1: Safe Stream Deployment")
	fmt.Println("-------------------------------------")
	if err := deployStreamSafely(ctx, tnClient, streamId); err != nil {
		log.Fatalf("Deployment failed: %v", err)
	}
	fmt.Println()

	// Load primitive actions
	primitiveActions, err := tnClient.LoadPrimitiveActions()
	if err != nil {
		log.Fatalf("Failed to load primitive actions: %v", err)
	}

	dataProvider := tnClient.Address()

	// Example 2: Demonstrate two ways to insert records synchronously
	fmt.Println("📋 EXAMPLE 2: Synchronous Record Insertion")
	fmt.Println("------------------------------------------")

	// Method A: Using WithSyncBroadcast
	fmt.Println("🅰️  Method A: Using WithSyncBroadcast(true)")
	testValue1 := 123.45
	insertTx1, err := primitiveActions.InsertRecord(ctx, types.InsertRecordInput{
		DataProvider: dataProvider.Address(),
		StreamId:     streamId.String(),
		EventTime:    int(time.Now().Unix()),
		Value:        testValue1,
	}, kwilClientType.WithSyncBroadcast(true))
	if err != nil {
		log.Fatalf("Failed to insert record with WithSyncBroadcast: %v", err)
	}
	fmt.Printf("   ✅ Record inserted and mined: %s\n", insertTx1.String())

	// Method B: Manual WaitForTx
	fmt.Println("🅱️  Method B: Manual WaitForTx")
	testValue2 := 456.78
	insertTx2, err := primitiveActions.InsertRecord(ctx, types.InsertRecordInput{
		DataProvider: dataProvider.Address(),
		StreamId:     streamId.String(),
		EventTime:    int(time.Now().Unix()) + 1,
		Value:        testValue2,
	})
	if err != nil {
		log.Fatalf("Failed to submit record insertion: %v", err)
	}
	fmt.Printf("   Transaction submitted: %s\n", insertTx2.String())
	fmt.Println("   ⏳ Waiting for insertion to be mined...")

	txRes, err := tnClient.WaitForTx(ctx, insertTx2, time.Second*5)
	if err != nil {
		log.Fatalf("Failed to wait for insertion: %v", err)
	}
	if txRes.Result.Code != uint32(kwilTypes.CodeOk) {
		log.Fatalf("Insertion failed: %s", txRes.Result.Log)
	}
	fmt.Printf("   ✅ Record inserted and confirmed: %s\n", insertTx2.String())
	fmt.Println()

	// Verify both records are accessible
	fmt.Println("📋 EXAMPLE 3: Verify Records After Synchronous Insertion")
	fmt.Println("-------------------------------------------------------")
	records, err := primitiveActions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: dataProvider.Address(),
		StreamId:     streamId.String(),
	})
	if err != nil {
		log.Fatalf("Failed to retrieve records: %v", err)
	}

	fmt.Printf("✅ Retrieved %d records from stream:\n", len(records.Results))
	for i, record := range records.Results {
		fmt.Printf("   Record %d: %s (Time: %d)\n", i+1, record.Value.String(), record.EventTime)
	}
	fmt.Println()

	// Example 3: Proper stream destruction with verification
	fmt.Println("📋 EXAMPLE 4: Safe Stream Destruction with Verification")
	fmt.Println("------------------------------------------------------")
	if err := destroyStreamSafely(ctx, tnClient, streamId); err != nil {
		log.Fatalf("Destruction failed: %v", err)
	}

	// Verify destruction by trying to insert (should fail)
	fmt.Println("🧪 Testing insertion after destruction...")
	insertTx, err := primitiveActions.InsertRecord(ctx, types.InsertRecordInput{
		DataProvider: dataProvider.Address(),
		StreamId:     streamId.String(),
		EventTime:    int(time.Now().Unix()) + 2,
		Value:        789.01,
	}, kwilClientType.WithSyncBroadcast(true))

	if err != nil {
		fmt.Printf("✅ PERFECT: Insertion failed immediately (transaction rejected)\n")
		fmt.Printf("   Error: %v\n", err)
	} else {
		// Transaction was submitted, now check if it succeeded or failed on-chain
		fmt.Printf("   Transaction submitted: %s\n", insertTx.String())
		fmt.Println("   ⏳ Waiting to see if transaction succeeds or fails...")

		txRes, waitErr := tnClient.WaitForTx(ctx, insertTx, time.Second*5)
		if waitErr != nil {
			fmt.Printf("✅ GOOD: Transaction failed to process\n")
			fmt.Printf("   Wait Error: %v\n", waitErr)
		} else if txRes.Result.Code != uint32(kwilTypes.CodeOk) {
			fmt.Printf("✅ PERFECT: Transaction was rejected on-chain (Code: %d)\n", txRes.Result.Code)
			fmt.Printf("   Transaction Error: %s\n", txRes.Result.Log)
		} else {
			fmt.Printf("⚠️  WARNING: Insertion succeeded after destruction!\n")
			fmt.Printf("   This indicates a race condition - stream destruction wasn't complete\n")
		}
	}

	// Try to retrieve records (should also fail)
	fmt.Println("🧪 Testing record retrieval after destruction...")
	_, err = primitiveActions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: dataProvider.Address(),
		StreamId:     streamId.String(),
	})

	if err != nil {
		fmt.Printf("✅ PERFECT: Record retrieval failed as expected\n")
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("⚠️  WARNING: Records still accessible after destruction\n")
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📚 KEY TAKEAWAYS:")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("✅ Use WaitForTx() for DeployStream and DestroyStream")
	fmt.Println("✅ Use WithSyncBroadcast(true) for record operations when order matters")
	fmt.Println("✅ Always check transaction result codes")
	fmt.Println("✅ Verify operations completed before proceeding with dependent actions")
	fmt.Println("⚠️  Async operations can cause race conditions in sequential workflows")
}
