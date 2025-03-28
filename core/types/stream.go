package types

import (
	"context"
	"github.com/cockroachdb/apd/v3"
	"github.com/golang-sql/civil"
	kwilType "github.com/kwilteam/kwil-db/core/types"
	"github.com/kwilteam/kwil-db/node/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"time"
)

type GetRecordInput struct {
	DataProvider string
	StreamId     string
	From         *int
	To           *int
	FrozenAt     *int
	BaseDate     *int
}

type GetIndexInput = GetRecordInput

type GetFirstRecordInput struct {
	AfterDate *civil.Date
	FrozenAt  *time.Time
}

type GetFirstRecordUnixInput struct {
	AfterDate *int
	FrozenAt  *time.Time
}

type StreamRecord struct {
	EventTime int
	Value     apd.Decimal
}

type StreamIndex = StreamRecord

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

type IAction interface {
	// ExecuteProcedure Executes an arbitrary procedure on the stream. Execute refers to the write calls
	ExecuteProcedure(ctx context.Context, procedure string, args [][]any) (types.Hash, error)
	// CallProcedure calls an arbitrary procedure on the stream. Call refers to the read calls
	CallProcedure(ctx context.Context, procedure string, args []any) (*kwilType.QueryResult, error)

	// GetRecord reads the records of the stream within the given date range
	GetRecord(ctx context.Context, input GetRecordInput) ([]StreamRecord, error)
	// GetIndex reads the index of the stream within the given date range
	GetIndex(ctx context.Context, input GetIndexInput) ([]StreamIndex, error)
	// GetType gets the type of the stream -- Primitive or Composed
	GetType(ctx context.Context, locator StreamLocator) (StreamType, error)
	// GetFirstRecord gets the first record of the stream
	//GetFirstRecord(ctx context.Context, input GetFirstRecordInput) (*StreamRecord, error)

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
}
