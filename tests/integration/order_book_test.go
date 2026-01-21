//go:build kwiltest

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ═══════════════════════════════════════════════════════════════
// TEST SUITE: Order Book Integration Tests
// ═══════════════════════════════════════════════════════════════
//
// These tests verify SDK-GO order book functionality against a live node.
// They focus on action invocation, result verification, and happy-path scenarios.
//
// COMPLEX SCENARIOS (tested at node level only):
// - Matching engine logic: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/matching_engine_test.go
// - Settlement with attestations: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/settlement_test.go
// - Fee distribution: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/fee_distribution_test.go
// - LP rewards: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/rewards_test.go
//
// Full node test reference: https://github.com/trufnetwork/node/tree/main/tests/streams/order_book
// ═══════════════════════════════════════════════════════════════

// ═══════════════════════════════════════════════════════════════
// MARKET OPERATIONS TESTS (Node-Level Testing Required)
// ═══════════════════════════════════════════════════════════════
//
// MARKET CREATION & QUERY OPERATIONS:
// These operations require the ethereum_bridge namespace for funding to create
// and interact with markets.
//
// Operations tested at node level:
//   - CreateMarket: Creates a new prediction market
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/market_creation_test.go#L47
//
//   - GetMarketInfo: Retrieves market details by ID
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/market_creation_test.go#L99
//
//   - GetMarketByHash: Retrieves market details by query hash
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/market_creation_test.go#L131
//
//   - ListMarkets: Returns paginated list of markets with optional filtering
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/market_creation_test.go#L171
//
//   - MarketExists: Checks if market exists by hash (lightweight)
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/market_creation_test.go#L229
//
//   - ValidateMarketCollateral: Checks binary token parity and vault balance
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/validate_market_collateral_test.go
//
// SDK-GO VALIDATION TESTS (below) verify input validation without requiring node infrastructure.
// ═══════════════════════════════════════════════════════════════

// TestOrderBookMarketValidation tests input validation for market operations
func TestOrderBookMarketValidation(t *testing.T) {
	testCreateMarketValidation(t, context.Background(), nil)
}

// testCreateMarketValidation tests input validation
func testCreateMarketValidation(t *testing.T, ctx context.Context, orderBook types.IOrderBook) {
	tests := []struct {
		name    string
		input   types.CreateMarketInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input",
			input: types.CreateMarketInput{
				QueryHash:    make([]byte, 32),
				SettleTime:   time.Now().Add(1 * time.Hour).Unix(),
				MaxSpread:    5,
				MinOrderSize: 100,
			},
			wantErr: false,
		},
		{
			name: "invalid hash length",
			input: types.CreateMarketInput{
				QueryHash:    make([]byte, 16),
				SettleTime:   time.Now().Add(1 * time.Hour).Unix(),
				MaxSpread:    5,
				MinOrderSize: 100,
			},
			wantErr: true,
			errMsg:  "must be exactly 32 bytes",
		},
		{
			name: "invalid settle time",
			input: types.CreateMarketInput{
				QueryHash:    make([]byte, 32),
				SettleTime:   -1,
				MaxSpread:    5,
				MinOrderSize: 100,
			},
			wantErr: true,
			errMsg:  "settle_time must be positive",
		},
		{
			name: "invalid max spread (too low)",
			input: types.CreateMarketInput{
				QueryHash:    make([]byte, 32),
				SettleTime:   time.Now().Add(1 * time.Hour).Unix(),
				MaxSpread:    0,
				MinOrderSize: 100,
			},
			wantErr: true,
			errMsg:  "max_spread must be between 1 and 50",
		},
		{
			name: "invalid max spread (too high)",
			input: types.CreateMarketInput{
				QueryHash:    make([]byte, 32),
				SettleTime:   time.Now().Add(1 * time.Hour).Unix(),
				MaxSpread:    51,
				MinOrderSize: 100,
			},
			wantErr: true,
			errMsg:  "max_spread must be between 1 and 50",
		},
		{
			name: "invalid min order size",
			input: types.CreateMarketInput{
				QueryHash:    make([]byte, 32),
				SettleTime:   time.Now().Add(1 * time.Hour).Unix(),
				MaxSpread:    5,
				MinOrderSize: 0,
			},
			wantErr: true,
			errMsg:  "min_order_size must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err, "Validation should fail")
				assert.Contains(t, err.Error(), tt.errMsg, "Error message should match")
			} else {
				require.NoError(t, err, "Validation should succeed")
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════
// ORDER OPERATIONS TESTS (Node-Level Testing Required)
// ═══════════════════════════════════════════════════════════════
//
// ORDER PLACEMENT & MODIFICATION OPERATIONS:
// These operations require the ethereum_bridge namespace for funding test wallets,
// which is only available in the full node test infrastructure.
//
// Operations tested at node level:
//   - PlaceBuyOrder: Place buy orders for YES or NO shares
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/buy_order_test.go
//
//   - PlaceSellOrder: Sell shares you own
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/sell_order_test.go
//
//   - PlaceSplitLimitOrder: Mint binary pairs and list unwanted side for sale
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/split_limit_order_test.go
//
//   - CancelOrder: Cancel open buy or sell orders
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/cancel_order_test.go
//
//   - ChangeBid: Atomically modify buy order price and amount
//   - ChangeAsk: Atomically modify sell order price and amount
//     Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/change_order_test.go
//
// SDK-GO VALIDATION TESTS (below) verify input validation without requiring node infrastructure.
// ═══════════════════════════════════════════════════════════════

// TestOrderBookOrderValidation tests input validation for order operations
func TestOrderBookOrderValidation(t *testing.T) {
	testOrderValidation(t, context.Background(), nil)
}

// testOrderValidation tests input validation for order operations
func testOrderValidation(t *testing.T, ctx context.Context, orderBook types.IOrderBook) {
	t.Run("PlaceBuyOrderValidation", func(t *testing.T) {
		tests := []struct {
			name    string
			input   types.PlaceBuyOrderInput
			wantErr bool
			errMsg  string
		}{
			{
				name: "valid input",
				input: types.PlaceBuyOrderInput{
					QueryID: 1,
					Outcome: true,
					Price:   56,
					Amount:  100,
				},
				wantErr: false,
			},
			{
				name: "invalid query id",
				input: types.PlaceBuyOrderInput{
					QueryID: 0,
					Outcome: true,
					Price:   56,
					Amount:  100,
				},
				wantErr: true,
				errMsg:  "query_id must be positive",
			},
			{
				name: "invalid price (too low)",
				input: types.PlaceBuyOrderInput{
					QueryID: 1,
					Outcome: true,
					Price:   0,
					Amount:  100,
				},
				wantErr: true,
				errMsg:  "price must be between 1 and 99",
			},
			{
				name: "invalid price (too high)",
				input: types.PlaceBuyOrderInput{
					QueryID: 1,
					Outcome: true,
					Price:   100,
					Amount:  100,
				},
				wantErr: true,
				errMsg:  "price must be between 1 and 99",
			},
			{
				name: "invalid amount (zero)",
				input: types.PlaceBuyOrderInput{
					QueryID: 1,
					Outcome: true,
					Price:   56,
					Amount:  0,
				},
				wantErr: true,
				errMsg:  "amount must be positive",
			},
			{
				name: "invalid amount (too large)",
				input: types.PlaceBuyOrderInput{
					QueryID: 1,
					Outcome: true,
					Price:   56,
					Amount:  1_000_000_001,
				},
				wantErr: true,
				errMsg:  "amount exceeds maximum",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.input.Validate()
				if tt.wantErr {
					require.Error(t, err, "Validation should fail")
					assert.Contains(t, err.Error(), tt.errMsg, "Error message should match")
				} else {
					require.NoError(t, err, "Validation should succeed")
				}
			})
		}
	})

	t.Run("CancelOrderValidation", func(t *testing.T) {
		tests := []struct {
			name    string
			input   types.CancelOrderInput
			wantErr bool
			errMsg  string
		}{
			{
				name: "valid buy order cancel",
				input: types.CancelOrderInput{
					QueryID: 1,
					Outcome: true,
					Price:   -56,
				},
				wantErr: false,
			},
			{
				name: "valid sell order cancel",
				input: types.CancelOrderInput{
					QueryID: 1,
					Outcome: true,
					Price:   56,
				},
				wantErr: false,
			},
			{
				name: "invalid price (zero - holdings)",
				input: types.CancelOrderInput{
					QueryID: 1,
					Outcome: true,
					Price:   0,
				},
				wantErr: true,
				errMsg:  "price cannot be 0 (holdings cannot be cancelled)",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.input.Validate()
				if tt.wantErr {
					require.Error(t, err, "Validation should fail")
					assert.Contains(t, err.Error(), tt.errMsg, "Error message should match")
				} else {
					require.NoError(t, err, "Validation should succeed")
				}
			})
		}
	})
}

// ═══════════════════════════════════════════════════════════════
// QUERY OPERATIONS TESTS (Node-Level Testing Required)
// ═══════════════════════════════════════════════════════════════
//
// MARKET QUERY OPERATIONS:
// These operations require the ethereum_bridge namespace for funding test wallets
// to create positions, holdings, and orders before querying them.
//
// Operations tested at node level:
//   - GetOrderBook: Retrieve all buy/sell orders for a market outcome
//     Returns all orders (excludes holdings with price=0)
//     Ordering: By price (best first), then by last_updated (FIFO)
//
//   - GetUserPositions: Retrieve caller's portfolio across all markets
//     Returns holdings (price=0), buy orders (price<0), and sell orders (price>0)
//
//   - GetMarketDepth: Returns aggregated volume per price level
//     Combines all orders at same price into single depth level
//
//   - GetBestPrices: Returns current bid/ask spread
//     BestBid: Highest buy price (nil if no bids)
//     BestAsk: Lowest sell price (nil if no asks)
//     Spread: BestAsk - BestBid (nil if either side empty)
//
//   - GetUserCollateral: Returns caller's total locked collateral value
//     TotalLocked: total locked collateral in wei
//     BuyOrdersLocked: collateral locked in buy orders
//     SharesValue: value of shares at $1.00 per share
//
// Full query operations testing:
//   Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/queries_test.go
//
// ═══════════════════════════════════════════════════════════════

// ═══════════════════════════════════════════════════════════════
// SETTLEMENT & REWARDS TESTS (Reference Only - Node-Level Required)
// ═══════════════════════════════════════════════════════════════
//
// SETTLEMENT OPERATIONS (Node-only):
// Full settlement testing requires attestation creation/signing infrastructure.
// Settlement process:
//   1. Create data provider
//   2. Create stream with 32-char ID
//   3. Insert outcome data (value = 1.0 for YES, -1.0 for NO)
//   4. Request attestation of outcome
//   5. Sign attestation (requires cryptographic precompiles)
//   6. Call settle_market with signed attestation
//   7. Verify market marked settled + winning outcome set
//   8. Verify collateral distributed to winners
//
// SDK Limitation: Can call SettleMarket() if attestation exists,
// but cannot create/sign attestations (node extension only)
//
// Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/settlement_test.go
// Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/settlement_payout_test.go
//
// LP REWARDS OPERATIONS (Node-only):
// Full rewards testing requires block sampling infrastructure.
// LP rewards process:
//   1. Create market with LP fee vault
//   2. Place paired buy orders (YES @ p + NO @ 100-p)
//   3. Sample LP rewards at block height (SampleLPRewards action)
//   4. Calculate scores based on spread and distance from midpoint
//   5. Distribute fees proportionally to scores
//
// Dynamic spread rules:
//   - Midpoint 0-29¢: 5¢ spread
//   - Midpoint 30-59¢: 4¢ spread
//   - Midpoint 60-79¢: 3¢ spread
//   - Midpoint 80-99¢: INELIGIBLE
//
// SDK Limitation: Can call SampleLPRewards(), but cannot control
// block advancement or verify LP score calculations (node-side state)
//
// Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/rewards_test.go
// Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/fee_distribution_test.go
// Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/fee_distribution_audit_test.go
//
// MATCHING ENGINE (Node-only):
// Matching engine testing requires node-level async matching infrastructure.
// Matching scenarios:
//
//   1. Direct Match:
//      - Buy and sell orders at same price
//      - Shares transferred from seller to buyer
//      - Collateral transferred from buyer to seller
//
//   2. Mint Match:
//      - Opposite buy orders (YES @ p + NO @ 100-p)
//      - Creates shares from nothing
//      - Both buyers receive holdings
//
//   3. Burn Match:
//      - Opposite sell orders (YES @ p + NO @ 100-p)
//      - Destroys shares
//      - Collateral returned to both sellers
//
//   4. Partial Fills:
//      - Large order matched against smaller orders
//      - Remaining amount stays in order book
//
//   5. Multi-Round Matching:
//      - Single order matched against multiple sequential orders
//      - FIFO ordering within price level
//
// SDK Limitation: Matching happens automatically during order placement
// (node-side logic), cannot be explicitly triggered or verified from SDK
//
// Reference: https://github.com/trufnetwork/node/blob/main/tests/streams/order_book/matching_engine_test.go
// ═══════════════════════════════════════════════════════════════

