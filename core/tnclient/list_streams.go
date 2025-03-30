package tnclient

import (
	"context"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/contractsapi"
	"github.com/trufnetwork/sdk-go/core/types"
)

// ListStreams returns list streams from the TN network
func (c *Client) ListStreams(ctx context.Context, input types.ListStreamsInput) ([]types.ListStreamsOutput, error) {
	var args []any

	args = append(args, input.DataProvider)
	args = append(args, input.Limit)
	args = append(args, input.Offset)
	args = append(args, input.OrderBy)

	result, err := c.kwilClient.Call(ctx, "", "list_streams", args)
	if err != nil || result.Error != nil {
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return nil, errors.New(*result.Error)
	}

	return contractsapi.DecodeCallResult[types.ListStreamsOutput](result.QueryResult)
}
