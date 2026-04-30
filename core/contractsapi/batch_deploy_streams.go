package contractsapi

import (
	"context"

	"github.com/pkg/errors"
	kwilClientType "github.com/trufnetwork/kwil-db/core/client/types"
	"github.com/trufnetwork/kwil-db/core/gatewayclient"
	kwilType "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
)

// BatchDeployStreamsInput defines the input for the BatchDeployStreams function.
type BatchDeployStreamsInput struct {
	KwilClient  *gatewayclient.GatewayClient `validate:"required"`
	Definitions []types.StreamDefinition     `validate:"required"`
	// SchemaName is the name of the schema where create_streams procedure exists.
	// Typically, it might be empty if it's a root/global procedure.
	SchemaName string
}

// BatchDeployStreams deploys multiple streams by calling the create_streams SQL action.
// It returns the transaction hash of the batch operation and an error if the submission fails.
// Waiting for the transaction and parsing individual results should be handled separately.
//
// Each StreamDefinition.AllowZeros (default false) controls per-stream
// persistence of value=0 inserts. The fourth array argument to
// create_streams is omitted entirely when every stream uses the default
// (false), keeping the on-the-wire payload compatible with pre-feature
// nodes that haven't been upgraded.
func BatchDeployStreams(ctx context.Context, input BatchDeployStreamsInput) (kwilType.Hash, error) {
	if len(input.Definitions) == 0 {
		return kwilType.Hash{}, errors.New("no stream definitions provided for batch deployment")
	}

	streamIds := make([]string, len(input.Definitions))
	streamTypes := make([]string, len(input.Definitions))
	allowZeros := make([]bool, len(input.Definitions))
	anyAllowZeros := false

	for i, def := range input.Definitions {
		streamIds[i] = def.StreamId.String()
		streamTypes[i] = string(def.StreamType)
		allowZeros[i] = def.AllowZeros
		if def.AllowZeros {
			anyAllowZeros = true
		}
	}

	// Omit the allow_zeros argument entirely when no stream opted in so
	// the on-the-wire payload stays compatible with pre-feature nodes
	// that haven't been upgraded. Only append the third array when at
	// least one stream needs it, in which case the node must be on the
	// migration that added $allow_zeros.
	inner := []any{streamIds, streamTypes}
	if anyAllowZeros {
		inner = append(inner, allowZeros)
	}
	args := [][]any{inner}

	txHash, err := input.KwilClient.Execute(ctx, input.SchemaName, "create_streams", args, kwilClientType.WithNonce(0))
	if err != nil {
		return kwilType.Hash{}, errors.Wrap(err, "batch deploy transaction failed to execute")
	}

	return txHash, nil
}
