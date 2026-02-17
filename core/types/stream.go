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

	// GetStreamOwner gets the owner of the stream
	GetStreamOwner(ctx context.Context, locator StreamLocator) ([]byte, error)

	// BatchStreamExists checks for the existence of multiple streams.
	BatchStreamExists(ctx context.Context, streamsInput []StreamLocator) ([]StreamExistsResult, error)

	// BatchFilterStreamsByExistence filters a list of streams based on their existence in the database.
	// Use this instead of BatchStreamExists if you want less data returned.
	BatchFilterStreamsByExistence(ctx context.Context, streamsInput []StreamLocator, returnExisting bool) ([]StreamLocator, error)

	// GetHistory retrieves the transaction history for a wallet on a specific bridge
	GetHistory(ctx context.Context, input GetHistoryInput) ([]BridgeHistory, error)
}
