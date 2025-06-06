# Composed Stream Interface

## Overview

The Composed Stream interface provides advanced capabilities for creating and managing aggregated data streams in the TRUF.NETWORK ecosystem.

## Taxonomy Concept

A taxonomy defines how multiple primitive or composed streams are combined to create a new, more complex stream. Key components include:

- **Parent Stream**: The new composed stream being created
- **Child Streams**: Source streams used for aggregation
- **Weights**: Relative importance of each child stream

### Taxonomy Example

```go
taxonomy := types.Taxonomy{
    ParentStream: composedStreamLocator,
    TaxonomyItems: []types.TaxonomyItem{
        {
            ChildStream: primitiveStream1Locator,
            Weight:      0.6,  // 60% contribution
        },
        {
            ChildStream: primitiveStream2Locator,
            Weight:      0.4,  // 40% contribution
        },
    },
    StartDate: &startTimestamp,
}
```

## Methods

### `DescribeTaxonomies`

```go
DescribeTaxonomies(ctx context.Context, params types.DescribeTaxonomiesParams) ([]types.TaxonomyItem, error)
```

Retrieves the current taxonomy configuration for a composed stream.

**Parameters:**
- `ctx`: Operation context
- `params`: Taxonomy description parameters
  - `Stream`: Stream locator
  - `LatestVersion`: Flag to return only the most recent taxonomy

**Returns:**
- List of taxonomy items
- Error if retrieval fails

### `SetTaxonomy`

```go
SetTaxonomy(ctx context.Context, taxonomies []types.TaxonomyItem) (kwiltypes.Hash, error)
```

Configures or updates the taxonomy for a composed stream.

**Parameters:**
- `ctx`: Operation context
- `taxonomies`: Taxonomy configuration

**Returns:**
- Transaction hash
- Error if setting taxonomy fails

## Best Practices

1. Carefully design taxonomy weights

## Error Handling

Always check for errors when working with composed streams:
- Validate taxonomy before setting
- Handle potential child stream access issues
- Manage weight distribution carefully

## Example Usage

```go
// Create a composed stream aggregating market sentiment and economic indicators
composedStreamId := util.GenerateStreamId("market-composite-index")
err := tnClient.DeployStream(ctx, composedStreamId, types.StreamTypeComposed)

composedActions, err := tnClient.LoadComposedActions()
taxonomyTx, err := composedActions.InsertTaxonomy(ctx, types.Taxonomy{
    ParentStream: tnClient.OwnStreamLocator(composedStreamId),
    TaxonomyItems: []types.TaxonomyItem{
        {
            ChildStream: sentimentStreamLocator,
            Weight:      0.6,
        },
        {
            ChildStream: economicIndicatorLocator,
            Weight:      0.4,
        },
    },
})
```