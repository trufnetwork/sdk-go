package types

// BridgeHistory represents a transaction history record from the bridge extension.
type BridgeHistory struct {
	Type                string  `json:"type"`
	Amount              string  `json:"amount"` // NUMERIC(78,0) as string
	FromAddress         []byte  `json:"from_address"`
	ToAddress           []byte  `json:"to_address"`
	InternalTxHash      []byte  `json:"internal_tx_hash"`
	ExternalTxHash      []byte  `json:"external_tx_hash"`
	Status              string  `json:"status"`
	BlockHeight         int64   `json:"block_height"`
	BlockTimestamp      int64   `json:"block_timestamp"`
	ExternalBlockHeight *int64  `json:"external_block_height"`
}

// GetHistoryInput is input for GetHistory
type GetHistoryInput struct {
	BridgeIdentifier string `validate:"required"`
	Wallet           string `validate:"required"`
	Limit            *int
	Offset           *int
}
