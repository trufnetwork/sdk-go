package types

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-sql/civil"
	"github.com/kwilteam/kwil-db/core/types/transactions"
	"github.com/pkg/errors"
)

type Taxonomy struct {
	TaxonomyItems []TaxonomyItem
	StartDate     *civil.Date
	EndDate       *civil.Date
}

type TaxonomyUnix struct {
	TaxonomyItems []TaxonomyItem
	StartDate     *int
	EndDate       *int
}

type TaxonomyItem struct {
	ChildStream StreamLocator
	Weight      float64
}

type DescribeTaxonomiesParams struct {
	// LatestVersion if true, will return the latest version of the taxonomy only
	LatestVersion bool
}

type IComposedStream interface {
	// IStream methods are also available in IPrimitiveStream
	IStream
	// DescribeTaxonomies returns the taxonomy of the stream
	DescribeTaxonomies(ctx context.Context, params DescribeTaxonomiesParams) (Taxonomy, error)
	// DescribeTaxonomiesUnix returns the taxonomy of the stream with Unix timestamp
	DescribeTaxonomiesUnix(ctx context.Context, params DescribeTaxonomiesParams) (TaxonomyUnix, error)
	// SetTaxonomy sets the taxonomy of the stream
	SetTaxonomy(ctx context.Context, taxonomies Taxonomy) (transactions.TxHash, error)
	// SetTaxonomyUnix sets the taxonomy of the stream with Unix timestamp
	SetTaxonomyUnix(ctx context.Context, taxonomies TaxonomyUnix) (transactions.TxHash, error)
}

// MarshalJSON Custom marshaler for TaxonomyDefinition
// TaxonomyDefinition -> ["st906974fb3f30a28200e907c604b15b",899]
func (t *TaxonomyItem) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{t.ChildStream.StreamId.String(), t.Weight})
}

// UnmarshalJSON Custom unmarshaller for TaxonomyDefinition
// ["st906974fb3f30a28200e907c604b15b",899] -> TaxonomyDefinition
func (t *TaxonomyItem) UnmarshalJSON(b []byte) error {
	var items []json.RawMessage
	err := json.Unmarshal(b, &items)
	if err != nil {
		return errors.WithStack(err)
	}
	if len(items) != 2 {
		return errors.New(fmt.Sprintf("expected 2 elements, got %d", len(items)))
	}

	// Unmarshal the first item as parentOf type
	if err := json.Unmarshal(items[0], &t.ChildStream.StreamId); err != nil {
		return errors.Wrap(err, "expected string")
	}

	// Unmarshal the second item as weight type
	if err := json.Unmarshal(items[1], &t.Weight); err != nil {
		return errors.Wrap(err, "expected float64")
	}

	return nil
}
