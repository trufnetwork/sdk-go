package contractsapi

import (
	"github.com/kwilteam/kwil-db/core/client"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"

	"github.com/trufnetwork/sdk-go/core/util"
)
import "github.com/go-playground/validator/v10"

type DestroyStreamInput struct {
	StreamId   util.StreamId `validate:"required"`
	KwilClient client.Client `validate:"required"`
}

func (i DestroyStreamInput) Validate() error {
	return validator.New().Struct(i)
}

type DestroyStreamOutput struct {
	TxHash kwiltypes.Hash
}

// DestroyStream destroys a stream from TN
//func DestroyStream(ctx context.Context, input DestroyStreamInput) (*DestroyStreamOutput, error) {
//	if err := input.Validate(); err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	txHash, err := input.KwilClient.DropDatabase(ctx, input.StreamId.String())
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	return &DestroyStreamOutput{
//		TxHash: txHash,
//	}, nil
//}
