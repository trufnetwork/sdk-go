package contractsapi

import (
	"context"
	"fmt"
	kwiltypes "github.com/kwilteam/kwil-db/core/types"

	// "github.com/kwilteam/kwil-db/core/types/transactions"
	"github.com/pkg/errors"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"strconv"
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

// checkValidComposedStream checks if the stream is a valid composed stream
// and returns an error if it is not. Valid means:
// - the stream is initialized
// - the stream is a composed stream
func (c *ComposedAction) checkValidComposedStream(ctx context.Context) error {
	// then check if is composed
	//streamType, err := c.GetType(ctx)
	//if err != nil {
	//	return errors.WithStack(err)
	//}

	//if streamType != types.StreamTypeComposed {
	//	return ErrorStreamNotComposed
	//}

	return nil
}

func (c *ComposedAction) checkedExecute(ctx context.Context, method string, args [][]any) (kwiltypes.Hash, error) {
	err := c.checkValidComposedStream(ctx)
	if err != nil {
		return kwiltypes.Hash{}, errors.WithStack(err)
	}

	return c.execute(ctx, method, args)
}

type DescribeTaxonomiesResult struct {
	ChildStreamId     util.StreamId `json:"child_stream_id"`
	ChildDataProvider string        `json:"child_data_provider"`
	// decimals are received as strings by kwil to avoid precision loss
	// as decimal are more arbitrary than golang's float64
	Weight    string `json:"weight"`
	CreatedAt int    `json:"created_at"`
	Version   int    `json:"version"`
	StartDate string `json:"start_date"` // cannot use *string nor *civil.Date as decoding it will cause an error
	EndDate   string `json:"end_date"`   // cannot use *string nor *civil.Date as decoding it will cause an error
}

type DescribeTaxonomiesUnixResult struct {
	ChildStreamId     util.StreamId `json:"child_stream_id"`
	ChildDataProvider string        `json:"child_data_provider"`
	// decimals are received as strings by kwil to avoid precision loss
	// as decimal are more arbitrary than golang's float64
	Weight    string `json:"weight"`
	CreatedAt int    `json:"created_at"`
	Version   int    `json:"version"`
	StartDate int    `json:"start_date"`
	EndDate   int    `json:"end_date"`
}

func (c *ComposedAction) DescribeTaxonomies(ctx context.Context, params types.DescribeTaxonomiesParams) (types.Taxonomy, error) {
	// TODO: Implement this according to the new architecture
	return types.Taxonomy{}, nil
	records, err := c.call(ctx, "describe_taxonomies", []any{params.LatestVersion})
	if err != nil {
		return types.Taxonomy{}, errors.WithStack(err)
	}

	result, err := DecodeCallResult[DescribeTaxonomiesUnixResult](records)
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

		taxonomyItems = append(taxonomyItems, types.TaxonomyItem{
			ChildStream: types.StreamLocator{
				StreamId:     r.ChildStreamId,
				DataProvider: dpAddress,
			},
			Weight: weight,
		})
	}

	var startDateCivil *int
	if len(result) > 0 && result[0].StartDate != 0 {
		startDateCivil = &result[0].StartDate
	}

	return types.Taxonomy{
		TaxonomyItems: taxonomyItems,
		StartDate:     startDateCivil,
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
		childDataProviderString := taxonomy.ChildStream.DataProvider.Address()
		childDataProviders = append(childDataProviders, fmt.Sprintf("%s", childDataProviderString))
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

	var args [][]any
	args = append(args, []any{taxonomies.ParentStream.DataProvider.Address(), taxonomies.ParentStream.StreamId.String(), childDataProviders, childStreamIDs.Strings(), weights, startDate})
	return c.checkedExecute(ctx, "insert_taxonomy", args)
}
