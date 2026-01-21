package contractsapi

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	kwilClientType "github.com/kwilteam/kwil-db/core/types/client"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ═══════════════════════════════════════════════════════════════
// SETTLEMENT & REWARDS OPERATIONS
// ═══════════════════════════════════════════════════════════════

// SettleMarket settles a market using attestation results
// Maps to: settle_market($query_id)
// Migration: 032-order-book-actions.sql:2162-2319
//
// Prerequisites:
// - settle_time reached
// - Signed attestation exists for market hash
// - Market not already settled
//
// Settlement Process:
// 1. Validates market collateral integrity
// 2. Retrieves signed attestation by market hash
// 3. Parses outcome from attestation result
// 4. Updates market: settled=true, winning_outcome, settled_at
// 5. Calls process_settlement() to distribute funds and fees
func (o *OrderBook) SettleMarket(ctx context.Context, input types.SettleMarketInput,
	opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	if err := input.Validate(); err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return o.execute(ctx, "settle_market", [][]any{{
		input.QueryID,
	}}, opts...)
}

// SampleLPRewards samples liquidity provider rewards for a block
// Maps to: sample_lp_rewards($query_id, $block)
// Migration: 034-order-book-rewards.sql:313-550
//
// Behavior:
// - Identifies LPs with paired buy orders (YES @ p + NO @ 100-p) within dynamic spread
// - Calculates scores using Polymarket formula: amount × ((spread - min_dist) / spread)²
// - Normalizes to percentages (sum = 100%)
// - Stores in ob_rewards table
//
// Dynamic Spread Rules:
//
//	Midpoint Distance | Spread
//	0-29¢            | 5¢
//	30-59¢           | 4¢
//	60-79¢           | 3¢
//	80-99¢           | INELIGIBLE
//
// Recommended: Sample every 50 blocks (~10 minutes) for fair distribution
func (o *OrderBook) SampleLPRewards(ctx context.Context, input types.SampleLPRewardsInput,
	opts ...kwilClientType.TxOpt) (kwiltypes.Hash, error) {
	if err := input.Validate(); err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return o.execute(ctx, "sample_lp_rewards", [][]any{{
		input.QueryID,
		input.Block,
	}}, opts...)
}

// ═══════════════════════════════════════════════════════════════
// AUDIT & HISTORY OPERATIONS
// ═══════════════════════════════════════════════════════════════

// GetDistributionSummary retrieves fee distribution summary for a market
// Maps to: get_distribution_summary($query_id)
// Migration: 036-order-book-audit.sql:107-147
//
// Returns: Summary of fee distribution for a settled market
// Returns nil if market not settled or no distribution yet
func (o *OrderBook) GetDistributionSummary(ctx context.Context, input types.GetDistributionSummaryInput) (*types.DistributionSummary, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.QueryID}
	result, err := o.call(ctx, "get_distribution_summary", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(result.Values) == 0 {
		return nil, fmt.Errorf("no distribution found for query_id=%d (market may not be settled yet)", input.QueryID)
	}

	row := result.Values[0]
	summary, err := parseDistributionSummaryRow(row)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return summary, nil
}

// GetDistributionDetails retrieves per-LP reward details
// Maps to: get_distribution_details($distribution_id)
// Migration: 036-order-book-audit.sql:158-197
//
// Returns: Per-LP breakdown of rewards for a distribution
func (o *OrderBook) GetDistributionDetails(ctx context.Context, input types.GetDistributionDetailsInput) ([]types.LPRewardDetail, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.DistributionID}
	result, err := o.call(ctx, "get_distribution_details", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var details []types.LPRewardDetail
	for _, row := range result.Values {
		detail, err := parseLPRewardDetailRow(row)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		details = append(details, detail)
	}

	return details, nil
}

// GetParticipantRewardHistory retrieves reward history for a wallet
// Maps to: get_participant_reward_history($wallet_hex)
// Migration: 036-order-book-audit.sql:208-251
//
// Returns: All rewards received by a participant across all markets
func (o *OrderBook) GetParticipantRewardHistory(ctx context.Context, input types.GetParticipantRewardHistoryInput) ([]types.RewardHistory, error) {
	if err := input.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	args := []any{input.WalletHex}
	result, err := o.call(ctx, "get_participant_reward_history", args)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var history []types.RewardHistory
	for _, row := range result.Values {
		entry, err := parseRewardHistoryRow(row)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		history = append(history, entry)
	}

	return history, nil
}

// ═══════════════════════════════════════════════════════════════
// PARSING HELPERS
// ═══════════════════════════════════════════════════════════════

// parseDistributionSummaryRow parses a row from get_distribution_summary
// Row format: distribution_id, total_fees_distributed, total_lp_count, block_count, distributed_at
func parseDistributionSummaryRow(row []any) (*types.DistributionSummary, error) {
	if len(row) < 5 {
		return nil, fmt.Errorf("invalid row: expected 5 columns, got %d", len(row))
	}

	summary := &types.DistributionSummary{}

	// Column 0: distribution_id (INT)
	if err := extractIntColumn(row[0], &summary.DistributionID, 0, "distribution_id"); err != nil {
		return nil, err
	}

	// Column 1: total_fees_distributed (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[1], &summary.TotalFeesDistributed, 1, "total_fees_distributed"); err != nil {
		return nil, err
	}

	// Column 2: total_lp_count (INT8)
	if err := extractInt64Column(row[2], &summary.TotalLPCount, 2, "total_lp_count"); err != nil {
		return nil, err
	}

	// Column 3: block_count (INT8)
	if err := extractInt64Column(row[3], &summary.BlockCount, 3, "block_count"); err != nil {
		return nil, err
	}

	// Column 4: distributed_at (INT8)
	if err := extractInt64Column(row[4], &summary.DistributedAt, 4, "distributed_at"); err != nil {
		return nil, err
	}

	return summary, nil
}

// parseLPRewardDetailRow parses a row from get_distribution_details
// Row format: wallet_address, reward_amount, total_reward_percent
func parseLPRewardDetailRow(row []any) (types.LPRewardDetail, error) {
	if len(row) < 3 {
		return types.LPRewardDetail{}, fmt.Errorf("invalid row: expected 3 columns, got %d", len(row))
	}

	detail := types.LPRewardDetail{}

	// Column 0: wallet_address (BYTEA)
	if err := extractBytesColumn(row[0], &detail.WalletAddress, 0, "wallet_address"); err != nil {
		return detail, err
	}

	// Column 1: reward_amount (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[1], &detail.RewardAmount, 1, "reward_amount"); err != nil {
		return detail, err
	}

	// Column 2: total_reward_percent (NUMERIC(10,2) as string)
	if err := extractStringColumn(row[2], &detail.TotalRewardPercent, 2, "total_reward_percent"); err != nil {
		return detail, err
	}

	return detail, nil
}

// parseRewardHistoryRow parses a row from get_participant_reward_history
// Row format: distribution_id, query_id, reward_amount, total_reward_percent, distributed_at
func parseRewardHistoryRow(row []any) (types.RewardHistory, error) {
	if len(row) < 5 {
		return types.RewardHistory{}, fmt.Errorf("invalid row: expected 5 columns, got %d", len(row))
	}

	history := types.RewardHistory{}

	// Column 0: distribution_id (INT)
	if err := extractIntColumn(row[0], &history.DistributionID, 0, "distribution_id"); err != nil {
		return history, err
	}

	// Column 1: query_id (INT)
	if err := extractIntColumn(row[1], &history.QueryID, 1, "query_id"); err != nil {
		return history, err
	}

	// Column 2: reward_amount (NUMERIC(78,0) as string)
	if err := extractStringColumn(row[2], &history.RewardAmount, 2, "reward_amount"); err != nil {
		return history, err
	}

	// Column 3: total_reward_percent (NUMERIC(10,2) as string)
	if err := extractStringColumn(row[3], &history.TotalRewardPercent, 3, "total_reward_percent"); err != nil {
		return history, err
	}

	// Column 4: distributed_at (INT8)
	if err := extractInt64Column(row[4], &history.DistributedAt, 4, "distributed_at"); err != nil {
		return history, err
	}

	return history, nil
}
