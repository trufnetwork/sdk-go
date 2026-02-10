package types

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	kwilClientType "github.com/trufnetwork/kwil-db/core/client/types"
)

// ═══════════════════════════════════════════════════════════════
// INTERFACES
// ═══════════════════════════════════════════════════════════════

// IOrderBook provides methods for interacting with the prediction market order book
type IOrderBook interface {
	// ═══════════════════════════════════════════════════════════════
	// MARKET OPERATIONS
	// ═══════════════════════════════════════════════════════════════

	// CreateMarket creates a new prediction market
	// Maps to: create_market($bridge, $query_components, $settle_time, $max_spread, $min_order_size)
	// Migration: 032-order-book-actions.sql:85-226
	CreateMarket(ctx context.Context, input CreateMarketInput,
		opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error)

	// GetMarketInfo retrieves market details by ID
	// Maps to: get_market_info($query_id)
	// Migration: 032-order-book-actions.sql:157-185
	GetMarketInfo(ctx context.Context, input GetMarketInfoInput) (*MarketInfo, error)

	// GetMarketByHash retrieves market details by query hash
	// Maps to: get_market_by_hash($query_hash)
	// Migration: 032-order-book-actions.sql:199-227
	GetMarketByHash(ctx context.Context, input GetMarketByHashInput) (*MarketInfo, error)

	// ListMarkets returns paginated list of markets with optional filtering
	// Maps to: list_markets($settled_filter, $limit_val, $offset_val)
	// Migration: 032-order-book-actions.sql:242-284
	ListMarkets(ctx context.Context, input ListMarketsInput) ([]MarketSummary, error)

	// MarketExists checks if market exists by hash (lightweight)
	// Maps to: market_exists($query_hash)
	// Migration: 032-order-book-actions.sql:296-307
	MarketExists(ctx context.Context, input MarketExistsInput) (bool, error)

	// ValidateMarketCollateral checks binary token parity and vault balance
	// Maps to: validate_market_collateral($query_id)
	// Migration: 037-order-book-validation.sql:24-119
	ValidateMarketCollateral(ctx context.Context, input ValidateMarketCollateralInput) (*MarketValidation, error)

	// ═══════════════════════════════════════════════════════════════
	// ORDER PLACEMENT
	// ═══════════════════════════════════════════════════════════════

	// PlaceBuyOrder places a buy order for YES or NO shares
	// Maps to: place_buy_order($query_id, $outcome, $price, $amount)
	// Migration: 032-order-book-actions.sql:920-1070
	PlaceBuyOrder(ctx context.Context, input PlaceBuyOrderInput,
		opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error)

	// PlaceSellOrder places a sell order for shares you own
	// Maps to: place_sell_order($query_id, $outcome, $price, $amount)
	// Migration: 032-order-book-actions.sql:1099-1244
	PlaceSellOrder(ctx context.Context, input PlaceSellOrderInput,
		opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error)

	// PlaceSplitLimitOrder mints binary pairs and lists unwanted side for sale
	// Maps to: place_split_limit_order($query_id, $true_price, $amount)
	// Migration: 032-order-book-actions.sql:1295-1459
	PlaceSplitLimitOrder(ctx context.Context, input PlaceSplitLimitOrderInput,
		opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error)

	// CancelOrder cancels an open buy or sell order
	// Maps to: cancel_order($query_id, $outcome, $price)
	// Migration: 032-order-book-actions.sql:1506-1646
	CancelOrder(ctx context.Context, input CancelOrderInput,
		opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error)

	// ChangeBid atomically modifies buy order price and amount
	// Maps to: change_bid($query_id, $outcome, $old_price, $new_price, $new_amount)
	// Migration: 035-order-book-change-order.sql:1705-1876
	ChangeBid(ctx context.Context, input ChangeBidInput,
		opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error)

	// ChangeAsk atomically modifies sell order price and amount
	// Maps to: change_ask($query_id, $outcome, $old_price, $new_price, $new_amount)
	// Migration: 035-order-book-change-order.sql:1937-2124
	ChangeAsk(ctx context.Context, input ChangeAskInput,
		opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error)

	// ═══════════════════════════════════════════════════════════════
	// QUERY OPERATIONS
	// ═══════════════════════════════════════════════════════════════

	// GetOrderBook retrieves all buy/sell orders for a market outcome
	// Maps to: get_order_book($query_id, $outcome)
	// Migration: 038-order-book-queries.sql:18-73
	GetOrderBook(ctx context.Context, input GetOrderBookInput) ([]OrderBookEntry, error)

	// GetUserPositions retrieves caller's portfolio across all markets
	// Maps to: get_user_positions()
	// Migration: 038-order-book-queries.sql:84-138
	GetUserPositions(ctx context.Context) ([]UserPosition, error)

	// GetMarketDepth returns aggregated volume per price level
	// Maps to: get_market_depth($query_id, $outcome)
	// Migration: 038-order-book-queries.sql:149-208
	GetMarketDepth(ctx context.Context, input GetMarketDepthInput) ([]DepthLevel, error)

	// GetBestPrices returns current bid/ask spread
	// Maps to: get_best_prices($query_id, $outcome)
	// Migration: 038-order-book-queries.sql:219-268
	GetBestPrices(ctx context.Context, input GetBestPricesInput) (*BestPrices, error)

	// GetUserCollateral returns caller's total locked collateral value
	// Maps to: get_user_collateral()
	// Migration: 038-order-book-queries.sql:279-359
	GetUserCollateral(ctx context.Context) (*UserCollateral, error)

	// ═══════════════════════════════════════════════════════════════
	// SETTLEMENT & REWARDS
	// ═══════════════════════════════════════════════════════════════

	// SettleMarket settles a market using attestation results
	// Maps to: settle_market($query_id)
	// Migration: 032-order-book-actions.sql:2162-2319
	SettleMarket(ctx context.Context, input SettleMarketInput,
		opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error)

	// SampleLPRewards samples liquidity provider rewards for a block
	// Maps to: sample_lp_rewards($query_id, $block)
	// Migration: 034-order-book-rewards.sql:313-550
	SampleLPRewards(ctx context.Context, input SampleLPRewardsInput,
		opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error)

	// ═══════════════════════════════════════════════════════════════
	// AUDIT & HISTORY
	// ═══════════════════════════════════════════════════════════════

	// GetDistributionSummary retrieves fee distribution summary for a market
	// Maps to: get_distribution_summary($query_id)
	// Migration: 036-order-book-audit.sql:107-147
	GetDistributionSummary(ctx context.Context, input GetDistributionSummaryInput) (*DistributionSummary, error)

	// GetDistributionDetails retrieves per-LP reward details
	// Maps to: get_distribution_details($distribution_id)
	// Migration: 036-order-book-audit.sql:158-197
	GetDistributionDetails(ctx context.Context, input GetDistributionDetailsInput) ([]LPRewardDetail, error)

	// GetParticipantRewardHistory retrieves reward history for a wallet
	// Maps to: get_participant_reward_history($wallet_hex)
	// Migration: 036-order-book-audit.sql:208-251
	GetParticipantRewardHistory(ctx context.Context, input GetParticipantRewardHistoryInput) ([]RewardHistory, error)
}

// ═══════════════════════════════════════════════════════════════
// INPUT TYPES - MARKET OPERATIONS
// ═══════════════════════════════════════════════════════════════

// CreateMarketInput contains parameters for creating a new market
type CreateMarketInput struct {
	Bridge          string // Bridge namespace for collateral (hoodi_tt2, sepolia_bridge, ethereum_bridge)
	QueryComponents []byte // ABI-encoded tuple: (address data_provider, bytes32 stream_id, string action_id, bytes args)
	SettleTime      int64  // Unix timestamp when market can be settled (must be future)
	MaxSpread       int    // Maximum spread for LP rewards (1-50 cents)
	MinOrderSize    int64  // Minimum order size for LP rewards (must be positive)
}

// ValidBridges defines the supported bridge namespaces
var ValidBridges = map[string]bool{
	"hoodi_tt2":       true,
	"sepolia_bridge":  true,
	"ethereum_bridge": true,
}

// Validate checks if CreateMarketInput is valid
func (c *CreateMarketInput) Validate() error {
	// Validate bridge
	if c.Bridge == "" {
		return fmt.Errorf("bridge is required")
	}
	if !ValidBridges[c.Bridge] {
		return fmt.Errorf("bridge must be one of: hoodi_tt2, sepolia_bridge, ethereum_bridge, got %s", c.Bridge)
	}

	// Validate query components
	if len(c.QueryComponents) == 0 {
		return fmt.Errorf("query_components is required")
	}
	// ABI-encoded tuple minimum size: address(32) + bytes32(32) + string offset(32) + bytes offset(32) = 128 bytes
	if len(c.QueryComponents) < 128 {
		return fmt.Errorf("query_components too short for ABI-encoded tuple, got %d bytes (minimum 128)", len(c.QueryComponents))
	}

	// Validate settle time
	if c.SettleTime <= 0 {
		return fmt.Errorf("settle_time must be positive (unix timestamp)")
	}
	// Ensure settle_time is in the future
	if c.SettleTime <= time.Now().Unix() {
		return fmt.Errorf("settle_time must be a future unix timestamp, got %d (current time: %d)", c.SettleTime, time.Now().Unix())
	}

	// Validate max spread (1-50 cents)
	if c.MaxSpread < 1 || c.MaxSpread > 50 {
		return fmt.Errorf("max_spread must be between 1 and 50 cents, got %d", c.MaxSpread)
	}

	// Validate min order size
	if c.MinOrderSize <= 0 {
		return fmt.Errorf("min_order_size must be positive, got %d", c.MinOrderSize)
	}
	return nil
}

// GetMarketInfoInput contains parameters for getting market info by ID
type GetMarketInfoInput struct {
	QueryID int // Market ID from ob_queries.id
}

// Validate checks if GetMarketInfoInput is valid
func (g *GetMarketInfoInput) Validate() error {
	if g.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", g.QueryID)
	}
	return nil
}

// GetMarketByHashInput contains parameters for getting market info by hash
type GetMarketByHashInput struct {
	QueryHash []byte // 32-byte SHA256 hash
}

// Validate checks if GetMarketByHashInput is valid
func (g *GetMarketByHashInput) Validate() error {
	if len(g.QueryHash) != 32 {
		return fmt.Errorf("query_hash must be exactly 32 bytes, got %d", len(g.QueryHash))
	}
	return nil
}

// ListMarketsInput contains parameters for listing markets
type ListMarketsInput struct {
	SettledFilter *bool // nil=all, true=settled only, false=active only
	Limit         *int  // Max results (default 100, max 100)
	Offset        *int  // Skip N results (default 0)
}

// Validate checks if ListMarketsInput is valid
func (l *ListMarketsInput) Validate() error {
	if l.Limit != nil && (*l.Limit < 1 || *l.Limit > 100) {
		return fmt.Errorf("limit must be between 1 and 100, got %d", *l.Limit)
	}
	if l.Offset != nil && *l.Offset < 0 {
		return fmt.Errorf("offset must be non-negative, got %d", *l.Offset)
	}
	return nil
}

// MarketExistsInput contains parameters for checking market existence
type MarketExistsInput struct {
	QueryHash []byte // 32-byte SHA256 hash to check
}

// Validate checks if MarketExistsInput is valid
func (m *MarketExistsInput) Validate() error {
	if len(m.QueryHash) != 32 {
		return fmt.Errorf("query_hash must be exactly 32 bytes, got %d", len(m.QueryHash))
	}
	return nil
}

// ValidateMarketCollateralInput contains parameters for validating market collateral
type ValidateMarketCollateralInput struct {
	QueryID int // Market ID to validate
}

// Validate checks if ValidateMarketCollateralInput is valid
func (v *ValidateMarketCollateralInput) Validate() error {
	if v.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", v.QueryID)
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════
// INPUT TYPES - ORDER OPERATIONS
// ═══════════════════════════════════════════════════════════════

// PlaceBuyOrderInput contains parameters for placing a buy order
type PlaceBuyOrderInput struct {
	QueryID int   // Market ID from ob_queries.id
	Outcome bool  // TRUE for YES shares, FALSE for NO shares
	Price   int   // Price per share in cents (1-99 = $0.01 to $0.99)
	Amount  int64 // Number of shares to buy
}

// Validate checks if PlaceBuyOrderInput is valid
func (p *PlaceBuyOrderInput) Validate() error {
	if p.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", p.QueryID)
	}
	if p.Price < 1 || p.Price > 99 {
		return fmt.Errorf("price must be between 1 and 99 cents, got %d", p.Price)
	}
	if p.Amount <= 0 {
		return fmt.Errorf("amount must be positive, got %d", p.Amount)
	}
	if p.Amount > 1_000_000_000 {
		return fmt.Errorf("amount exceeds maximum of 1,000,000,000, got %d", p.Amount)
	}
	return nil
}

// PlaceSellOrderInput contains parameters for placing a sell order
type PlaceSellOrderInput struct {
	QueryID int   // Market ID
	Outcome bool  // TRUE for YES, FALSE for NO
	Price   int   // Price in cents (1-99)
	Amount  int64 // Number of shares to sell
}

// Validate checks if PlaceSellOrderInput is valid
func (p *PlaceSellOrderInput) Validate() error {
	if p.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", p.QueryID)
	}
	if p.Price < 1 || p.Price > 99 {
		return fmt.Errorf("price must be between 1 and 99 cents, got %d", p.Price)
	}
	if p.Amount <= 0 {
		return fmt.Errorf("amount must be positive, got %d", p.Amount)
	}
	if p.Amount > 1_000_000_000 {
		return fmt.Errorf("amount exceeds maximum of 1,000,000,000, got %d", p.Amount)
	}
	return nil
}

// PlaceSplitLimitOrderInput contains parameters for placing a split limit order
type PlaceSplitLimitOrderInput struct {
	QueryID   int   // Market ID
	TruePrice int   // YES price in cents (1-99)
	Amount    int64 // Number of share PAIRS to mint
}

// Validate checks if PlaceSplitLimitOrderInput is valid
func (p *PlaceSplitLimitOrderInput) Validate() error {
	if p.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", p.QueryID)
	}
	if p.TruePrice < 1 || p.TruePrice > 99 {
		return fmt.Errorf("true_price must be between 1 and 99 cents, got %d", p.TruePrice)
	}
	if p.Amount <= 0 {
		return fmt.Errorf("amount must be positive, got %d", p.Amount)
	}
	if p.Amount > 1_000_000_000 {
		return fmt.Errorf("amount exceeds maximum of 1,000,000,000, got %d", p.Amount)
	}
	return nil
}

// CancelOrderInput contains parameters for cancelling an order
type CancelOrderInput struct {
	QueryID int  // Market ID
	Outcome bool // Order outcome
	Price   int  // Order price (-99 to 99, excluding 0)
}

// Validate checks if CancelOrderInput is valid
func (c *CancelOrderInput) Validate() error {
	if c.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", c.QueryID)
	}
	if c.Price == 0 {
		return fmt.Errorf("price cannot be 0 (holdings cannot be cancelled)")
	}
	if c.Price < -99 || c.Price > 99 {
		return fmt.Errorf("price must be between -99 and 99 (excluding 0), got %d", c.Price)
	}
	return nil
}

// ChangeBidInput contains parameters for changing a buy order
type ChangeBidInput struct {
	QueryID   int   // Market ID
	Outcome   bool  // Order outcome
	OldPrice  int   // Current buy order price (must be negative: -99 to -1)
	NewPrice  int   // New buy order price (must be negative: -99 to -1)
	NewAmount int64 // New order amount
}

// Validate checks if ChangeBidInput is valid
func (c *ChangeBidInput) Validate() error {
	if c.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", c.QueryID)
	}
	if c.OldPrice >= 0 || c.OldPrice < -99 {
		return fmt.Errorf("old_price must be negative (buy order) between -99 and -1, got %d", c.OldPrice)
	}
	if c.NewPrice >= 0 || c.NewPrice < -99 {
		return fmt.Errorf("new_price must be negative (buy order) between -99 and -1, got %d", c.NewPrice)
	}
	if c.OldPrice == c.NewPrice {
		return fmt.Errorf("new_price must differ from old_price")
	}
	if c.NewAmount <= 0 {
		return fmt.Errorf("new_amount must be positive, got %d", c.NewAmount)
	}
	if c.NewAmount > 1_000_000_000 {
		return fmt.Errorf("new_amount exceeds maximum of 1,000,000,000, got %d", c.NewAmount)
	}
	return nil
}

// ChangeAskInput contains parameters for changing a sell order
type ChangeAskInput struct {
	QueryID   int   // Market ID
	Outcome   bool  // Order outcome
	OldPrice  int   // Current sell order price (must be positive: 1 to 99)
	NewPrice  int   // New sell order price (must be positive: 1 to 99)
	NewAmount int64 // New order amount
}

// Validate checks if ChangeAskInput is valid
func (c *ChangeAskInput) Validate() error {
	if c.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", c.QueryID)
	}
	if c.OldPrice <= 0 || c.OldPrice > 99 {
		return fmt.Errorf("old_price must be positive (sell order) between 1 and 99, got %d", c.OldPrice)
	}
	if c.NewPrice <= 0 || c.NewPrice > 99 {
		return fmt.Errorf("new_price must be positive (sell order) between 1 and 99, got %d", c.NewPrice)
	}
	if c.OldPrice == c.NewPrice {
		return fmt.Errorf("new_price must differ from old_price")
	}
	if c.NewAmount <= 0 {
		return fmt.Errorf("new_amount must be positive, got %d", c.NewAmount)
	}
	if c.NewAmount > 1_000_000_000 {
		return fmt.Errorf("new_amount exceeds maximum of 1,000,000,000, got %d", c.NewAmount)
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════
// INPUT TYPES - QUERY OPERATIONS
// ═══════════════════════════════════════════════════════════════

// GetOrderBookInput contains parameters for getting order book
type GetOrderBookInput struct {
	QueryID int  // Market ID
	Outcome bool // TRUE for YES order book, FALSE for NO order book
}

// Validate checks if GetOrderBookInput is valid
func (g *GetOrderBookInput) Validate() error {
	if g.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", g.QueryID)
	}
	return nil
}

// GetMarketDepthInput contains parameters for getting market depth
type GetMarketDepthInput struct {
	QueryID int  // Market ID
	Outcome bool // Order book side
}

// Validate checks if GetMarketDepthInput is valid
func (g *GetMarketDepthInput) Validate() error {
	if g.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", g.QueryID)
	}
	return nil
}

// GetBestPricesInput contains parameters for getting best prices
type GetBestPricesInput struct {
	QueryID int  // Market ID
	Outcome bool // Order book side
}

// Validate checks if GetBestPricesInput is valid
func (g *GetBestPricesInput) Validate() error {
	if g.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", g.QueryID)
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════
// INPUT TYPES - SETTLEMENT & REWARDS
// ═══════════════════════════════════════════════════════════════

// SettleMarketInput contains parameters for settling a market
type SettleMarketInput struct {
	QueryID int // Market ID to settle
}

// Validate checks if SettleMarketInput is valid
func (s *SettleMarketInput) Validate() error {
	if s.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", s.QueryID)
	}
	return nil
}

// SampleLPRewardsInput contains parameters for sampling LP rewards
type SampleLPRewardsInput struct {
	QueryID int   // Market ID
	Block   int64 // Block height to sample
}

// Validate checks if SampleLPRewardsInput is valid
func (s *SampleLPRewardsInput) Validate() error {
	if s.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", s.QueryID)
	}
	if s.Block < 0 {
		return fmt.Errorf("block must be non-negative, got %d", s.Block)
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════
// INPUT TYPES - AUDIT & HISTORY
// ═══════════════════════════════════════════════════════════════

// GetDistributionSummaryInput contains parameters for getting distribution summary
type GetDistributionSummaryInput struct {
	QueryID int // Market ID
}

// Validate checks if GetDistributionSummaryInput is valid
func (g *GetDistributionSummaryInput) Validate() error {
	if g.QueryID < 1 {
		return fmt.Errorf("query_id must be positive, got %d", g.QueryID)
	}
	return nil
}

// GetDistributionDetailsInput contains parameters for getting distribution details
type GetDistributionDetailsInput struct {
	DistributionID int // Distribution ID from get_distribution_summary
}

// Validate checks if GetDistributionDetailsInput is valid
func (g *GetDistributionDetailsInput) Validate() error {
	if g.DistributionID < 1 {
		return fmt.Errorf("distribution_id must be positive, got %d", g.DistributionID)
	}
	return nil
}

// GetParticipantRewardHistoryInput contains parameters for getting participant reward history
type GetParticipantRewardHistoryInput struct {
	WalletHex string // Ethereum address as 0x-prefixed hex string
}

// Validate checks if GetParticipantRewardHistoryInput is valid
func (g *GetParticipantRewardHistoryInput) Validate() error {
	if g.WalletHex == "" {
		return fmt.Errorf("wallet_hex is required")
	}
	if len(g.WalletHex) != 42 || g.WalletHex[:2] != "0x" {
		return fmt.Errorf("wallet_hex must be 0x-prefixed 40-character hex string, got %s", g.WalletHex)
	}
	// Validate hex characters after "0x" prefix
	_, err := hex.DecodeString(g.WalletHex[2:])
	if err != nil {
		return fmt.Errorf("wallet_hex contains invalid hex characters: %w", err)
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════
// OUTPUT TYPES - MARKET OPERATIONS
// ═══════════════════════════════════════════════════════════════

// MarketInfo contains detailed information about a market
type MarketInfo struct {
	ID              int     // Query ID (returned by get_market_by_hash)
	Hash            []byte  // 32-byte query hash (BYTEA)
	QueryComponents []byte  // ABI-encoded query components (BYTEA) - only from get_market_info
	Bridge          string  // Bridge namespace (TEXT) - only from get_market_info
	SettleTime      int64   // Unix timestamp (INT8)
	Settled         bool    // Settlement status (BOOL)
	WinningOutcome  *bool   // TRUE=YES won, FALSE=NO won, nil=not settled (BOOL nullable)
	SettledAt       *int64  // Settlement timestamp, nil if not settled (INT8 nullable)
	MaxSpread       int     // LP reward spread (1-50 cents) (INT)
	MinOrderSize    int64   // LP reward minimum (INT8)
	CreatedAt       int64   // Creation block height (INT8)
	Creator         []byte  // Creator's Ethereum address (BYTEA)
}

// MarketSummary contains summary information about a market
type MarketSummary struct {
	ID             int     // Query ID
	Hash           []byte  // 32-byte query hash
	SettleTime     int64   // Unix timestamp
	Settled        bool    // Settlement status
	WinningOutcome *bool   // Winner if settled
	MaxSpread      int     // LP reward spread
	MinOrderSize   int64   // LP reward minimum
	CreatedAt      int64   // Creation block
}

// MarketValidation contains market collateral validation results
type MarketValidation struct {
	ValidTokenBinaries bool   // TRUE if total_true = total_false
	ValidCollateral    bool   // TRUE if vault balance matches expected
	TotalTrue          int64  // Total TRUE shares (holdings + sell orders)
	TotalFalse         int64  // Total FALSE shares (holdings + sell orders)
	VaultBalance       string // Current vault balance (NUMERIC(78,0) as string)
	ExpectedCollateral string // Expected collateral (NUMERIC(78,0) as string)
	OpenBuysValue      int64  // Sum of buy order collateral in cents
}

// ═══════════════════════════════════════════════════════════════
// OUTPUT TYPES - QUERY OPERATIONS
// ═══════════════════════════════════════════════════════════════

// OrderBookEntry represents a single order in the order book
type OrderBookEntry struct {
	ParticipantID int    // Participant ID (INT)
	Price         int    // Order price (negative=buy, positive=sell) (INT)
	Amount        int64  // Share amount (INT8)
	LastUpdated   int64  // Unix timestamp for FIFO ordering (INT8)
	WalletAddress []byte // Participant's Ethereum address (BYTEA/TEXT)
}

// UserPosition represents a user's position in a market
type UserPosition struct {
	QueryID      int    // Market ID (INT)
	Outcome      bool   // TRUE=YES, FALSE=NO (BOOL)
	Price        int    // 0=holding, <0=buy, >0=sell (INT)
	Amount       int64  // Share amount (INT8)
	PositionType string // 'holding', 'buy_order', 'sell_order' (TEXT)
}

// DepthLevel represents aggregated volume at a price level
type DepthLevel struct {
	Price       int   // Price level (INT)
	TotalAmount int64 // Aggregated amount at this price (INT8)
}

// BestPrices contains the current bid/ask spread
type BestPrices struct {
	BestBid *int // Highest buy price, nil if no bids (INT nullable)
	BestAsk *int // Lowest sell price, nil if no asks (INT nullable)
	Spread  *int // BestAsk - BestBid, nil if either side empty (INT nullable)
}

// UserCollateral contains user's total locked collateral
type UserCollateral struct {
	TotalLocked     string // NUMERIC(78,0) as string - total locked collateral in wei
	BuyOrdersLocked string // NUMERIC(78,0) as string - collateral locked in buy orders
	SharesValue     string // NUMERIC(78,0) as string - value of shares at $1.00 per share
}

// ═══════════════════════════════════════════════════════════════
// OUTPUT TYPES - SETTLEMENT & AUDIT
// ═══════════════════════════════════════════════════════════════

// DistributionSummary contains fee distribution summary for a market
type DistributionSummary struct {
	DistributionID       int    // Unique distribution ID (INT)
	TotalFeesDistributed string // NUMERIC(78,0) as string - total fees in wei
	TotalLPCount         int64  // Number of LPs who received rewards (INT8)
	BlockCount           int64  // Number of blocks sampled (INT8)
	DistributedAt        int64  // Distribution timestamp (INT8)
}

// LPRewardDetail contains per-LP reward details
type LPRewardDetail struct {
	WalletAddress      []byte // LP's Ethereum address (BYTEA)
	RewardAmount       string // NUMERIC(78,0) as string - reward in wei
	TotalRewardPercent string // NUMERIC(10,2) as string - sum of percentages across blocks
}

// RewardHistory contains reward history for a participant
type RewardHistory struct {
	DistributionID     int    // Distribution ID (INT)
	QueryID            int    // Market ID (INT)
	RewardAmount       string // NUMERIC(78,0) as string
	TotalRewardPercent string // NUMERIC(10,2) as string
	DistributedAt      int64  // Timestamp (INT8)
}
