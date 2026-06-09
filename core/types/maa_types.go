package types

// Modular Agent Address (MAA / "agent wallet") result types.
//
// An MAA lets a token holder (the unrestricted owner/funder) delegate a limited set of actions to an
// agent key (the restricted creator) that can operate the wallet but provably cannot move funds out.
// A creator registers a rule once (immutable); any funder joins that rule to obtain a distinct,
// deterministically-derived wallet address. These structs mirror the on-chain getter columns (migration
// 048). Addresses and hashes are 0x-prefixed lowercase hex as returned by the node.

// MAACreateRuleInput holds the parameters for CreateAgentRule (on-chain maa_create_rule).
type MAACreateRuleInput struct {
	Salt       []byte   // rule nonce; may be nil. Lets one creator register several distinct rules.
	FeeMode    string   // "bps" or "flat".
	FeeBps     int      // 0..10000 (10000 = 100%); used when FeeMode == "bps".
	FeeFlat    string   // base-unit decimal string; used when FeeMode == "flat". Empty is treated as "0".
	Namespaces []string // allow-list: parallel arrays with Actions and BodyHashes.
	Actions    []string
	BodyHashes [][]byte // optional per-entry body-hash pins; nil (or a nil element) = unpinned.
}

// MAARule is a rule's terms (maa_get_rule).
type MAARule struct {
	RuleID         string // 32-byte content-hash identifier
	RestrictedAddr string // the agent / rule creator
	RulesHash      string // commitment over fee + allow-list
	FeeMode        string // "bps" | "flat"
	FeeBps         int64
	FeeFlat        string // base-unit decimal string
	CreatedAt      int64
}

// MAAAllowedAction is one allow-list entry (maa_get_allowed_actions).
type MAAAllowedAction struct {
	Namespace string
	Action    string
	BodyHash  string // "" when unpinned
}

// MAAInstance is an agent wallet and its two component keys (maa_get_instance) — the primary
// explorer/wallet lookup: MAA address -> {rule, restricted, unrestricted}.
type MAAInstance struct {
	MAAAddress       string // 20-byte ETH address that holds funds
	RuleID           string
	RestrictedAddr   string // the agent
	UnrestrictedAddr string // the owner / funder
	CreatedAt        int64
}

// MAARuleRef references a rule created by an agent (maa_list_by_restricted).
type MAARuleRef struct {
	RuleID    string
	CreatedAt int64
}

// MAAOwnedWallet references a wallet an owner funded (maa_list_by_unrestricted).
type MAAOwnedWallet struct {
	MAAAddress string
	RuleID     string
	CreatedAt  int64
}

// MAARuleWallet references a wallet under a rule (maa_list_instances_by_rule).
type MAARuleWallet struct {
	MAAAddress       string
	UnrestrictedAddr string
	CreatedAt        int64
}

// MAAEvent is one append-only audit row (maa_get_events).
type MAAEvent struct {
	ID             int64
	MAAAddress     string // "" for rule-level events (e.g. CREATE_RULE)
	EventType      string // CREATE_RULE | JOIN | ... (FUND/EXEC/WITHDRAW added by later issues)
	ActorRole      string // restricted | unrestricted
	ActorAddr      string
	InnerNamespace string // "" until exec events
	InnerAction    string
	Amount         string // "" unless populated by fee/withdraw events
	TxHash         string
	BlockHeight    int64
	BlockTimestamp int64
}
