package types

// GetRecordRawOutput represents the raw SQL output from get_record, get_index,
// get_index_change, and get_first_record procedures.
//
// This struct is shared across multiple packages to avoid duplication:
// - core/contractsapi (HTTP-based implementations)
// - core/tnclient (transport-aware implementations)
type GetRecordRawOutput struct {
	EventTime string `json:"event_time"`
	Value     string `json:"value"`
}
