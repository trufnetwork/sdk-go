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

type IPrimitiveStream interface {
	IStream
	InsertRecords(ctx context.Context, inputs []InsertRecordInput) (transactions.TxHash, error)
}