package contractsapi

import (
	"context"
	"fmt"
	"github.com/kwilteam/kwil-db/core/types/client"
	"github.com/kwilteam/kwil-db/core/types/transactions"
	"github.com/kwilteam/kwil-db/parse"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/contracts"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

type DeployStreamInput struct {
	StreamId   util.StreamId    `validate:"required"`
	StreamType types.StreamType `validate:"required"`
	KwilClient client.Client    `validate:"required"`
	Deployer   []byte           `validate:"required"`
}

type DeployStreamOutput struct {
	TxHash transactions.TxHash
}

// DeployStream deploys a stream to TN
func DeployStream(ctx context.Context, input DeployStreamInput) (transactions.TxHash, error) {
	contractContent, err := GetContractContent(input)
	schema, err := parse.Parse(contractContent)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	schema.Name = input.StreamId.String()

	return input.KwilClient.DeployDatabase(ctx, schema)
}

// GetContractContent returns the contract content based on the stream type
func GetContractContent(input DeployStreamInput) ([]byte, error) {
	switch input.StreamType {
	case types.StreamTypeComposed:
		return contracts.ComposedContractContent, nil
	case types.StreamTypePrimitive:
		return contracts.PrivateContractContent, nil
	case types.StreamTypeComposedUnix:
		return contracts.ComposedUnixContractContent, nil
	case types.StreamTypePrimitiveUnix:
		return contracts.PrivateUnixContractContent, nil
	default:
		return nil, errors.New(fmt.Sprintf("unknown stream type: %v", input.StreamType))
	}
}
