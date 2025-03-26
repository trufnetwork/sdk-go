package types

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kwilteam/kwil-db/core/types"
	"github.com/pkg/errors"
)

type Taxonomy struct {
	ParentStream  StreamLocator
	TaxonomyItems []TaxonomyItem
	CreatedAt     int
	GroupSequence int
	StartDate     *int
}

type TaxonomyItem struct {
	ChildStream StreamLocator
	Weight      float64
}

type DescribeTaxonomiesParams struct {
	Stream StreamLocator
	// LatestVersion if true, will return the latest version of the taxonomy only
	LatestVersion bool
}

type IComposedAction interface {
	// IAction methods are also available in IPrimitiveAction
	IAction
	// DescribeTaxonomies returns the taxonomy of the stream with Unix timestamp
	DescribeTaxonomies(ctx context.Context, params DescribeTaxonomiesParams) (Taxonomy, error)
	// SetTaxonomyUnix sets the taxonomy of the stream with Unix timestamp
	InsertTaxonomy(ctx context.Context, taxonomies Taxonomy) (types.Hash, error)
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
