# Composed Stream Interface

The `IComposedStream` interface extends `IStream` and provides additional methods for managing composed streams.

## Methods

### `DescribeTaxonomies`

```go
DescribeTaxonomies(ctx context.Context, params types.DescribeTaxonomiesParams) ([]types.TaxonomyItem, error)
```

Describes the taxonomies of the composed stream.

**Parameters:**
- `ctx`: The context for the operation.
- `params`: The parameters for describing taxonomies.

**Returns:**
- `[]types.TaxonomyItem`: The described taxonomies.
- `error`: An error if the operation fails.

### `SetTaxonomy`

```go
SetTaxonomy(ctx context.Context, taxonomies []types.TaxonomyItem) (kwiltypes.Hash, error)
```

Sets the taxonomy of the composed stream.

**Parameters:**
- `ctx`: The context for the operation.
- `taxonomies`: The taxonomy items to set.

**Returns:**
- `transactions.TxHash`: The transaction hash for the operation.
- `error`: An error if the operation fails.

