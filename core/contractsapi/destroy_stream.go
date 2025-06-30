package contractsapi

import (
	"context"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/kwil-db/core/gatewayclient"
	"github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

type DestroyStreamInput struct {
	StreamId   util.StreamId                `validate:"required"`
	KwilClient *gatewayclient.GatewayClient `validate:"required"`
}

// DestroyStream destroys a stream from TN
func DestroyStream(ctx context.Context, input DestroyStreamInput) (types.Hash, error) {
	addr, _ := auth.EthSecp256k1Authenticator{}.Identifier(input.KwilClient.Signer().CompactID())
	return input.KwilClient.Execute(ctx, "", "delete_stream", [][]any{{
		addr,
		input.StreamId.String(),
	}})
}
