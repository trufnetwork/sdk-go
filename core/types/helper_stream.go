package types

import (
	"context"
	_ "embed"
	"github.com/kwilteam/kwil-db/core/types/transactions"
)

const HelperContractName = "helper_contract"

// ## Types & Interfaces

// IHelperStream defines the interface for helper contract operations
type IHelperStream interface {
	// InsertRecords inserts records into the stream
	InsertRecords(ctx context.Context, inputs TnRecordBatch) (transactions.TxHash, error)
	// InsertRecordsUnix inserts records into the stream
	InsertRecordsUnix(ctx context.Context, inputs TnRecordUnixBatch) (transactions.TxHash, error)
}

// TNRecordRow represents a row in the TN record batch
type TNRecordRow struct {
	DateValue    string
	Value        string
	StreamID     string
	DataProvider string
}

// TnRecordBatch represents a batch of TN records
type TnRecordBatch struct {
	Rows []TNRecordRow
}

// RawInsertRecordsInput represents the input for the insert_records call
type RawInsertRecordsInput struct {
	DataProvider []string `validate:"required"`
	StreamID     []string `validate:"required"`
	DateValue    []string `validate:"required"`
	Value        []string `validate:"required"`
}

// TNRecordUnixRow represents a row in the TN record batch with unix timestamp
type TNRecordUnixRow struct {
	DateValue    string
	Value        string
	StreamID     string
	DataProvider string
}

// TnRecordUnixBatch represents a batch of TN records with unix timestamp
type TnRecordUnixBatch struct {
	Rows []TNRecordUnixRow
}

// RawInsertRecordsUnixInput represents the input for the insert_records_unix call
type RawInsertRecordsUnixInput struct {
	DataProvider []string `validate:"required"`
	StreamID     []string `validate:"required"`
	DateValue    []string `validate:"required"`
	Value        []string `validate:"required"`
}
