package contractsapi

import (
	"context"
	"github.com/kwilteam/kwil-db/core/client"
	kwilTypes "github.com/kwilteam/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

type DeployStreamInput struct {
	StreamId   util.StreamId    `validate:"required"`
	StreamType types.StreamType `validate:"required"`
	KwilClient *client.Client   `validate:"required"`
	Deployer   []byte           `validate:"required"`
}

// DeployStream deploys a stream to TN
func DeployStream(ctx context.Context, input DeployStreamInput) (kwilTypes.Hash, error) {
	return input.KwilClient.Execute(ctx, "", "create_stream", [][]any{{
		input.StreamId.String(),
		input.StreamType.String(),
	}})
}
