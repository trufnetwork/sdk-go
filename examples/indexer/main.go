// Prediction Market Indexer API Example (Go)
//
// Demonstrates how to query the TrufNetwork Prediction Market Indexer
// to retrieve historical market data, settlements, snapshots, and LP rewards.
//
// For full API documentation, endpoint details, field descriptions, and architecture:
//   https://github.com/trufnetwork/node/blob/main/docs/prediction-market-indexer.md
//
// Usage:
//
//	cd examples/indexer && go run main.go
package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
)

// Indexer URLs
// Production: https://indexer.infra.truf.network
// Testnet:    http://ec2-52-15-66-172.us-east-2.compute.amazonaws.com:8080
var indexerURL = "http://ec2-52-15-66-172.us-east-2.compute.amazonaws.com:8080"

// Example wallet addresses (from testnet order book examples)
const (
	buyerWallet = "1c6790935a3a1A6B914399Ba743BEC8C41Fe89Fb"
	lp1Wallet   = "c11Ff6d3cC60823EcDCAB1089F1A4336053851EF"
)

// API response types

type APIResponse[T any] struct {
	OK   bool `json:"ok"`
	Data T    `json:"data"`
}

type Market struct {
	QueryID        int32  `json:"query_id"`
	QueryHash      string `json:"query_hash"`
	SettleTime     int64  `json:"settle_time"`
	Settled        bool   `json:"settled"`
	WinningOutcome *bool  `json:"winning_outcome"`
	SettledAt      *int64 `json:"settled_at"`
	CreatedAt      int64  `json:"created_at"`
	Creator        string `json:"creator"`
	MaxSpread      int32  `json:"max_spread"`
	MinOrderSize   string `json:"min_order_size"`
	Bridge         string `json:"bridge"`
}

type Snapshot struct {
	BlockHeight   int64  `json:"block_height"`
	Timestamp     int64  `json:"timestamp"`
	YesBidPrice   *int32 `json:"yes_bid_price"`
	YesAskPrice   *int32 `json:"yes_ask_price"`
	NoBidPrice    *int32 `json:"no_bid_price"`
	NoAskPrice    *int32 `json:"no_ask_price"`
	YesVolume     *int64 `json:"yes_volume"`
	NoVolume      *int64 `json:"no_volume"`
	MidpointPrice *int32 `json:"midpoint_price"`
	Spread        *int32 `json:"spread"`
}

type Settlement struct {
	QueryID            int32  `json:"query_id"`
	WinningShares      int64  `json:"winning_shares"`
	LosingShares       int64  `json:"losing_shares"`
	Payout             string `json:"payout"`
	RefundedCollateral string `json:"refunded_collateral"`
	Timestamp          int64  `json:"timestamp"`
}

type SettlementsData struct {
	WalletAddress string       `json:"wallet_address"`
	Settlements   []Settlement `json:"settlements"`
	TotalWon      int64        `json:"total_won"`
	TotalLost     int64        `json:"total_lost"`
}

type Reward struct {
	QueryID            int32   `json:"query_id"`
	TotalRewardPercent float64 `json:"total_reward_percent"`
	RewardAmount       string  `json:"reward_amount"`
	BlocksSampled      int32   `json:"blocks_sampled"`
	DistributedAt      int64   `json:"distributed_at"`
}

type RewardsData struct {
	WalletAddress string   `json:"wallet_address"`
	Rewards       []Reward `json:"rewards"`
	TotalRewards  string   `json:"total_rewards"`
}

func weiToUSDC(weiStr string) string {
	wei := new(big.Float)
	wei.SetString(weiStr)
	divisor := new(big.Float).SetFloat64(1e18)
	result := new(big.Float).Quo(wei, divisor)
	return result.Text('f', 4)
}

func fetchJSON(url string, target any) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func queryMarkets() ([]Market, error) {
	fmt.Println("============================================================")
	fmt.Println("Endpoint 1: List Historical Markets")
	fmt.Println("============================================================")

	var result APIResponse[[]Market]
	err := fetchJSON(fmt.Sprintf("%s/v0/prediction-market/markets?limit=5", indexerURL), &result)
	if err != nil {
		return nil, err
	}

	fmt.Printf("\nFound %d markets:\n", len(result.Data))
	for _, m := range result.Data {
		status := "ACTIVE"
		if m.Settled {
			status = "SETTLED"
		}
		outcome := ""
		if m.WinningOutcome != nil {
			if *m.WinningOutcome {
				outcome = " (YES wins)"
			} else {
				outcome = " (NO wins)"
			}
		}
		fmt.Printf("  Market #%d: %s%s\n", m.QueryID, status, outcome)
		fmt.Printf("    Hash: %s...\n", m.QueryHash[:16])
		fmt.Printf("    Bridge: %s, Max Spread: %dc\n", m.Bridge, m.MaxSpread)
	}

	return result.Data, nil
}

func querySnapshots(queryID int32) error {
	fmt.Println("\n============================================================")
	fmt.Printf("Endpoint 2: Order Book Snapshots (Market #%d)\n", queryID)
	fmt.Println("============================================================")

	var result APIResponse[[]Snapshot]
	err := fetchJSON(fmt.Sprintf("%s/v0/prediction-market/markets/%d/snapshots?limit=5", indexerURL, queryID), &result)
	if err != nil {
		return err
	}

	if len(result.Data) == 0 {
		fmt.Println("  No snapshots found for this market.")
		return nil
	}

	fmt.Printf("\nFound %d snapshots:\n", len(result.Data))
	for _, s := range result.Data {
		mid := "N/A"
		spread := "N/A"
		if s.MidpointPrice != nil {
			mid = fmt.Sprintf("%d", *s.MidpointPrice)
		}
		if s.Spread != nil {
			spread = fmt.Sprintf("%d", *s.Spread)
		}
		fmt.Printf("  Block %d: midpoint=%sc, spread=%sc\n", s.BlockHeight, mid, spread)
	}

	return nil
}

func querySettlements(wallet string) error {
	fmt.Println("\n============================================================")
	fmt.Printf("Endpoint 3: Participant Settlements (%s...)\n", wallet[:10])
	fmt.Println("============================================================")

	var result APIResponse[SettlementsData]
	err := fetchJSON(fmt.Sprintf("%s/v0/prediction-market/participants/%s/settlements?limit=10", indexerURL, wallet), &result)
	if err != nil {
		return err
	}

	data := result.Data
	fmt.Printf("\n  Wallet: %s\n", data.WalletAddress)
	fmt.Printf("  Total Won: %d, Total Lost: %d\n", data.TotalWon, data.TotalLost)

	for _, s := range data.Settlements {
		fmt.Printf("\n  Market #%d:\n", s.QueryID)
		fmt.Printf("    Winning shares: %d, Losing shares: %d\n", s.WinningShares, s.LosingShares)
		fmt.Printf("    Payout: %s USDC, Refund: %s USDC\n", weiToUSDC(s.Payout), weiToUSDC(s.RefundedCollateral))
	}

	return nil
}

func queryRewards(wallet string) error {
	fmt.Println("\n============================================================")
	fmt.Printf("Endpoint 4: LP Rewards (%s...)\n", wallet[:10])
	fmt.Println("============================================================")

	var result APIResponse[RewardsData]
	err := fetchJSON(fmt.Sprintf("%s/v0/prediction-market/participants/%s/rewards?limit=10", indexerURL, wallet), &result)
	if err != nil {
		return err
	}

	data := result.Data
	fmt.Printf("\n  Wallet: %s\n", data.WalletAddress)
	fmt.Printf("  Total Rewards: %s USDC\n", weiToUSDC(data.TotalRewards))

	for _, r := range data.Rewards {
		fmt.Printf("\n  Market #%d:\n", r.QueryID)
		fmt.Printf("    Reward: %s USDC (%.2f%%)\n", weiToUSDC(r.RewardAmount), r.TotalRewardPercent)
		fmt.Printf("    Blocks Sampled: %d\n", r.BlocksSampled)
	}

	return nil
}

func main() {
	fmt.Println("TrufNetwork Prediction Market Indexer - Go Example")
	fmt.Println("Indexer URL:", indexerURL)
	fmt.Println()

	// 1. List markets
	markets, err := queryMarkets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying markets: %v\n", err)
		os.Exit(1)
	}

	// 2. Snapshots for the most recent market
	if len(markets) > 0 {
		if err := querySnapshots(markets[0].QueryID); err != nil {
			fmt.Fprintf(os.Stderr, "Error querying snapshots: %v\n", err)
		}
	}

	// 3. Settlement results for buyer
	if err := querySettlements(buyerWallet); err != nil {
		fmt.Fprintf(os.Stderr, "Error querying settlements: %v\n", err)
	}

	// 4. LP rewards for LP1
	if err := queryRewards(lp1Wallet); err != nil {
		fmt.Fprintf(os.Stderr, "Error querying rewards: %v\n", err)
	}

	fmt.Println("\n============================================================")
	fmt.Println("Done! All 4 indexer endpoints demonstrated.")
	fmt.Println("============================================================")
}
