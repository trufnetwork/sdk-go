package tnclient

import (
	"context"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/logging"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"go.uber.org/zap"
	"time"
)

// DeployComposedStreamWithTaxonomy deploys a composed stream with taxonomy
func (c *Client) DeployComposedStreamWithTaxonomy(ctx context.Context, streamId util.StreamId, taxonomy types.Taxonomy) error {
	// create the stream
	txHashCreate, err := c.DeployStream(ctx, streamId, types.StreamTypeComposed)
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = c.WaitForTx(ctx, txHashCreate, time.Second*10)
	if err != nil {
		return errors.WithStack(err)
	}
	logging.Logger.Info("Deployed stream, with txHash", zap.String("streamId", streamId.String()), zap.String("txHash", txHashCreate.String()))

	// load the composed actions
	stream, err := c.LoadComposedActions()
	if err != nil {
		return errors.WithStack(err)
	}

	// set the taxonomy
	txHashSet, err := stream.InsertTaxonomy(ctx, taxonomy)
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = c.WaitForTx(ctx, txHashSet, time.Second*10)
	if err != nil {
		return errors.WithStack(err)
	}
	logging.Logger.Info("Set taxonomy for stream", zap.String("streamId", streamId.String()), zap.String("txHash", txHashSet.String()))

	return nil
}
