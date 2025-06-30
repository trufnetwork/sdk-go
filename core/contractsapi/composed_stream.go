package contractsapi

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	kwiltypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

type ComposedAction struct {
	Action
}

var _ types.IComposedAction = (*ComposedAction)(nil)

var (
	ErrorStreamNotComposed = errors.New("stream is not a composed stream")
)

func ComposedStreamFromStream(stream Action) (*ComposedAction, error) {
	return &ComposedAction{
		Action: stream,
	}, nil
}

func LoadComposedActions(opts NewActionOptions) (*ComposedAction, error) {
	stream, err := LoadAction(opts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return ComposedStreamFromStream(*stream)
}

// CheckValidComposedStream checks if the stream is a valid composed stream
// and returns an error if it is not. Valid means:
// - the stream is initialized
// - the stream is a composed stream
func (c *ComposedAction) CheckValidComposedStream(ctx context.Context, locator types.StreamLocator) error {
	// then check if is composed
	streamType, err := c.GetType(ctx, locator)
	if err != nil {
		return errors.WithStack(err)
	}

	if streamType != types.StreamTypeComposed {
		return ErrorStreamNotComposed
	}

	return nil
}

type DescribeTaxonomiesResult struct {
	DataProvider      string `json:"data_provider"`
	StreamId          string `json:"stream_id"`
	ChildDataProvider string `json:"child_data_provider"`
	ChildStreamId     string `json:"child_stream_id"`
	// decimals are received as strings by kwil to avoid precision loss
	// as decimal are more arbitrary than golang's float64
	Weight        string `json:"weight"`
	CreatedAt     string `json:"created_at"`
	GroupSequence string `json:"group_sequence"`
	StartDate     string `json:"start_date"`
}

func (c *ComposedAction) DescribeTaxonomies(ctx context.Context, params types.DescribeTaxonomiesParams) (types.Taxonomy, error) {
	records, err := c.call(ctx, "describe_taxonomies", []any{
		params.Stream.DataProvider.Address(),
		params.Stream.StreamId.String(),
		params.LatestVersion})
	if err != nil {
		return types.Taxonomy{}, errors.WithStack(err)
	}

	result, err := DecodeCallResult[DescribeTaxonomiesResult](records)
	if err != nil {
		return types.Taxonomy{}, errors.WithStack(err)
	}

	var taxonomyItems []types.TaxonomyItem
	for _, r := range result {
		dpAddress, err := util.NewEthereumAddressFromString(r.ChildDataProvider)
		if err != nil {
			return types.Taxonomy{}, errors.WithStack(err)
		}
		weight, err := strconv.ParseFloat(r.Weight, 64)
		if err != nil {
			return types.Taxonomy{}, errors.WithStack(err)
		}

		childStreamId, err := util.NewStreamId(r.ChildStreamId)
		if err != nil {
			return types.Taxonomy{}, errors.WithStack(err)
		}

		taxonomyItems = append(taxonomyItems, types.TaxonomyItem{
			ChildStream: types.StreamLocator{
				StreamId:     *childStreamId,
				DataProvider: dpAddress,
			},
			Weight: weight,
		})
	}

	var (
		startDate     *int
		createdAt     int
		groupSequence int
	)
	if len(result) > 0 {
		if result[0].StartDate != "" {
			startDateInt, err := strconv.Atoi(result[0].StartDate)
			if err != nil {
				return types.Taxonomy{}, errors.WithStack(err)
			}

			startDate = &startDateInt
		}

		if result[0].CreatedAt != "" {
			createdAtInt, err := strconv.Atoi(result[0].CreatedAt)
			if err != nil {
				return types.Taxonomy{}, errors.WithStack(err)
			}

			createdAt = createdAtInt
		}

		if result[0].GroupSequence != "" {
			groupSequenceInt, err := strconv.Atoi(result[0].GroupSequence)
			if err != nil {
				return types.Taxonomy{}, errors.WithStack(err)
			}

			groupSequence = groupSequenceInt
		}
	}

	return types.Taxonomy{
		ParentStream:  types.StreamLocator{StreamId: params.Stream.StreamId, DataProvider: params.Stream.DataProvider},
		TaxonomyItems: taxonomyItems,
		CreatedAt:     createdAt,
		GroupSequence: groupSequence,
		StartDate:     startDate,
	}, nil
}

func (c *ComposedAction) InsertTaxonomy(ctx context.Context, taxonomies types.Taxonomy) (kwiltypes.Hash, error) {
	var (
		childDataProviders []string
		childStreamIDs     util.StreamIdSlice
		weights            kwiltypes.DecimalArray
		startDate          int
	)

	//parentDataProviderHexString := taxonomies.ParentStream.DataProvider.Address()
	// kwil expects no 0x prefix
	//parentDataProviderHex := parentDataProviderHexString[2:]
	for _, taxonomy := range taxonomies.TaxonomyItems {
		childDataProviders = append(childDataProviders, taxonomy.ChildStream.DataProvider.Address())
		childStreamIDs = append(childStreamIDs, taxonomy.ChildStream.StreamId)
		weightNumeric, err := kwiltypes.ParseDecimalExplicit(strconv.FormatFloat(taxonomy.Weight, 'f', -1, 64), 36, 18)
		if err != nil {
			return kwiltypes.Hash{}, errors.WithStack(err)
		}
		weights = append(weights, weightNumeric)
	}
	if taxonomies.StartDate != nil {
		startDate = *taxonomies.StartDate
	}

	return c.execute(ctx, "insert_taxonomy", [][]any{{
		taxonomies.ParentStream.DataProvider.Address(),
		taxonomies.ParentStream.StreamId.String(),
		childDataProviders,
		childStreamIDs.Strings(),
		weights,
		startDate,
	}})
}
