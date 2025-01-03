package types

import (
	"context"
	"github.com/golang-sql/civil"
	"github.com/kwilteam/kwil-db/core/types/transactions"
)

type InsertRecordInput struct {
	DateValue civil.Date
	Value     int
}

type InsertRecordUnixInput struct {
	DateValue int
	Value     int
}

type IPrimitiveStream interface {
	// IStream methods are also available in IPrimitiveStream
	IStream
	// InsertRecords inserts records into the stream
	InsertRecords(ctx context.Context, inputs []InsertRecordInput) (transactions.TxHash, error)
	// InsertRecordsUnix inserts records into the stream
	InsertRecordsUnix(ctx context.Context, inputs []InsertRecordUnixInput) (transactions.TxHash, error)
}
