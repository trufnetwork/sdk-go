package types

import (
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/golang-sql/civil"
)

type GetRecordInput struct {
	DateFrom *civil.Date
	DateTo   *civil.Date
	FrozenAt *time.Time
	BaseDate *civil.Date
}

type GetRecordUnixInput struct {
	DateFrom *int
	DateTo   *int
	FrozenAt *time.Time
	BaseDate *int
}

type GetIndexInput = GetRecordInput
type GetIndexUnixInput = GetRecordUnixInput

type GetFirstRecordInput struct {
	AfterDate *civil.Date
	FrozenAt  *time.Time
}

type GetFirstRecordUnixInput struct {
	AfterDate *int
	FrozenAt  *time.Time
}

type StreamRecord struct {
	DateValue civil.Date
	Value     apd.Decimal
}

type StreamRecordUnix struct {
	DateValue int
	Value     apd.Decimal
}

type StreamIndex = StreamRecord
type StreamIndexUnix = StreamRecordUnix

type IActions interface {
	// ExecuteProcedure Executes an arbitrary procedure on the stream. Execute refers to the write calls
	//ExecuteProcedure(ctx context.Context, procedure string, args [][]any) (types.Hash, error)
	// CallProcedure calls an arbitrary procedure on the stream. Call refers to the read calls
	//CallProcedure(ctx context.Context, procedure string, args []any) (*types.QueryResult, error)

	// InitializeStream initializes the stream. Majority of other methods need the stream to be initialized
	//InitializeStream(ctx context.Context) (types.Hash, error)
	// GetRecord reads the records of the stream within the given date range
	//GetRecord(ctx context.Context, input GetRecordInput) ([]StreamRecord, error)
	// GetIndex reads the index of the stream within the given date range
	//GetIndex(ctx context.Context, input GetIndexInput) ([]StreamIndex, error)
	//GetRecordUnix reads the records of the stream within the given date rang
	//GetRecordUnix(ctx context.Context, input GetRecordUnixInput) ([]StreamRecordUnix, error)
	// GetIndexUnix reads the index of the stream within the given date range
	//GetIndexUnix(ctx context.Context, input GetIndexUnixInput) ([]StreamIndexUnix, error)
	// GetType gets the type of the stream -- Primitive or Composed
	//GetType(ctx context.Context) (StreamType, error)
	// GetFirstRecord gets the first record of the stream
	//GetFirstRecord(ctx context.Context, input GetFirstRecordInput) (*StreamRecord, error)

	// SetReadVisibility sets the read visibility of the stream -- Private or Public
	//SetReadVisibility(ctx context.Context, visibility util.VisibilityEnum) (types.Hash, error)
	// GetReadVisibility gets the read visibility of the stream -- Private or Public
	//GetReadVisibility(ctx context.Context) (*util.VisibilityEnum, error)
	// SetComposeVisibility sets the compose visibility of the stream -- Private or Public
	//SetComposeVisibility(ctx context.Context, visibility util.VisibilityEnum) (types.Hash, error)
	// GetComposeVisibility gets the compose visibility of the stream -- Private or Public
	//GetComposeVisibility(ctx context.Context) (*util.VisibilityEnum, error)

	// AllowReadWallet allows a wallet to read the stream, if reading is private
	//AllowReadWallet(ctx context.Context, wallet util.EthereumAddress) (types.Hash, error)
	// DisableReadWallet disables a wallet from reading the stream
	//DisableReadWallet(ctx context.Context, wallet util.EthereumAddress) (types.Hash, error)
	// AllowComposeStream allows a stream to use this stream as child, if composing is private
	//AllowComposeStream(ctx context.Context, locator StreamLocator) (types.Hash, error)
	// DisableComposeStream disables a stream from using this stream as child
	//DisableComposeStream(ctx context.Context, locator StreamLocator) (types.Hash, error)

	// GetAllowedReadWallets gets the wallets allowed to read the stream
	//GetAllowedReadWallets(ctx context.Context) ([]util.EthereumAddress, error)
	// GetAllowedComposeStreams gets the streams allowed to compose this stream
	//GetAllowedComposeStreams(ctx context.Context) ([]StreamLocator, error)

	// SetDefaultBaseDate insert a metadata row with `default_base_date` key
	//SetDefaultBaseDate(ctx context.Context, baseDate string) (types.Hash, error)
}
