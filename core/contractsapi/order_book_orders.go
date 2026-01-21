package contractsapi

import (
	"context"

	"github.com/pkg/errors"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	kwilClientType "github.com/kwilteam/kwil-db/core/types/client"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ═══════════════════════════════════════════════════════════════
// ORDER PLACEMENT OPERATIONS
// ═══════════════════════════════════════════════════════════════

// PlaceBuyOrder places a buy order for YES or NO shares
// Maps to: place_buy_order($query_id, $outcome, $price, $amount)
// Migration: 032-order-book-actions.sql:920-1070
//
// Collateral Locked: amount × price × 10^16 wei
// Example: 10 shares @ $0.56 = 10 × 56 × 10^16 = 5.6 × 10^18 wei (5.6 TRUF)
func (o *OrderBook) PlaceBuyOrder(ctx context.Context, input types.PlaceBuyOrderInput,
	opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	if err := input.Validate(); err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return o.execute(ctx, "place_buy_order", [][]any{{
		input.QueryID,
		input.Outcome,
		input.Price,
		input.Amount,
	}}, opts...)
}

// PlaceSellOrder places a sell order for shares you own
// Maps to: place_sell_order($query_id, $outcome, $price, $amount)
// Migration: 032-order-book-actions.sql:1099-1244
//
// Prerequisites: User must own >= Amount shares of Outcome
// Shares are moved from holdings (price=0) to sell order book (price>0)
func (o *OrderBook) PlaceSellOrder(ctx context.Context, input types.PlaceSellOrderInput,
	opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	if err := input.Validate(); err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return o.execute(ctx, "place_sell_order", [][]any{{
		input.QueryID,
		input.Outcome,
		input.Price,
		input.Amount,
	}}, opts...)
}

// PlaceSplitLimitOrder mints binary pairs and lists unwanted side for sale
// Maps to: place_split_limit_order($query_id, $true_price, $amount)
// Migration: 032-order-book-actions.sql:1295-1459
//
// Behavior:
// - Mints Amount YES shares + Amount NO shares
// - Holds YES shares (price=0, not listed)
// - Sells NO shares at price (100 - TruePrice)
// - Locks Amount × $1.00 collateral (amount × 10^18 wei)
//
// Example:
//
//	TruePrice=56 → Holds YES, Sells NO @ $0.44
//	Amount=100 → Locks 100 TRUF, creates 100 YES holdings + 100 NO sell @ 44¢
func (o *OrderBook) PlaceSplitLimitOrder(ctx context.Context, input types.PlaceSplitLimitOrderInput,
	opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	if err := input.Validate(); err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return o.execute(ctx, "place_split_limit_order", [][]any{{
		input.QueryID,
		input.TruePrice,
		input.Amount,
	}}, opts...)
}

// CancelOrder cancels an open buy or sell order
// Maps to: cancel_order($query_id, $outcome, $price)
// Migration: 032-order-book-actions.sql:1506-1646
//
// Behavior:
// - For buy orders (price < 0): Refunds locked collateral
// - For sell orders (price > 0): Returns shares to holdings (price=0)
// - Holdings (price=0) cannot be cancelled (use PlaceSellOrder to list them)
//
// Error cases:
// - Order not found or doesn't belong to caller
// - Market already settled
// - price=0 (holdings can't be cancelled)
func (o *OrderBook) CancelOrder(ctx context.Context, input types.CancelOrderInput,
	opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	if err := input.Validate(); err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return o.execute(ctx, "cancel_order", [][]any{{
		input.QueryID,
		input.Outcome,
		input.Price,
	}}, opts...)
}

// ChangeBid atomically modifies buy order price and amount
// Maps to: change_bid($query_id, $outcome, $old_price, $new_price, $new_amount)
// Migration: 035-order-book-change-order.sql:1705-1876
//
// Key Features:
// - ATOMIC: Either both cancel+place succeed, or neither happens
// - TIMESTAMP PRESERVATION: New order inherits old order's last_updated (maintains FIFO queue position)
// - NET COLLATERAL: Only locks/unlocks the difference
// - FLEXIBLE AMOUNT: Can increase or decrease order size
//
// Example:
//
//	Change buy order from 100 @ $0.54 to 150 @ $0.50
//	OldPrice=-54, NewPrice=-50, NewAmount=150
//	Net collateral: (150×50 - 100×54) × 10^16 = 2100 × 10^16 = 21 TRUF additional
func (o *OrderBook) ChangeBid(ctx context.Context, input types.ChangeBidInput,
	opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	if err := input.Validate(); err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return o.execute(ctx, "change_bid", [][]any{{
		input.QueryID,
		input.Outcome,
		input.OldPrice,
		input.NewPrice,
		input.NewAmount,
	}}, opts...)
}

// ChangeAsk atomically modifies sell order price and amount
// Maps to: change_ask($query_id, $outcome, $old_price, $new_price, $new_amount)
// Migration: 035-order-book-change-order.sql:1937-2124
//
// Key Features:
// - ATOMIC: Either both cancel+place succeed, or neither happens
// - TIMESTAMP PRESERVATION: New order inherits old order's last_updated
// - FLEXIBLE AMOUNT: Can increase (pull from holdings) or decrease (return to holdings)
// - NO COLLATERAL: Sell orders just move shares between holdings and order book
//
// Amount Adjustment:
// - Increase (new_amount > old_amount): Pulls additional shares from holdings
// - Decrease (new_amount < old_amount): Returns excess shares to holdings
// - Same (new_amount = old_amount): Just moves order to new price
//
// Example:
//
//	Change sell order from 100 @ $0.60 to 150 @ $0.55
//	OldPrice=60, NewPrice=55, NewAmount=150
//	Pulls 50 additional shares from holdings
func (o *OrderBook) ChangeAsk(ctx context.Context, input types.ChangeAskInput,
	opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	if err := input.Validate(); err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return o.execute(ctx, "change_ask", [][]any{{
		input.QueryID,
		input.Outcome,
		input.OldPrice,
		input.NewPrice,
		input.NewAmount,
	}}, opts...)
}
