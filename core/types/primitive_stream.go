package types

import (
	"context"

	"github.com/golang-sql/civil"
	"github.com/kwilteam/kwil-db/core/types/client"
	"github.com/kwilteam/kwil-db/core/types/transactions"
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
	InsertRecords(ctx context.Context, inputs []InsertRecordInput, opts ...client.TxOpt) (transactions.TxHash, error)
	// InsertRecordsUnix inserts records into the stream
	InsertRecordsUnix(ctx context.Context, inputs []InsertRecordUnixInput, opts ...client.TxOpt) (transactions.TxHash, error)
}
