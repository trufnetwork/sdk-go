package types

import (
	"context"
	"github.com/trufnetwork/kwil-db/node/types"

	kwilClientType "github.com/trufnetwork/kwil-db/core/client/types"
)

type InsertRecordInput struct {
	DataProvider string
	StreamId     string
	EventTime    int
	Value        float64
}

type IPrimitiveAction interface {
	// IAction methods are also available in IPrimitiveAction
	IAction
	// InsertRecord insert a recors into the stream
	InsertRecord(ctx context.Context, inputs InsertRecordInput, opts ...kwilClientType.TxOpt) (types.Hash, error)
	// InsertRecords inserts records into the stream
	InsertRecords(ctx context.Context, inputs []InsertRecordInput, opts ...kwilClientType.TxOpt) (types.Hash, error)
	// GetFirstRecordUnix gets the first record of the stream with Unix timestamp
	//GetFirstRecordUnix(ctx context.Context, input GetFirstRecordUnixInput) (*StreamRecordUnix, error)
	// CheckValidPrimitiveStream checks if the stream is a valid primitive stream
	CheckValidPrimitiveStream(ctx context.Context, locator StreamLocator) error
}
