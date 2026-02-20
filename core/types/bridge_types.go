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
	BlockHeight         uint64  `json:"block_height"`
	BlockTimestamp      int64   `json:"block_timestamp"`
	ExternalBlockHeight *int64  `json:"external_block_height"`
}

// GetHistoryInput is input for GetHistory
type GetHistoryInput struct {
	BridgeIdentifier string `validate:"required"`
	Wallet           string `validate:"required"`
	Limit            *int   `validate:"omitempty,min=1"`
	Offset           *int   `validate:"omitempty,min=0"`
}

// WithdrawalProof represents the proofs and signatures needed to claim a withdrawal on EVM.
type WithdrawalProof struct {
	Chain       string   `json:"chain"`
	ChainID     string   `json:"chain_id"`
	Contract    string   `json:"contract"`
	CreatedAt   int64    `json:"created_at"`
	BlockHeight uint64   `json:"block_height"`
	Recipient   string   `json:"recipient"`
	Amount      string   `json:"amount"` // NUMERIC(78,0) as string
	BlockHash   []byte   `json:"block_hash"`
	Root        []byte   `json:"root"`
	Proofs      [][]byte `json:"proofs"`
	Signatures  [][]byte `json:"signatures"`
}

// GetWithdrawalProofInput is input for GetWithdrawalProof
type GetWithdrawalProofInput struct {
	BridgeIdentifier string `validate:"required"`
	Wallet           string `validate:"required"`
}
