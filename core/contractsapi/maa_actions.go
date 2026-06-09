package contractsapi

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cockroachdb/apd/v3"
	"github.com/pkg/errors"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// Modular Agent Address (MAA / "agent wallet") actions.
//
// These wrap the on-chain rule store (migration 048). The two write actions (CreateAgentRule,
// JoinAgentAddress) derive their result address OFF-CHAIN with the shared keccak derivation
// (core/util/maa_address.go), so the caller learns the rule_id / wallet address immediately — before
// the wallet is funded — rather than waiting on the transaction. The getters expose the public
// transparency surface (rule terms, allow-list, the wallet's two component keys, audit log).

// CreateAgentRule registers an agent-wallet rule. The caller (signer) becomes the restricted agent.
// It returns the locally-derived rule_id (the handle a funder passes to JoinAgentAddress) and the
// submission transaction hash. The rule is immutable once created. Funding never happens here.
func (s *Action) CreateAgentRule(ctx context.Context, input types.MAACreateRuleInput) (ruleID []byte, txHash string, err error) {
	if input.FeeMode != "bps" && input.FeeMode != "flat" {
		return nil, "", errors.New("fee_mode must be 'bps' or 'flat'")
	}
	if input.FeeBps < 0 || input.FeeBps > 10000 {
		return nil, "", errors.Errorf("fee_bps must be between 0 and 10000 (10000 = 100%%), got %d", input.FeeBps)
	}
	feeFlat := input.FeeFlat
	if feeFlat == "" {
		feeFlat = "0"
	}
	if _, _, derr := apd.NewFromString(feeFlat); derr != nil {
		return nil, "", errors.Wrapf(derr, "invalid fee_flat %q", feeFlat)
	}

	// Derive the rule_id locally from the same inputs the chain hashes (caller = restricted agent).
	restricted, err := s.callerAddressBytes()
	if err != nil {
		return nil, "", err
	}
	rulesHash, err := util.ComputeRulesHash(input.FeeMode, int64(input.FeeBps), feeFlat,
		input.Namespaces, input.Actions, input.BodyHashes)
	if err != nil {
		return nil, "", err
	}
	ruleID, err = util.DeriveRuleID(restricted, rulesHash, input.Salt)
	if err != nil {
		return nil, "", err
	}

	// Args must match maa_create_rule($salt, $fee_mode, $fee_bps, $fee_flat, $namespaces, $actions, $body_hashes).
	args := []any{input.Salt, input.FeeMode, input.FeeBps, feeFlat, input.Namespaces, input.Actions, input.BodyHashes}
	hash, err := s.execute(ctx, "maa_create_rule", [][]any{args})
	if err != nil {
		return nil, "", err
	}
	return ruleID, hash.String(), nil
}

// JoinAgentAddress joins an existing rule as the unrestricted owner/funder. The caller (signer) becomes
// the owner. It returns the locally-derived MAA address (the wallet to fund) and the submission
// transaction hash. The rule's restricted creator is looked up on-chain to derive the address.
func (s *Action) JoinAgentAddress(ctx context.Context, ruleID []byte) (maaAddress []byte, txHash string, err error) {
	if len(ruleID) != 32 {
		return nil, "", errors.Errorf("rule_id must be 32 bytes, got %d", len(ruleID))
	}

	unrestricted, err := s.callerAddressBytes()
	if err != nil {
		return nil, "", err
	}

	// Resolve the rule's restricted creator so the wallet can be derived locally (also validates the rule exists).
	rule, err := s.GetAgentRule(ctx, ruleID)
	if err != nil {
		return nil, "", errors.Wrap(err, "look up rule for join")
	}
	if rule == nil {
		return nil, "", errors.New("unknown rule_id")
	}
	restricted, err := util.NewEthereumAddressFromString(rule.RestrictedAddr)
	if err != nil {
		return nil, "", errors.Wrap(err, "parse restricted address")
	}
	maaAddress, err = util.DeriveMAAAddress(unrestricted, restricted.Bytes(), ruleID)
	if err != nil {
		return nil, "", err
	}

	hash, err := s.execute(ctx, "maa_join", [][]any{{ruleID}})
	if err != nil {
		return nil, "", err
	}
	return maaAddress, hash.String(), nil
}

// GetAgentRule returns a rule's terms (maa_get_rule), or nil if no such rule exists.
func (s *Action) GetAgentRule(ctx context.Context, ruleID []byte) (*types.MAARule, error) {
	res, err := s.call(ctx, "maa_get_rule", []any{ruleID})
	if err != nil {
		return nil, err
	}
	if len(res.Values) == 0 {
		return nil, nil // no such rule
	}
	r := res.Values[0]
	if len(r) < 7 {
		return nil, fmt.Errorf("malformed maa_get_rule row: expected >=7 columns, got %d", len(r))
	}
	return &types.MAARule{
		RuleID:         maaStr(r[0]),
		RestrictedAddr: maaStr(r[1]),
		RulesHash:      maaStr(r[2]),
		FeeMode:        maaStr(r[3]),
		FeeBps:         maaInt64(r[4]),
		FeeFlat:        maaStr(r[5]),
		CreatedAt:      maaInt64(r[6]),
	}, nil
}

// GetAgentRuleAllowedActions returns a rule's allow-list (maa_get_allowed_actions), in canonical order.
func (s *Action) GetAgentRuleAllowedActions(ctx context.Context, ruleID []byte) ([]types.MAAAllowedAction, error) {
	res, err := s.call(ctx, "maa_get_allowed_actions", []any{ruleID})
	if err != nil {
		return nil, err
	}
	out := make([]types.MAAAllowedAction, 0, len(res.Values))
	for _, r := range res.Values {
		if len(r) < 3 {
			return nil, fmt.Errorf("malformed maa_get_allowed_actions row: expected >=3 columns, got %d", len(r))
		}
		out = append(out, types.MAAAllowedAction{
			Namespace: maaStr(r[0]),
			Action:    maaStr(r[1]),
			BodyHash:  maaStr(r[2]),
		})
	}
	return out, nil
}

// GetAgentWallet returns an agent wallet and its two component keys (maa_get_instance), or nil if the
// address is not a known wallet.
func (s *Action) GetAgentWallet(ctx context.Context, maaAddress []byte) (*types.MAAInstance, error) {
	res, err := s.call(ctx, "maa_get_instance", []any{maaAddress})
	if err != nil {
		return nil, err
	}
	if len(res.Values) == 0 {
		return nil, nil // not a known wallet
	}
	r := res.Values[0]
	if len(r) < 5 {
		return nil, fmt.Errorf("malformed maa_get_instance row: expected >=5 columns, got %d", len(r))
	}
	return &types.MAAInstance{
		MAAAddress:       maaStr(r[0]),
		RuleID:           maaStr(r[1]),
		RestrictedAddr:   maaStr(r[2]),
		UnrestrictedAddr: maaStr(r[3]),
		CreatedAt:        maaInt64(r[4]),
	}, nil
}

// ListAgentRulesByRestricted lists the rules an agent created (maa_list_by_restricted). agent is a
// 0x-hex address (with or without prefix).
func (s *Action) ListAgentRulesByRestricted(ctx context.Context, agent string, limit, offset int) ([]types.MAARuleRef, error) {
	res, err := s.call(ctx, "maa_list_by_restricted", []any{agent, limit, offset})
	if err != nil {
		return nil, err
	}
	out := make([]types.MAARuleRef, 0, len(res.Values))
	for _, r := range res.Values {
		if len(r) < 2 {
			return nil, fmt.Errorf("malformed maa_list_by_restricted row: expected >=2 columns, got %d", len(r))
		}
		out = append(out, types.MAARuleRef{RuleID: maaStr(r[0]), CreatedAt: maaInt64(r[1])})
	}
	return out, nil
}

// ListAgentWalletsByOwner lists the wallets an owner funded (maa_list_by_unrestricted). owner is a
// 0x-hex address (with or without prefix).
func (s *Action) ListAgentWalletsByOwner(ctx context.Context, owner string, limit, offset int) ([]types.MAAOwnedWallet, error) {
	res, err := s.call(ctx, "maa_list_by_unrestricted", []any{owner, limit, offset})
	if err != nil {
		return nil, err
	}
	out := make([]types.MAAOwnedWallet, 0, len(res.Values))
	for _, r := range res.Values {
		if len(r) < 3 {
			return nil, fmt.Errorf("malformed maa_list_by_unrestricted row: expected >=3 columns, got %d", len(r))
		}
		out = append(out, types.MAAOwnedWallet{
			MAAAddress: maaStr(r[0]),
			RuleID:     maaStr(r[1]),
			CreatedAt:  maaInt64(r[2]),
		})
	}
	return out, nil
}

// ListAgentWalletsByRule lists every wallet funded under a rule (maa_list_instances_by_rule).
func (s *Action) ListAgentWalletsByRule(ctx context.Context, ruleID []byte, limit, offset int) ([]types.MAARuleWallet, error) {
	res, err := s.call(ctx, "maa_list_instances_by_rule", []any{ruleID, limit, offset})
	if err != nil {
		return nil, err
	}
	out := make([]types.MAARuleWallet, 0, len(res.Values))
	for _, r := range res.Values {
		if len(r) < 3 {
			return nil, fmt.Errorf("malformed maa_list_instances_by_rule row: expected >=3 columns, got %d", len(r))
		}
		out = append(out, types.MAARuleWallet{
			MAAAddress:       maaStr(r[0]),
			UnrestrictedAddr: maaStr(r[1]),
			CreatedAt:        maaInt64(r[2]),
		})
	}
	return out, nil
}

// GetAgentRuleEvents returns a rule's append-only audit log (maa_get_events).
func (s *Action) GetAgentRuleEvents(ctx context.Context, ruleID []byte, limit, offset int) ([]types.MAAEvent, error) {
	res, err := s.call(ctx, "maa_get_events", []any{ruleID, limit, offset})
	if err != nil {
		return nil, err
	}
	out := make([]types.MAAEvent, 0, len(res.Values))
	for _, r := range res.Values {
		if len(r) < 11 {
			return nil, fmt.Errorf("malformed maa_get_events row: expected >=11 columns, got %d", len(r))
		}
		out = append(out, types.MAAEvent{
			ID:             maaInt64(r[0]),
			MAAAddress:     maaStr(r[1]),
			EventType:      maaStr(r[2]),
			ActorRole:      maaStr(r[3]),
			ActorAddr:      maaStr(r[4]),
			InnerNamespace: maaStr(r[5]),
			InnerAction:    maaStr(r[6]),
			Amount:         maaStr(r[7]),
			TxHash:         maaStr(r[8]),
			BlockHeight:    maaInt64(r[9]),
			BlockTimestamp: maaInt64(r[10]),
		})
	}
	return out, nil
}

// IsAgentWallet reports whether an address is a known (joined) agent wallet (maa_is_known).
func (s *Action) IsAgentWallet(ctx context.Context, maaAddress []byte) (bool, error) {
	res, err := s.call(ctx, "maa_is_known", []any{maaAddress})
	if err != nil {
		return false, err
	}
	if len(res.Values) == 0 {
		return false, nil // not a known wallet
	}
	if len(res.Values[0]) == 0 {
		return false, fmt.Errorf("malformed maa_is_known row: expected >=1 column, got 0")
	}
	return maaBool(res.Values[0][0]), nil
}

// callerAddressBytes returns the signer's 20-byte Ethereum address (the on-chain @caller for writes).
func (s *Action) callerAddressBytes() ([]byte, error) {
	idStr, err := auth.EthSecp256k1Authenticator{}.Identifier(s._client.Signer().CompactID())
	if err != nil {
		return nil, errors.Wrap(err, "resolve caller address from signer")
	}
	addr, err := util.NewEthereumAddressFromString(idStr)
	if err != nil {
		return nil, errors.Wrap(err, "parse caller address")
	}
	return addr.Bytes(), nil
}

// maaStr coerces a query-result cell to a string. NUMERIC cells arrive as *types.Decimal (Stringer);
// TEXT as string; NULL as nil -> "".
func maaStr(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// maaInt64 coerces an INT/INT8 query-result cell to int64.
func maaInt64(v any) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case int64:
		return t
	case int:
		return int64(t)
	case int32:
		return int64(t)
	default:
		n, _ := strconv.ParseInt(fmt.Sprint(t), 10, 64)
		return n
	}
}

// maaBool coerces a BOOL query-result cell to bool.
func maaBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	default:
		return false
	}
}
