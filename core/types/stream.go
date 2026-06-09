package types

import (
	"context"

	"github.com/cockroachdb/apd/v3"
	kwilType "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/kwil-db/node/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

type GetRecordInput struct {
	DataProvider string
	StreamId     string
	From         *int
	To           *int
	FrozenAt     *int
	BaseDate     *int
	Prefix       *string
	UseCache     *bool
}

type GetIndexInput = GetRecordInput

type GetIndexChangeInput struct {
	DataProvider string
	StreamId     string
	From         *int
	To           *int
	FrozenAt     *int
	BaseDate     *int
	TimeInterval int
	Prefix       *string
	UseCache     *bool
}

type GetFirstRecordInput struct {
	DataProvider string
	StreamId     string
	After        *int
	FrozenAt     *int
	UseCache     *bool
}

type StreamResult struct {
	EventTime int
	Value     apd.Decimal
}

type ReadWalletInput struct {
	Stream StreamLocator
	Wallet util.EthereumAddress
}

type VisibilityInput struct {
	Stream     StreamLocator
	Visibility util.VisibilityEnum
}

type CheckStreamExistsInput struct {
	DataProvider string
	StreamId     string
}

type DefaultBaseTimeInput struct {
	Stream   StreamLocator
	BaseTime int
}

type StreamExistsResult struct {
	StreamLocator StreamLocator
	Exists        bool
}

type IAction interface {
	// ExecuteProcedure Executes an arbitrary procedure on the stream. Execute refers to the write calls
	ExecuteProcedure(ctx context.Context, procedure string, args [][]any) (types.Hash, error)
	// CallProcedure calls an arbitrary procedure on the stream. Call refers to the read calls
	CallProcedure(ctx context.Context, procedure string, args []any) (*kwilType.QueryResult, error)

	// GetRecord reads the records of the stream within the given date range
	GetRecord(ctx context.Context, input GetRecordInput) (ActionResult, error)
	// GetIndex reads the index of the stream within the given date range
	GetIndex(ctx context.Context, input GetIndexInput) (ActionResult, error)
	// GetIndexChange reads the index change of the stream within the given date range
	GetIndexChange(ctx context.Context, input GetIndexChangeInput) (ActionResult, error)
	// GetType gets the type of the stream -- Primitive or Composed
	GetType(ctx context.Context, locator StreamLocator) (StreamType, error)
	// GetFirstRecord gets the first record of the stream
	GetFirstRecord(ctx context.Context, input GetFirstRecordInput) (ActionResult, error)

	// SetReadVisibility sets the read visibility of the stream -- Private or Public
	SetReadVisibility(ctx context.Context, input VisibilityInput) (types.Hash, error)
	// GetReadVisibility gets the read visibility of the stream -- Private or Public
	GetReadVisibility(ctx context.Context, locator StreamLocator) (*util.VisibilityEnum, error)
	// SetComposeVisibility sets the compose visibility of the stream -- Private or Public
	SetComposeVisibility(ctx context.Context, input VisibilityInput) (types.Hash, error)
	// GetComposeVisibility gets the compose visibility of the stream -- Private or Public
	GetComposeVisibility(ctx context.Context, locator StreamLocator) (*util.VisibilityEnum, error)

	// AllowReadWallet allows a wallet to read the stream, if reading is private
	AllowReadWallet(ctx context.Context, input ReadWalletInput) (types.Hash, error)
	// DisableReadWallet disables a wallet from reading the stream
	DisableReadWallet(ctx context.Context, input ReadWalletInput) (types.Hash, error)
	// AllowComposeStream allows a stream to use this stream as child, if composing is private
	AllowComposeStream(ctx context.Context, locator StreamLocator) (types.Hash, error)
	// DisableComposeStream disables a stream from using this stream as child
	DisableComposeStream(ctx context.Context, locator StreamLocator) (types.Hash, error)

	// GetAllowedReadWallets gets the wallets allowed to read the stream
	GetAllowedReadWallets(ctx context.Context, locator StreamLocator) ([]util.EthereumAddress, error)
	// GetAllowedComposeStreams gets the streams allowed to compose this stream
	GetAllowedComposeStreams(ctx context.Context, locator StreamLocator) ([]StreamLocator, error)

	// SetDefaultBaseTime insert a metadata row with `default_base_time` key
	SetDefaultBaseTime(ctx context.Context, input DefaultBaseTimeInput) (types.Hash, error)

	// SetAllowZeros toggles whether value=0 inserts are persisted on the
	// stream. Default behavior is FALSE (zeros are dropped on insert).
	// Owner-gated. The toggle is forward-only — historical state is not
	// rewritten by flipping the flag.
	SetAllowZeros(ctx context.Context, locator StreamLocator, value bool) (types.Hash, error)

	// GetAllowZeros returns the current allow_zeros setting for the stream.
	// Returns false if the stream has no explicit metadata row (the
	// implicit default).
	GetAllowZeros(ctx context.Context, locator StreamLocator) (bool, error)

	// GetStreamOwner gets the owner of the stream
	GetStreamOwner(ctx context.Context, locator StreamLocator) ([]byte, error)

	// BatchStreamExists checks for the existence of multiple streams.
	BatchStreamExists(ctx context.Context, streamsInput []StreamLocator) ([]StreamExistsResult, error)

	// BatchFilterStreamsByExistence filters a list of streams based on their existence in the database.
	// Use this instead of BatchStreamExists if you want less data returned.
	BatchFilterStreamsByExistence(ctx context.Context, streamsInput []StreamLocator, returnExisting bool) ([]StreamLocator, error)

	// GetHistory retrieves the transaction history for a wallet on a specific bridge
	GetHistory(ctx context.Context, input GetHistoryInput) ([]BridgeHistory, error)

	// GetWalletBalance retrieves the wallet balance for a specific bridge instance
	GetWalletBalance(ctx context.Context, bridgeIdentifier string, walletAddress string) (string, error)

	// Withdraw performs a withdrawal operation by bridging tokens from TN to a destination chain
	Withdraw(ctx context.Context, bridgeIdentifier string, amount string, recipient string) (string, error)

	// Transfer sends tokens from the caller to another in-network wallet via the
	// bridge's public transfer action ("<bridgeIdentifier>_transfer"). Costs a
	// 1-token fee on top of `amount`, paid in the same token as the bridge.
	Transfer(ctx context.Context, bridgeIdentifier string, recipient string, amount string) (string, error)

	// GetWithdrawalProof retrieves the proofs and signatures needed to claim a withdrawal on EVM.
	GetWithdrawalProof(ctx context.Context, input GetWithdrawalProofInput) ([]WithdrawalProof, error)

	// --- Modular Agent Addresses (agent wallets) ---

	// CreateAgentRule registers an agent-wallet rule (the caller becomes the restricted agent) and
	// returns the locally-derived rule_id together with the submission transaction hash.
	CreateAgentRule(ctx context.Context, input MAACreateRuleInput) (ruleID []byte, txHash string, err error)

	// JoinAgentAddress joins an existing rule as the unrestricted owner/funder and returns the
	// locally-derived MAA address (the wallet to fund) together with the submission transaction hash.
	JoinAgentAddress(ctx context.Context, ruleID []byte) (maaAddress []byte, txHash string, err error)

	// GetAgentRule returns a rule's terms (fee + commitment), or nil if no such rule exists.
	GetAgentRule(ctx context.Context, ruleID []byte) (*MAARule, error)

	// GetAgentRuleAllowedActions returns a rule's allow-list in canonical order.
	GetAgentRuleAllowedActions(ctx context.Context, ruleID []byte) ([]MAAAllowedAction, error)

	// GetAgentWallet returns an agent wallet and its two component keys, or nil if unknown.
	GetAgentWallet(ctx context.Context, maaAddress []byte) (*MAAInstance, error)

	// ListAgentRulesByRestricted lists the rules an agent created.
	ListAgentRulesByRestricted(ctx context.Context, agent string, limit, offset int) ([]MAARuleRef, error)

	// ListAgentWalletsByOwner lists the wallets an owner funded.
	ListAgentWalletsByOwner(ctx context.Context, owner string, limit, offset int) ([]MAAOwnedWallet, error)

	// ListAgentWalletsByRule lists every wallet funded under a rule.
	ListAgentWalletsByRule(ctx context.Context, ruleID []byte, limit, offset int) ([]MAARuleWallet, error)

	// GetAgentRuleEvents returns a rule's append-only audit log.
	GetAgentRuleEvents(ctx context.Context, ruleID []byte, limit, offset int) ([]MAAEvent, error)

	// IsAgentWallet reports whether an address is a known (joined) agent wallet.
	IsAgentWallet(ctx context.Context, maaAddress []byte) (bool, error)
}
