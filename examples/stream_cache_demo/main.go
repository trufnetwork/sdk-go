package main

import (
	"context"
	"fmt"
	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"log"
	"time"

	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

func main() {
	ctx := context.Background()

	pk, err := crypto.Secp256k1PrivateKeyFromHex("0000000000000000000000000000000000000000000000000000000000000001")
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}
	signer := &auth.EthPersonalSigner{Key: *pk}

	// Initialize client (replace with your actual endpoint and signer)
	tnClient, err := tnclient.NewClient(
		ctx,
		"https://gateway.testnet.truf.network",
		tnclient.WithSigner(signer),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Example stream parameters
	streamId := "stf37ad83c0b92c7419925b7633c0e62"
	provider := "0x4710a8d8f0d845da110086812a32de6d90d7ff5c"

	// Load primitive actions
	actions, err := tnClient.LoadPrimitiveActions()
	if err != nil {
		log.Fatalf("Failed to load primitive actions: %v", err)
	}

	fmt.Println("=== TRUF SDK Cache Demo ===")
	fmt.Printf("Stream: %s\n", streamId)
	fmt.Printf("Provider: %s\n", provider)
	fmt.Println()

	// Demo 1: GetRecord with cache
	fmt.Println("1. GetRecord Operations")
	demoGetRecord(ctx, actions, provider, streamId)

	// Demo 2: GetIndex with cache
	fmt.Println("\n2. GetIndex Operations")
	demoGetIndex(ctx, actions, provider, streamId)

	// Demo 3: GetIndexChange with cache
	fmt.Println("\n3. GetIndexChange Operations")
	demoGetIndexChange(ctx, actions, provider, streamId)

	// Demo 4: GetFirstRecord with cache
	fmt.Println("\n4. GetFirstRecord Operations")
	demoGetFirstRecord(ctx, actions, provider, streamId)

	// Demo 5: Cache metadata analysis
	fmt.Println("\n5. Cache Metadata Analysis")
	demoCacheMetadata(ctx, actions, provider, streamId)

	// Demo 6: Performance comparison
	fmt.Println("\n6. Performance Comparison")
	demoPerformanceComparison(ctx, actions, provider, streamId)

	// Demo 7: Backward compatibility
	fmt.Println("\n7. Backward Compatibility")
	demoBackwardCompatibility(ctx, actions, provider, streamId)
}

func demoGetRecord(ctx context.Context, actions types.IAction, provider, streamId string) {
	from := int(time.Now().AddDate(0, 0, -7).Unix()) // 7 days ago
	to := int(time.Now().Unix())

	// First call - cache miss expected
	useCache := true
	result, err := actions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: provider,
		StreamId:     streamId,
		From:         &from,
		To:           &to,
		UseCache:     &useCache,
	})
	if err != nil {
		log.Printf("GetRecord failed: %v", err)
		return
	}

	fmt.Printf("  First call: %d records, Cache hit: %v\n",
		len(result.Results), result.Metadata.CacheHit)

	// Second call - cache hit expected
	result2, err := actions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: provider,
		StreamId:     streamId,
		From:         &from,
		To:           &to,
		UseCache:     &useCache,
	})
	if err != nil {
		log.Printf("GetRecord (repeat) failed: %v", err)
		return
	}

	fmt.Printf("  Second call: %d records, Cache hit: %v\n",
		len(result2.Results), result2.Metadata.CacheHit)

	// Analyze cache performance
	if result2.Metadata.CacheHit {
		fmt.Printf("  ✓ Cache working correctly\n")
		if dataAge := result2.Metadata.GetDataAge(); dataAge != nil {
			fmt.Printf("  Cache age: %v\n", *dataAge)
		}
	}
}

func demoGetIndex(ctx context.Context, actions types.IAction, provider, streamId string) {
	from := int(time.Now().AddDate(0, 0, -30).Unix()) // 30 days ago
	to := int(time.Now().Unix())
	baseDate := int(time.Now().AddDate(0, 0, -30).Unix()) // Base date for index

	useCache := true
	result, err := actions.GetIndex(ctx, types.GetIndexInput{
		DataProvider: provider,
		StreamId:     streamId,
		From:         &from,
		To:           &to,
		BaseDate:     &baseDate,
		UseCache:     &useCache,
	})
	if err != nil {
		log.Printf("GetIndex failed: %v", err)
		return
	}

	fmt.Printf("  Index query: %d records, Cache hit: %v\n",
		len(result.Results), result.Metadata.CacheHit)

	// Demonstrate index calculation caching
	result2, err := actions.GetIndex(ctx, types.GetIndexInput{
		DataProvider: provider,
		StreamId:     streamId,
		From:         &from,
		To:           &to,
		BaseDate:     &baseDate,
		UseCache:     &useCache,
	})
	if err != nil {
		log.Printf("GetIndex (repeat) failed: %v", err)
		return
	}

	fmt.Printf("  Repeat query: %d records, Cache hit: %v\n",
		len(result2.Results), result2.Metadata.CacheHit)

	if result2.Metadata.CacheHit {
		fmt.Printf("  ✓ Index calculations cached successfully\n")
	}
}

func demoGetIndexChange(ctx context.Context, actions types.IAction, provider, streamId string) {
	from := int(time.Now().AddDate(0, 0, -7).Unix()) // 7 days ago
	to := int(time.Now().Unix())
	timeInterval := 86400 // 24 hours

	useCache := true
	result, err := actions.GetIndexChange(ctx, types.GetIndexChangeInput{
		DataProvider: provider,
		StreamId:     streamId,
		From:         &from,
		To:           &to,
		TimeInterval: timeInterval,
		UseCache:     &useCache,
	})
	if err != nil {
		log.Printf("GetIndexChange failed: %v", err)
		return
	}

	fmt.Printf("  Index change query: %d records, Cache hit: %v\n",
		len(result.Results), result.Metadata.CacheHit)

	// Show index change values
	for i, change := range result.Results {
		if i >= 3 { // Show first 3 only
			fmt.Printf("  ... and %d more records\n", len(result.Results)-3)
			break
		}
		fmt.Printf("  Change %d: Time=%d, Value=%s%%\n",
			i+1, change.EventTime, change.Value.String())
	}
}

func demoGetFirstRecord(ctx context.Context, actions types.IAction, provider, streamId string) {
	useCache := true
	result, err := actions.GetFirstRecord(ctx, types.GetFirstRecordInput{
		DataProvider: provider,
		StreamId:     streamId,
		UseCache:     &useCache,
	})
	if err != nil {
		log.Printf("GetFirstRecord failed: %v", err)
		return
	}

	fmt.Printf("  First record query: %d records, Cache hit: %v\n",
		len(result.Results), result.Metadata.CacheHit)

	if len(result.Results) > 0 {
		first := result.Results[0]
		fmt.Printf("  First record: Time=%d, Value=%s\n",
			first.EventTime, first.Value.String())
	}

	// Demonstrate caching with 'After' parameter
	after := int(time.Now().AddDate(0, 0, -1).Unix()) // 1 day ago
	result2, err := actions.GetFirstRecord(ctx, types.GetFirstRecordInput{
		DataProvider: provider,
		StreamId:     streamId,
		After:        &after,
		UseCache:     &useCache,
	})
	if err != nil {
		log.Printf("GetFirstRecord (after) failed: %v", err)
		return
	}

	fmt.Printf("  First record after timestamp: %d records, Cache hit: %v\n",
		len(result2.Results), result2.Metadata.CacheHit)
}

func demoCacheMetadata(ctx context.Context, actions types.IAction, provider, streamId string) {
	useCache := true
	result, err := actions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: provider,
		StreamId:     streamId,
		UseCache:     &useCache,
	})
	if err != nil {
		log.Printf("Cache metadata demo failed: %v", err)
		return
	}

	metadata := result.Metadata
	fmt.Printf("  Detailed Cache Metadata:\n")
	fmt.Printf("    Cache Hit: %v\n", metadata.CacheHit)
	fmt.Printf("    Cache Disabled: %v\n", metadata.CacheDisabled)
	fmt.Printf("    Stream ID: %s\n", metadata.StreamId)
	fmt.Printf("    Data Provider: %s\n", metadata.DataProvider)
	fmt.Printf("    Rows Served: %d\n", metadata.RowsServed)

	if metadata.From != nil {
		fmt.Printf("    From: %d (%v)\n", *metadata.From, time.Unix(*metadata.From, 0))
	}
	if metadata.To != nil {
		fmt.Printf("    To: %d (%v)\n", *metadata.To, time.Unix(*metadata.To, 0))
	}
	if metadata.CachedAt != nil {
		fmt.Printf("    Cached At: %d (%v)\n", *metadata.CachedAt, time.Unix(*metadata.CachedAt, 0))
	}

	// Cache age analysis
	if dataAge := metadata.GetDataAge(); dataAge != nil {
		fmt.Printf("    Data Age: %v\n", *dataAge)

		// Test various expiration thresholds
		thresholds := []time.Duration{1 * time.Minute, 5 * time.Minute, 1 * time.Hour}
		for _, threshold := range thresholds {
			expired := metadata.IsExpired(threshold)
			status := "Fresh"
			if expired {
				status = "Expired"
			}
			fmt.Printf("    %s for %v threshold\n", status, threshold)
		}
	}
}

func demoPerformanceComparison(ctx context.Context, actions types.IAction, provider, streamId string) {
	from := int(time.Now().AddDate(0, 0, -1).Unix()) // 1 day ago
	to := int(time.Now().Unix())

	fmt.Printf("  Performance Comparison:\n")

	// Without cache
	start := time.Now()
	result1, err := actions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: provider,
		StreamId:     streamId,
		From:         &from,
		To:           &to,
		// UseCache is nil (default: disabled)
	})
	duration1 := time.Since(start)

	if err != nil {
		log.Printf("Performance test (no cache) failed: %v", err)
		return
	}

	fmt.Printf("    Without cache: %v, %d records\n", duration1, len(result1.Results))

	// With cache (first call)
	useCache := true
	start = time.Now()
	result2, err := actions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: provider,
		StreamId:     streamId,
		From:         &from,
		To:           &to,
		UseCache:     &useCache,
	})
	duration2 := time.Since(start)

	if err != nil {
		log.Printf("Performance test (cache miss) failed: %v", err)
		return
	}

	fmt.Printf("    With cache (miss): %v, %d records, Hit: %v\n",
		duration2, len(result2.Results), result2.Metadata.CacheHit)

	// With cache (second call - should hit)
	start = time.Now()
	result3, err := actions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: provider,
		StreamId:     streamId,
		From:         &from,
		To:           &to,
		UseCache:     &useCache,
	})
	duration3 := time.Since(start)

	if err != nil {
		log.Printf("Performance test (cache hit) failed: %v", err)
		return
	}

	fmt.Printf("    With cache (hit): %v, %d records, Hit: %v\n",
		duration3, len(result3.Results), result3.Metadata.CacheHit)

	// Calculate performance improvement
	if result3.Metadata.CacheHit && duration3 < duration1 {
		improvement := float64(duration1-duration3) / float64(duration1) * 100
		fmt.Printf("    ✓ Performance improvement: %.1f%%\n", improvement)
	}
}

func demoBackwardCompatibility(ctx context.Context, actions types.IAction, provider, streamId string) {
	fmt.Printf("  Backward Compatibility Test:\n")

	// Old-style API call (without UseCache parameter)
	result, err := actions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: provider,
		StreamId:     streamId,
		// No UseCache field - should work with existing code
	})
	if err != nil {
		log.Printf("Backward compatibility test failed: %v", err)
		return
	}

	fmt.Printf("    Old-style API: %d records, Cache hit: %v\n",
		len(result.Results), result.Metadata.CacheHit)

	// Verify metadata is still returned
	metadata := result.Metadata
	fmt.Printf("    Metadata returned: Stream=%s, Provider=%s, Rows=%d\n",
		metadata.StreamId, metadata.DataProvider, metadata.RowsServed)

	fmt.Printf("    ✓ Backward compatibility maintained\n")
}
