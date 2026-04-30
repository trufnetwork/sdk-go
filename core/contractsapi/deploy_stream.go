package contractsapi

import (
	"context"

	"github.com/trufnetwork/kwil-db/core/gatewayclient"
	kwilTypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

type DeployStreamInput struct {
	StreamId   util.StreamId                `validate:"required"`
	StreamType types.StreamType             `validate:"required"`
	KwilClient *gatewayclient.GatewayClient `validate:"required"`
	Deployer   []byte                       `validate:"required"`
	// AllowZeros, when true, opts the new stream out of the value=0 insert
	// filter so zero-valued records persist and surface in get_record. When
	// false (default), zeros are dropped at insert time — today's behavior.
	AllowZeros bool
}

// DeployStream deploys a stream to TN.
//
// When AllowZeros is false (the default), only the legacy two arguments
// are sent so the call shape matches pre-feature nodes that haven't
// learned about $allow_zeros. The third argument is appended only when
// the caller opts in, in which case the node must be on the migration
// that added the parameter.
func DeployStream(ctx context.Context, input DeployStreamInput) (kwilTypes.Hash, error) {
	args := []any{
		input.StreamId.String(),
		input.StreamType.String(),
	}
	if input.AllowZeros {
		args = append(args, input.AllowZeros)
	}
	return input.KwilClient.Execute(ctx, "", "create_stream", [][]any{args})
}
