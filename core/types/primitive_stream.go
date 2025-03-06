package types

import (
	"context"

	"github.com/golang-sql/civil"
	clientTypes "github.com/kwilteam/kwil-db/core/client/types"
	"github.com/kwilteam/kwil-db/core/types"
)

type InsertRecordInput struct {
	DateValue civil.Date
	Value     float64
}

type InsertRecordUnixInput struct {
	DateValue int
	Value     float64
}

type IPrimitiveStream interface {
	// IStream methods are also available in IPrimitiveStream
	IStream
	// InsertRecords inserts records into the stream
	InsertRecords(ctx context.Context, inputs []InsertRecordInput, opts ...clientTypes.TxOpt) (types.Hash, error)
	// InsertRecordsUnix inserts records into the stream
	InsertRecordsUnix(ctx context.Context, inputs []InsertRecordUnixInput, opts ...clientTypes.TxOpt) (types.Hash, error)
	// GetFirstRecordUnix gets the first record of the stream with Unix timestamp
	GetFirstRecordUnix(ctx context.Context, input GetFirstRecordUnixInput) (*StreamRecordUnix, error)
}
