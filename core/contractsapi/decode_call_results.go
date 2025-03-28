package contractsapi

import (
	"encoding/json"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/pkg/errors"
)

// DecodeCallResult decodes the result of a view call to the specified struct.
func DecodeCallResult[T any](result *kwiltypes.QueryResult) ([]T, error) {
	// Export returns all of the records in a slice. The map in each slice is
	// equivalent to a Record, which is keyed by the column name.
	records := result.ExportToStringMap()

	// Convert the []map[string]any to JSON bytes
	recordsBytes, err := json.Marshal(records)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal records")
	}

	// Unmarshal JSON bytes into a slice of getMetadataResult
	var results []T
	err = json.Unmarshal(recordsBytes, &results)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal records")
	}

	return results, nil
}
