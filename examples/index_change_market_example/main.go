// Creates a set of prediction markets that settle on how far an index moved,
// rather than on the level it published.
//
// A stream that publishes an index level (CPI at ~335, PCE at ~131) cannot back
// an inflation-rate market through the value actions: those compare a level
// against a rate. index_change_in_range computes the percentage change over an
// interval and returns one boolean, so a market can be struck on the rate while
// reading a stream that only publishes levels.
//
// Run with:
//
//	go run ./examples/index_change_market_example
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/contractsapi"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

const (
	endpoint = "https://gateway.testnet.truf.network"

	// WARNING: throwaway key, provided for testnet examples only. Do not use it
	// in production or hold anything of value in this wallet. It is the same
	// OBMarketCreator wallet the sdk-py order-book examples use
	// (0x32a46917DF74808b9aDD7DC6eF0c34520412FDF3).
	//
	// Override it with PRIVATE_KEY to create the markets from your own wallet.
	defaultPrivateKey = "a537437df2ed8d3bcb3b99b4f88818cadf8ac365cd0a66595bb50973ac4ecf51"

	// A testnet stream with roughly fifteen years of daily history, so a
	// year-over-year lookback lands on real data.
	dataProvider = "0x4710a8d8f0d845da110086812a32de6d90d7ff5c"
	streamID     = "st9f212b7c208afd83705cc0dbdadfe8"

	// Fallback observation point, used only if the stream's latest event time
	// cannot be read. The markets below observe their settlement time instead.
	fallbackObservedAt = int64(1783296000)

	yearInSeconds = int64(31536000)

	// Collateral namespace. The 2 TRUF market-creation fee is taken from
	// hoodi_tt regardless of this choice.
	bridge = "hoodi_tt2"
)

// The buckets of one market set. Every bucket shares an observation time and an
// interval and differs only in where it is struck, so exactly one of them can
// settle YES.
//
// Bounds are half-open, [min, max): a change landing exactly on a boundary
// belongs to the bucket above it. That is what lets the set tile the number line
// without two buckets settling YES at once. The outer two are struck with an
// open tail, which is what nil means here -- the node rejects a market with both
// tails open, since that is every outcome at once.
var buckets = []struct {
	question string
	min, max *string
}{
	{"below 2%", nil, str("2")},
	{"between 2% and 3%", str("2"), str("3")},
	{"3% or more", str("3"), nil},
}

func str(v string) *string { return &v }

func main() {
	ctx := context.Background()

	privateKey := os.Getenv("PRIVATE_KEY")
	if privateKey == "" {
		privateKey = defaultPrivateKey
	}
	pk, err := crypto.Secp256k1PrivateKeyFromHex(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	client, err := tnclient.NewClient(ctx, endpoint,
		tnclient.WithSigner(&auth.EthPersonalSigner{Key: *pk}))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	address := client.Address()

	fmt.Println("=== Index-change prediction markets ===")
	fmt.Printf("Endpoint: %s\n", endpoint)
	fmt.Printf("Wallet:   %s\n\n", address.Address())

	observedAt := describeTheRate(ctx, client)
	readTheBucketsOffline(observedAt)
	createTheMarkets(ctx, client)
}

// describeTheRate shows the number a market on this stream settles against, and
// returns the point in the stream it was measured at.
//
// The point is read from the stream rather than hardcoded, so this stays honest
// as the stream advances. A published index is not a live price -- CPI-style
// data lands monthly -- so the latest reading is routinely weeks old, and saying
// which day it belongs to matters more than calling it "current".
//
// Both calls are plain reads and cost nothing.
func describeTheRate(ctx context.Context, client *tnclient.Client) int64 {
	fmt.Println("--- What such a market measures ---")

	actions, err := client.LoadActions()
	if err != nil {
		log.Fatalf("Failed to load actions: %v", err)
	}

	observedAt := fallbackObservedAt

	// get_last_record($data_provider, $stream_id, $before, $frozen_at, $use_cache)
	latest, err := actions.CallProcedure(ctx, "get_last_record", []any{
		dataProvider, streamID, nil, nil, false,
	})
	if err == nil && len(latest.Values) > 0 {
		if at, ok := eventTime(latest.ColumnNames, latest.Values[0]); ok {
			observedAt = at
		}
	}

	// get_index_change($data_provider, $stream_id, $from, $to, $frozen_at,
	//                  $base_time, $time_interval, $use_cache)
	result, err := actions.CallProcedure(ctx, "get_index_change", []any{
		dataProvider, streamID, observedAt, observedAt, nil, nil, yearInSeconds, false,
	})
	if err != nil {
		fmt.Printf("Could not read the rate: %v\n\n", err)
		return observedAt
	}
	if len(result.Values) == 0 {
		fmt.Print("The stream has no value at that point.\n\n")
		return observedAt
	}

	row := result.Values[0]
	fmt.Printf("Stream %s moved %v%% over the year ending %s,\n",
		streamID, row[len(row)-1], time.Unix(observedAt, 0).UTC().Format("2006-01-02"))
	fmt.Printf("which is its latest reading, not today's date.\n\n")
	return observedAt
}

// eventTime pulls the event_time column out of a result row by name, so a change
// in column order does not silently turn a value into a timestamp.
func eventTime(columns []string, row []any) (int64, bool) {
	for i, name := range columns {
		if name != "event_time" || i >= len(row) {
			continue
		}
		switch v := row[i].(type) {
		case int64:
			return v, true
		case int:
			return int64(v), true
		case string:
			if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

// readTheBucketsOffline builds each bucket's query components and reads them
// back without touching the network, which is where the encoding contract is
// easiest to see.
func readTheBucketsOffline(observedAt int64) {
	fmt.Println("--- The bucket set, built and decoded locally ---")

	for _, bucket := range buckets {
		components, err := contractsapi.BuildIndexChangeInRangeQueryComponents(
			types.IndexChangeInRangeInput{
				DataProvider: dataProvider,
				StreamID:     streamID,
				Timestamp:    observedAt,
				TimeInterval: yearInSeconds,
				MinChange:    bucket.min,
				MaxChange:    bucket.max,
			})
		if err != nil {
			log.Fatalf("Failed to build %q: %v", bucket.question, err)
		}

		market, err := contractsapi.DecodeMarketData(components)
		if err != nil {
			log.Fatalf("Failed to decode %q: %v", bucket.question, err)
		}

		lower, upper, err := contractsapi.BucketBoundsFromMarketData(market)
		if err != nil {
			log.Fatalf("Failed to read the bounds of %q: %v", bucket.question, err)
		}

		// An open tail decodes as an empty string holding its slot, not as a
		// shorter list. Dropping the empty entry would slide the surviving bound
		// into the other position and turn "below 2%" into "2% and up".
		fmt.Printf("  %-20s type=%s thresholds=%q bounds=[%s, %s)\n",
			bucket.question, market.Type, market.Thresholds, bound(lower), bound(upper))
	}
	fmt.Println()
}

// createTheMarkets puts the set on testnet. Each market costs 2 TRUF, taken from
// the wallet's hoodi_tt balance.
func createTheMarkets(ctx context.Context, client *tnclient.Client) {
	fmt.Println("--- Creating the set on testnet (2 TRUF each) ---")

	orderBook, err := client.LoadOrderBook()
	if err != nil {
		log.Fatalf("Failed to load order book: %v", err)
	}
	// The binary market helpers hang off the concrete type; IOrderBook carries
	// only the generic CreateMarket.
	book, ok := orderBook.(*contractsapi.OrderBook)
	if !ok {
		log.Fatalf("Expected *contractsapi.OrderBook, got %T", orderBook)
	}

	// The market observes the stream at its settlement time, so it cannot be
	// resolved before then. Using a fresh time each run also keeps each run's
	// markets distinct: a market's identity is a hash of its query components,
	// and re-creating an identical market is refused.
	settleTime := time.Now().Add(30 * time.Minute).Unix()

	for _, bucket := range buckets {
		txHash, err := book.CreateIndexChangeInRangeMarket(ctx,
			contractsapi.CreateIndexChangeInRangeMarketInput{
				Bridge:       bridge,
				DataProvider: dataProvider,
				StreamID:     streamID,
				Timestamp:    settleTime,
				TimeInterval: yearInSeconds,
				MinChange:    bucket.min,
				MaxChange:    bucket.max,
				SettleTime:   settleTime,
				MaxSpread:    10,
				MinOrderSize: 1_000_000_000_000_000_000, // 1 token
			})
		if err != nil {
			fmt.Printf("  %-20s failed: %v\n", bucket.question, err)
			continue
		}

		if _, err := client.WaitForTx(ctx, txHash, 2*time.Second); err != nil {
			fmt.Printf("  %-20s not confirmed: %v\n", bucket.question, err)
			continue
		}
		fmt.Printf("  %-20s tx %s\n", bucket.question, txHash)
	}

	fmt.Printf("\nSettles at %s.\n",
		time.Unix(settleTime, 0).UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Println("Read any market back with examples/decode_market_example.")
}

func bound(value *float64) string {
	if value == nil {
		return "open"
	}
	return fmt.Sprintf("%g", *value)
}
