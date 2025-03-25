package contractsapi

//
//import (
//	"context"
//	"fmt"
//	"github.com/golang-sql/civil"
//	kwiltypes "github.com/kwilteam/kwil-db/core/types"
//
//	// "github.com/kwilteam/kwil-db/core/types/transactions"
//	"github.com/pkg/errors"
//	"github.com/trufnetwork/sdk-go/core/types"
//	"github.com/trufnetwork/sdk-go/core/util"
//	"strconv"
//)
//
//type ComposedStream struct {
//	Stream
//}
//
//var _ types.IComposedStream = (*ComposedStream)(nil)
//
//var (
//	ErrorStreamNotComposed = errors.New("stream is not a composed stream")
//)
//
//func ComposedStreamFromStream(stream Stream) (*ComposedStream, error) {
//	return &ComposedStream{
//		Stream: stream,
//	}, nil
//}
//
//func LoadComposedStream(opts NewActionOptions) (*ComposedStream, error) {
//	stream, err := LoadActions(opts)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//
//	return ComposedStreamFromStream(*stream)
//}
//
//// checkValidComposedStream checks if the stream is a valid composed stream
//// and returns an error if it is not. Valid means:
//// - the stream is initialized
//// - the stream is a composed stream
//func (c *ComposedStream) checkValidComposedStream(ctx context.Context) error {
//	// first check if is initialized
//	err := c.checkInitialized(ctx)
//	if err != nil {
//		return errors.WithStack(err)
//	}
//
//	// then check if is composed
//	streamType, err := c.GetType(ctx)
//	if err != nil {
//		return errors.WithStack(err)
//	}
//
//	if streamType != types.StreamTypeComposed {
//		return ErrorStreamNotComposed
//	}
//
//	return nil
//}
//
//func (c *ComposedStream) checkedExecute(ctx context.Context, method string, args [][]any) (kwiltypes.Hash, error) {
//	err := c.checkValidComposedStream(ctx)
//	if err != nil {
//		return kwiltypes.Hash{}, errors.WithStack(err)
//	}
//
//	return c.execute(ctx, method, args)
//}
//
//type DescribeTaxonomiesResult struct {
//	ChildStreamId     util.StreamId `json:"child_stream_id"`
//	ChildDataProvider string        `json:"child_data_provider"`
//	// decimals are received as strings by kwil to avoid precision loss
//	// as decimal are more arbitrary than golang's float64
//	Weight    string `json:"weight"`
//	CreatedAt int    `json:"created_at"`
//	Version   int    `json:"version"`
//	StartDate string `json:"start_date"` // cannot use *string nor *civil.Date as decoding it will cause an error
//	EndDate   string `json:"end_date"`   // cannot use *string nor *civil.Date as decoding it will cause an error
//}
//
//type DescribeTaxonomiesUnixResult struct {
//	ChildStreamId     util.StreamId `json:"child_stream_id"`
//	ChildDataProvider string        `json:"child_data_provider"`
//	// decimals are received as strings by kwil to avoid precision loss
//	// as decimal are more arbitrary than golang's float64
//	Weight    string `json:"weight"`
//	CreatedAt int    `json:"created_at"`
//	Version   int    `json:"version"`
//	StartDate int    `json:"start_date"`
//	EndDate   int    `json:"end_date"`
//}
//
//func (c *ComposedStream) DescribeTaxonomies(ctx context.Context, params types.DescribeTaxonomiesParams) (types.Taxonomy, error) {
//	records, err := c.call(ctx, "describe_taxonomies", []any{params.LatestVersion})
//	if err != nil {
//		return types.Taxonomy{}, errors.WithStack(err)
//	}
//
//	result, err := DecodeCallResult[DescribeTaxonomiesResult](records)
//	if err != nil {
//		return types.Taxonomy{}, errors.WithStack(err)
//	}
//
//	var taxonomyItems []types.TaxonomyItem
//	for _, r := range result {
//		dpAddress, err := util.NewEthereumAddressFromString(r.ChildDataProvider)
//		if err != nil {
//			return types.Taxonomy{}, errors.WithStack(err)
//		}
//		weight, err := strconv.ParseFloat(r.Weight, 64)
//		if err != nil {
//			return types.Taxonomy{}, errors.WithStack(err)
//		}
//
//		taxonomyItems = append(taxonomyItems, types.TaxonomyItem{
//			ChildStream: types.StreamLocator{
//				StreamId:     r.ChildStreamId,
//				DataProvider: dpAddress,
//			},
//			Weight: weight,
//		})
//	}
//
//	var startDateCivil *civil.Date
//	if len(result) > 0 && result[0].StartDate != "" {
//		parsedDate, err := civil.ParseDate(result[0].StartDate)
//		if err != nil {
//			return types.Taxonomy{}, err
//		}
//		startDateCivil = &parsedDate
//	}
//
//	var endDateCivil *civil.Date
//	if len(result) > 0 && result[0].EndDate != "" {
//		parsedDate, err := civil.ParseDate(result[0].EndDate)
//		if err != nil {
//			return types.Taxonomy{}, err
//		}
//		endDateCivil = &parsedDate
//	}
//
//	return types.Taxonomy{
//		TaxonomyItems: taxonomyItems,
//		StartDate:     startDateCivil,
//		EndDate:       endDateCivil,
//	}, nil
//}
//
//func (c *ComposedStream) DescribeTaxonomiesUnix(ctx context.Context, params types.DescribeTaxonomiesParams) (types.TaxonomyUnix, error) {
//	records, err := c.call(ctx, "describe_taxonomies", []any{params.LatestVersion})
//	if err != nil {
//		return types.TaxonomyUnix{}, errors.WithStack(err)
//	}
//
//	result, err := DecodeCallResult[DescribeTaxonomiesUnixResult](records)
//	if err != nil {
//		return types.TaxonomyUnix{}, errors.WithStack(err)
//	}
//
//	var taxonomyItems []types.TaxonomyItem
//	for _, r := range result {
//		dpAddress, err := util.NewEthereumAddressFromString(r.ChildDataProvider)
//		if err != nil {
//			return types.TaxonomyUnix{}, errors.WithStack(err)
//		}
//		weight, err := strconv.ParseFloat(r.Weight, 64)
//		if err != nil {
//			return types.TaxonomyUnix{}, errors.WithStack(err)
//		}
//
//		taxonomyItems = append(taxonomyItems, types.TaxonomyItem{
//			ChildStream: types.StreamLocator{
//				StreamId:     r.ChildStreamId,
//				DataProvider: dpAddress,
//			},
//			Weight: weight,
//		})
//	}
//
//	var startDateCivil *int
//	if len(result) > 0 && result[0].StartDate != 0 {
//		startDateCivil = &result[0].StartDate
//	}
//
//	var endDateCivil *int
//	if len(result) > 0 && result[0].EndDate != 0 {
//		endDateCivil = &result[0].EndDate
//	}
//
//	return types.TaxonomyUnix{
//		TaxonomyItems: taxonomyItems,
//		StartDate:     startDateCivil,
//		EndDate:       endDateCivil,
//	}, nil
//}
//
//func (c *ComposedStream) SetTaxonomy(ctx context.Context, taxonomies types.Taxonomy) (kwiltypes.Hash, error) {
//	var (
//		dataProviders []string
//		streamIDs     util.StreamIdSlice
//		weights       []string
//		startDate     string // null string is not able to be encoded by kwil, so lets left it empty by default
//		endDate       string // null string is not able to be encoded by kwil, so lets left it empty by default
//	)
//
//	for _, taxonomy := range taxonomies.TaxonomyItems {
//		dataProviderHexString := taxonomy.ChildStream.DataProvider.Address()
//		// kwil expects no 0x prefix
//		dataProviderHex := dataProviderHexString[2:]
//		dataProviders = append(dataProviders, fmt.Sprintf("%s", dataProviderHex))
//		streamIDs = append(streamIDs, taxonomy.ChildStream.StreamId)
//		weights = append(weights, fmt.Sprintf("%f", taxonomy.Weight))
//	}
//	if taxonomies.StartDate != nil {
//		startDate = taxonomies.StartDate.String()
//	}
//	if taxonomies.EndDate != nil {
//		endDate = taxonomies.EndDate.String()
//	}
//
//	var args [][]any
//	args = append(args, []any{dataProviders, streamIDs.Strings(), weights, startDate, endDate})
//	return c.checkedExecute(ctx, "set_taxonomy", args)
//}
//
//func (c *ComposedStream) SetTaxonomyUnix(ctx context.Context, taxonomies types.TaxonomyUnix) (kwiltypes.Hash, error) {
//	var (
//		dataProviders []string
//		streamIDs     util.StreamIdSlice
//		weights       []string
//		startDate     int
//		endDate       int
//	)
//
//	for _, taxonomy := range taxonomies.TaxonomyItems {
//		dataProviderHexString := taxonomy.ChildStream.DataProvider.Address()
//		// kwil expects no 0x prefix
//		dataProviderHex := dataProviderHexString[2:]
//		dataProviders = append(dataProviders, fmt.Sprintf("%s", dataProviderHex))
//		streamIDs = append(streamIDs, taxonomy.ChildStream.StreamId)
//		weights = append(weights, fmt.Sprintf("%f", taxonomy.Weight))
//	}
//	if taxonomies.StartDate != nil {
//		startDate = *taxonomies.StartDate
//	}
//	if taxonomies.EndDate != nil {
//		endDate = *taxonomies.EndDate
//	}
//
//	var args [][]any
//	args = append(args, []any{dataProviders, streamIDs.Strings(), weights, startDate, endDate})
//	return c.checkedExecute(ctx, "set_taxonomy", args)
//}
