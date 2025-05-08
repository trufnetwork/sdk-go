package contractsapi

import (
	"context"

	"github.com/kwilteam/kwil-db/core/client"
	kwilClientType "github.com/kwilteam/kwil-db/core/client/types"
	kwilType "github.com/kwilteam/kwil-db/core/types"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
)

// BatchDeployStreamsInput defines the input for the BatchDeployStreams function.
type BatchDeployStreamsInput struct {
	KwilClient  *client.Client           `validate:"required"`
	Definitions []types.StreamDefinition `validate:"required"`
	// SchemaName is the name of the schema where create_streams procedure exists.
	// Typically, it might be empty if it's a root/global procedure.
	SchemaName string
}

// BatchDeployStreams deploys multiple streams by calling the create_streams SQL action.
// It returns the transaction hash of the batch operation and an error if the submission fails.
// Waiting for the transaction and parsing individual results should be handled separately.
func BatchDeployStreams(ctx context.Context, input BatchDeployStreamsInput) (kwilType.Hash, error) {
	if len(input.Definitions) == 0 {
		return kwilType.Hash{}, errors.New("no stream definitions provided for batch deployment")
	}

	streamIds := make([]string, len(input.Definitions))
	streamTypes := make([]string, len(input.Definitions))

	for i, def := range input.Definitions {
		streamIds[i] = def.StreamId.String()
		streamTypes[i] = string(def.StreamType) // Assuming StreamType is string or has String() method
	}

	// The create_streams procedure expects two array arguments: $stream_ids TEXT[], $stream_types TEXT[]
	args := [][]any{{streamIds, streamTypes}}

	txHash, err := input.KwilClient.Execute(ctx, input.SchemaName, "create_streams", args, kwilClientType.WithNonce(0))
	if err != nil {
		return kwilType.Hash{}, errors.Wrap(err, "batch deploy transaction failed to execute")
	}

	return txHash, nil
}
