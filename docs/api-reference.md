# API Reference

## TRUF.NETWORK SDK Overview

The TRUF.NETWORK SDK provides a comprehensive toolkit for developers to interact with decentralized data streams. It enables seamless creation, management, and consumption of economic and financial data streams.

### Key Features

- Stream creation and management
- Primitive and composed stream support
- Flexible data retrieval
- Advanced permission management
- Secure, blockchain-backed data streams

## Interfaces

The SDK is structured around several key interfaces:

- [Client](#client-interface): Primary entry point for network interactions
- [Stream](#stream-interface): Core stream operations and access control
- [Primitive Stream](#primitive-stream-interface): Raw data stream management
- [Composed Stream](#composed-stream-interface): Aggregated data stream handling

## Core Concepts

### Streams

- **Primitive Streams**: Direct data sources with raw data points
- **Composed Streams**: Aggregated streams combining multiple data sources

### Data Management

- Secure, immutable data recording
- Flexible querying and indexing
- Granular access control

## Example Usage

```go
package main

import (
	"context"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

func main() {
	ctx := context.Background()

	// Initialize client with mainnet endpoint
	tnClient, err := tnclient.NewClient(
		ctx,
		"https://gateway.mainnet.truf.network",
		tnclient.WithSigner(mySigner),
	)
	if err != nil {
		// Handle client initialization error
	}

	// Deploy a primitive stream
	streamId := util.GenerateStreamId("my-economic-stream")
	deployTx, err := tnClient.DeployStream(
		ctx,
		streamId,
		types.StreamTypePrimitive,
	)
	// Handle deployment and further stream operations
}
```

## Getting Started

1. Install the SDK
2. Configure your network endpoint
3. Initialize a client
4. Create and manage streams

## Support and Community

- [GitHub Repository](https://github.com/trufnetwork/sdk-go)
- [Issue Tracker](https://github.com/trufnetwork/sdk-go/issues)
- [Documentation](https://docs.truf.network)

## Client Interface

### Overview

The Client Interface is the primary entry point for interacting with the TRUF.NETWORK ecosystem. It provides a comprehensive set of methods for managing streams, handling transactions, and interfacing with the underlying blockchain infrastructure.

### Key Features

- Stream lifecycle management
- Transaction handling
- Network interaction
- Address and identity management

### Initialization

#### `NewClient`

Create a client connection to the TRUF.NETWORK:

```go
tnClient, err := tnclient.NewClient(
	ctx,
	"https://gateway.mainnet.truf.network",
	tnclient.WithSigner(mySigner),
	// Optional configuration options
)
```

### Core Methods

#### Transaction Management

##### `WaitForTx`

Waits for a transaction to be mined and confirmed.

```go
txResponse, err := tnClient.WaitForTx(
	ctx,
	txHash,
	time.Second * 5  // Polling interval
)
```

#### Stream Lifecycle

##### `DeployStream`

Deploy a new stream (primitive or composed):

```go
streamId := util.GenerateStreamId("my-economic-stream")
txHash, err := tnClient.DeployStream(
	ctx,
	streamId,
	types.StreamTypePrimitive
)
```

##### `DestroyStream`

Remove an existing stream:

```go
txHash, err := tnClient.DestroyStream(ctx, streamId)
```

#### Stream Loading

##### `LoadPrimitiveStream`

Load an existing primitive stream:

```go
primitiveStream, err := tnClient.LoadPrimitiveStream(
	tnClient.OwnStreamLocator(streamId)
)
```

##### `LoadComposedStream`

Load an existing composed stream:

```go
composedStream, err := tnClient.LoadComposedStream(
	tnClient.OwnStreamLocator(streamId)
)
```

#### Identity Management

##### `OwnStreamLocator`

Generate a stream locator using the current client's address:

```go
streamLocator := tnClient.OwnStreamLocator(streamId)
```

##### `Address`

Retrieve the client's Ethereum address:

```go
clientAddress := tnClient.Address()
addressString := clientAddress.String()
```

### Example: Complete Stream Workflow

```go
func createAndManageStream(ctx context.Context, tnClient *tnclient.Client) error {
	// Generate unique stream ID
	streamId := util.GenerateStreamId("market-data-stream")

	// Deploy stream
	deployTx, err := tnClient.DeployStream(
		ctx,
		streamId,
		types.StreamTypePrimitive,
	)
	if err != nil {
		return fmt.Errorf("stream deployment failed: %v", err)
	}

	// Wait for deployment confirmation
	_, err = tnClient.WaitForTx(ctx, deployTx, time.Second * 5)
	if err != nil {
		return fmt.Errorf("deployment confirmation failed: %v", err)
	}

	// Load the stream
	primitiveStream, err := tnClient.LoadPrimitiveStream(
		tnClient.OwnStreamLocator(streamId)
	)
	if err != nil {
		return fmt.Errorf("stream loading failed: %v", err)
	}

	// Perform stream operations...
	return nil
}
```

### Best Practices

1. **Always handle errors**
2. **Use appropriate context timeouts**
3. **Log important transactions**
4. **Implement retry mechanisms**

### Considerations

- Ensure proper error handling and logging

## Stream Interface

### Overview

The Stream Interface is the core abstraction for data streams in the TRUF.NETWORK ecosystem. It provides a comprehensive set of methods for managing stream lifecycle, visibility, and access control.

### Key Concepts

- **Immutable Data**: Streams store data points that cannot be altered once recorded
- **Visibility Control**: Fine-grained access management
- **Flexible Querying**: Multiple methods for data retrieval
- **Permissions Management**: Granular control over stream access
- **Unified Data Types**: All stream data operations return `StreamResult` or `ActionResult` for consistency

### Data Type Unification

The SDK uses a unified approach for all stream data operations:

- **StreamResult**: Core data structure with `EventTime` and `Value` fields
- **ActionResult**: Contains an array of `StreamResult` plus `CacheMetadata`
- All data retrieval methods (`GetRecord`, `GetIndex`, `GetIndexChange`) return `ActionResult`
- This unified approach eliminates the need for separate `StreamIndex` and `StreamIndexChange` types, and provides cache metadata by default

### Cache Metadata

All stream data operations return cache metadata that provides insights into query performance and cache behavior:

#### CacheMetadata Structure

```go
type CacheMetadata struct {
    // Cache hit/miss statistics
    CacheHit      bool  `json:"cache_hit"`        // Whether the query hit the cache
    CacheDisabled bool  `json:"cache_disabled"`   // Whether caching is disabled
    
    // Cache timing information
    CachedAt      *int64 `json:"cached_at"`       // Unix timestamp when data was cached
    
    // Query context (populated by SDK)
    StreamId      string `json:"stream_id"`       // Stream identifier
    DataProvider  string `json:"data_provider"`   // Data provider address
    From          *int64 `json:"from"`           // Query start time
    To            *int64 `json:"to"`             // Query end time
    FrozenAt      *int64 `json:"frozen_at"`      // Time-travel timestamp
    RowsServed    int    `json:"rows_served"`    // Number of rows returned
}
```

#### Cache Metadata Methods

The `CacheMetadata` type provides helper methods for analyzing cache performance:

- `GetDataAge() *time.Duration`: Returns the age of cached data. Returns `nil` if no cache timestamp is available.
- `IsExpired(maxAge time.Duration) bool`: Checks if cached data is older than the specified duration.

#### Performance Analysis

Use cache metadata to optimize query performance:

```go
result, err := primitiveActions.GetRecord(ctx, types.GetRecordInput{
    DataProvider: provider,
    StreamId:     streamId,
    From:         &from,
    To:           &to,
    UseCache:     &[]bool{true}[0],
})
if err != nil {
    return err
}

// Analyze cache performance
if result.Metadata.CacheHit {
    fmt.Printf("Cache hit! Served %d rows from cache\n", result.Metadata.RowsServed)
    if dataAge := result.Metadata.GetDataAge(); dataAge != nil {
        fmt.Printf("Cache age: %v\n", *dataAge)
    }
} else {
    fmt.Printf("Cache miss - data retrieved from database\n")
}

// Check if cache data is too old
if result.Metadata.IsExpired(5 * time.Minute) {
    fmt.Println("Warning: Cache data is older than 5 minutes")
}
```

#### Cache Metadata Aggregation

For batch operations, use `AggregateCacheMetadata` to analyze overall cache performance:

```go
// Collect metadata from multiple queries
var metadataList []types.CacheMetadata
// ... perform multiple queries and collect metadata ...

// Aggregate statistics
aggregated := types.AggregateCacheMetadata(metadataList)
fmt.Printf("Cache hit rate: %.2f%% (%d hits / %d queries)\n", 
    aggregated.CacheHitRate * 100, 
    aggregated.CacheHits, 
    aggregated.TotalQueries)
fmt.Printf("Total rows served: %d\n", aggregated.TotalRowsServed)
```

### Methods

#### `GetRecord`

```go
GetRecord(ctx context.Context, input types.GetRecordInput) (types.ActionResult, error)
```

Retrieves the **raw time-series data** for the specified stream, including cache metadata. Internally the SDK calls the on-chain action `get_record`, which automatically delegates to either `get_record_primitive` or `get_record_composed` depending on the type of the stream.

**Returns `types.ActionResult`:**
- `Results`: Array of `StreamResult` containing the actual data
- `Metadata`: Cache performance and hit/miss statistics

**Usage Example:**
```go
// Basic usage without caching
result, err := primitiveActions.GetRecord(ctx, types.GetRecordInput{
    DataProvider: provider,
    StreamId:     streamId,
    From:         &from,
    To:           &to,
})
if err != nil {
    return err
}

// Access the results
for _, record := range result.Results {
    fmt.Printf("Time: %d, Value: %s\n", record.EventTime, record.Value.String())
}

// Access cache metadata
fmt.Printf("Cache Hit: %v\n", result.Metadata.CacheHit)
fmt.Printf("Rows Served: %d\n", result.Metadata.RowsServed)
```

**Cache-Optimized Usage:**
```go
// Enable caching for improved performance
useCache := true
result, err := primitiveActions.GetRecord(ctx, types.GetRecordInput{
    DataProvider: provider,
    StreamId:     streamId,
    From:         &from,
    To:           &to,
    UseCache:     &useCache,
})
if err != nil {
    return err
}

// Performance analysis
if result.Metadata.CacheHit {
    fmt.Printf("✓ Cache hit! Query served in optimized time\n")
    if dataAge := result.Metadata.GetDataAge(); dataAge != nil {
        fmt.Printf("Cache age: %v\n", *dataAge)
    }
} else {
    fmt.Printf("○ Cache miss - data retrieved from source\n")
}

// Validate cache freshness
if result.Metadata.IsExpired(10 * time.Minute) {
    fmt.Println("⚠ Warning: Cache data is older than 10 minutes")
}
```

**Behaviour**

1. If both `From` and `To` are `nil`, the latest data-point (LOCF-filled for composed streams) is returned.
2. Gap-filling logic is applied to primitive streams so that the value immediately preceding `From` is included—this guarantees that visualisations can safely draw a continuous line.
3. For composed streams, the value is calculated recursively by aggregating the weighted values of all child primitives **at each point in time**. All permission checks (`read`, `compose`) are enforced inside the SQL action.

**Input fields (types.GetRecordInput):**

- `DataProvider` (string) Owner address of the stream.
- `StreamId` (string) ID of the stream (`stxxxxxxxxxxxxxxxxxxxxxxxxxxxx`).
- `From`, `To` (\*int) Unix timestamp range (inclusive). Pass `nil` to make the bound open-ended.
- `FrozenAt` (\*int) Time-travel flag. Only events created **on or before** this block-timestamp are considered.
- `BaseDate` (\*int) Base date for index calculations. If not provided, defaults to the stream's `default_base_time` metadata.
- `Prefix` (\*string) Optional prefix filter for stream operations.
- `UseCache` (\*bool) Enable/disable caching for this query. When `nil`, defaults to `false`. When `true`, enables server-side caching for improved performance on repeated queries.

**Returned slice:** each `StreamResult` contains

- `EventTime` (int) Unix timestamp of the point.
- `Value` (apd.Decimal) Raw numeric value.

#### `GetIndex`

```go
GetIndex(ctx context.Context, input types.GetIndexInput) ([]types.StreamResult, error)
```

Returns a **rebased index** of the stream where the value at `BaseDate` (defaults to metadata key `default_base_time`) is normalised to **100**.

Mathematically:

```
index(t) = 100 × value(t) / value(baseDate)
```

The same recursive aggregation, gap-filling and permission rules described in `GetRecord` apply here; the only difference is the final normalisation step.

Important details

1. If `BaseDate` is `nil` the function will fall back to the first available record for the stream.
2. Division-by-zero protection is enforced in the SQL action—an error is thrown when the base value is 0.
3. For single-point queries (`From==To==nil`) only the latest indexed value is returned.

The returned `types.ActionResult` has the same structure as `GetRecord` but semantically represents an **index** instead of raw values, with each record's `Value` field containing the indexed data. Access the results via the `Results` field.

#### `GetIndexChange`

```go
GetIndexChange(ctx context.Context, input types.GetIndexChangeInput) (types.ActionResult, error)
```

Computes the **percentage change** of the index over a fixed time interval. Internally the SDK obtains the indexed series via `get_index` and then, for every returned row whose timestamp is `t`, finds the closest index value **at or before** `t − timeInterval`.

Formula:

```
Δindex(t) = ( index(t) − index(t − Δ ) ) / index(t − Δ ) × 100
```

where `Δ = timeInterval` (in seconds).

Only rows for which a matching _previous_ value exists and is non-zero are emitted. This is performed server-side by the SQL action `get_index_change`, ensuring minimal bandwidth usage.

Typical use-cases:

- **Day-over-day change**: pass `86400` seconds.
- **Year-on-year change**: pass `31 536 000` seconds.

**Input fields (types.GetIndexChangeInput):**

All fields from `GetIndexInput` plus:
- `TimeInterval` (int) Interval in seconds used for the delta computation (mandatory).
- `UseCache` (\*bool) Enable/disable caching for this query. When `nil`, defaults to `false`. When `true`, enables server-side caching for improved performance on repeated queries.

**Return value:** Returns `types.ActionResult` where each `Value` in the `Results` array represents **percentage change**, e.g. `2.5` means **+2.5 %**.

#### `GetFirstRecord`

```go
GetFirstRecord(ctx context.Context, input types.GetFirstRecordInput) (types.ActionResult, error)
```

Retrieves the first record from a stream, optionally after a specified timestamp.

**Parameters:**

- `ctx`: The context for the operation
- `input`: GetFirstRecordInput containing query parameters

**Input fields (types.GetFirstRecordInput):**

- `DataProvider` (string): Owner address of the stream
- `StreamId` (string): ID of the stream
- `After` (\*int): Optional timestamp to search after. If provided, returns the first record after this time
- `FrozenAt` (\*int): Time-travel flag. Only events created on or before this block-timestamp are considered
- `UseCache` (\*bool): Enable/disable caching for this query. When `nil`, defaults to `false`

**Returns `types.ActionResult`:**
- `Results`: Array containing a single `StreamResult` with the first record
- `Metadata`: Cache performance and hit/miss statistics

**Usage Example:**
```go
// Get the very first record in a stream
result, err := primitiveActions.GetFirstRecord(ctx, types.GetFirstRecordInput{
    DataProvider: provider,
    StreamId:     streamId,
    UseCache:     &[]bool{true}[0],
})
if err != nil {
    return err
}

if len(result.Results) > 0 {
    firstRecord := result.Results[0]
    fmt.Printf("First record: Time=%d, Value=%s\n", 
        firstRecord.EventTime, firstRecord.Value.String())
}

// Get the first record after a specific timestamp
after := int(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
result, err = primitiveActions.GetFirstRecord(ctx, types.GetFirstRecordInput{
    DataProvider: provider,
    StreamId:     streamId,
    After:        &after,
    UseCache:     &[]bool{true}[0],
})
```

#### `SetReadVisibility`

```go
SetReadVisibility(ctx context.Context, visibility util.VisibilityEnum) (transactions.TxHash, error)
```

Sets the read visibility of the stream.

**Parameters:**

- `ctx`: The context for the operation.
- `visibility`: The visibility setting (`Public`, `Private`).

**Returns:**

- `transactions.TxHash`: The transaction hash for the operation.
- `error`: An error if the operation fails.

#### `SetComposeVisibility`

```go
SetComposeVisibility(ctx context.Context, visibility util.VisibilityEnum) (transactions.TxHash, error)
```

Sets the compose visibility of the stream.

**Parameters:**

- `ctx`: The context for the operation.
- `visibility`: The visibility setting (`Public`, `Private`).

**Returns:**

- `transactions.TxHash`: The transaction hash for the operation.
- `error`: An error if the operation fails.

#### `AllowReadWallet`

```go
AllowReadWallet(ctx context.Context, wallet util.EthereumAddress) (transactions.TxHash, error)
```

Allows a wallet to read the stream.

**Parameters:**

- `ctx`: The context for the operation.
- `wallet`: The Ethereum address of the wallet.

**Returns:**

- `transactions.TxHash`: The transaction hash for the operation.
- `error`: An error if the operation fails.

#### `DisableReadWallet`

```go
DisableReadWallet(ctx context.Context, wallet util.EthereumAddress) (transactions.TxHash, error)
```

Disables a wallet from reading the stream.

**Parameters:**

- `ctx`: The context for the operation.
- `wallet`: The Ethereum address of the wallet.

**Returns:**

- `transactions.TxHash`: The transaction hash for the operation.
- `error`: An error if the operation fails.

#### `AllowComposeStream`

```go
AllowComposeStream(ctx context.Context, locator StreamLocator) (transactions.TxHash, error)
```

Allows a stream to use this stream as a child.

**Parameters:**

- `ctx`: The context for the operation.
- `locator`: The locator of the composed stream.

**Returns:**

- `transactions.TxHash`: The transaction hash for the operation.
- `error`: An error if the operation fails.

#### `DisableComposeStream`

```go
DisableComposeStream(ctx context.Context, locator StreamLocator) (transactions.TxHash, error)
```

Disables a stream from using this stream as a child.

**Parameters:**

- `ctx`: The context for the operation.
- `locator": The locator of the composed stream.

**Returns:**

- `transactions.TxHash`: The transaction hash for the operation.
- `error`: An error if the operation fails.

#### `CallProcedure`

```go
CallProcedure(ctx context.Context, procedure string, args []any) (*kwiltypes.QueryResult, error)
```

Invokes a **read-only** stored procedure on the underlying database and returns a `QueryResult` that you can inspect or decode into typed structs using `contractsapi.DecodeCallResult[T]`.

**Parameters:**

- `ctx`: Operation context.
- `procedure`: The name of the stored procedure to execute.
- `args`: A positional slice (`[]any`) containing the arguments expected by the procedure. Use `nil` for optional parameters you wish to skip.

**Returns:**

- `*kwiltypes.QueryResult`: The raw query result.
- `error`: An error if the call fails.

##### Example: Calling a Custom Read-Only Procedure

```go
// Load the generic Action API
actions, _ := tnClient.LoadActions()

// Prepare arguments
from := int(time.Now().AddDate(0, 0, -7).Unix())
to   := int(time.Now().Unix())
args := []any{from, to, nil, nil, 31_536_000} // 1-year interval

// Call the procedure
result, err := actions.CallProcedure(ctx, "get_divergence_index_change", args)
if err != nil {
	return err
}

fmt.Println("Columns:", result.ColumnNames)
for _, row := range result.Values {
	fmt.Println(row)
}
```

### Performance Optimization

#### Cache Strategy

The SDK provides intelligent caching to optimize query performance:

**When to Use Caching:**
- Repeated queries with identical parameters
- Dashboard or monitoring applications
- Data visualization with frequent refreshes
- Batch processing where data consistency is acceptable

**Cache Behavior:**
- Cache is pre-configured for specific streams by node operators
- No automatic invalidation when new data arrives - cache refreshes periodically based on operator configuration
- When `FrozenAt` or `BaseDate` parameters are specified, cache is bypassed
- Cache date is returned allowing users to determine acceptable data freshness
- Users can contact node operators for additional cached streams or host their own node

**Performance Tips:**
```go
// 1. Use caching for repeated queries
useCache := true
result, err := stream.GetRecord(ctx, types.GetRecordInput{
    DataProvider: provider,
    StreamId:     streamId,
    UseCache:     &useCache,
})

// 2. Monitor cache hit rates (batch example)
if aggregated.CacheHitRate < 0.5 {
    log.Printf("Low cache hit rate: %.2f%%", aggregated.CacheHitRate*100)
}

// 3. Check cache date to determine data freshness
// The cache doesn't expire but shows when data was cached
if time.Since(result.Metadata.CachedAt) > 5*time.Minute {
    // Data is older than 5 minutes - decide if this is acceptable
    // Contact node operator if more frequent updates are needed
}
```

#### Batch Operations

For multiple stream queries, leverage batch operations and cache aggregation:

```go
// Batch cache analysis
var allMetadata []types.CacheMetadata

// Perform multiple queries
for _, streamId := range streamIds {
    result, err := stream.GetRecord(ctx, types.GetRecordInput{
        DataProvider: provider,
        StreamId:     streamId,
        UseCache:     &[]bool{true}[0],
    })
    if err != nil {
        continue
    }
    allMetadata = append(allMetadata, result.Metadata)
}

// Analyze overall performance
aggregated := types.AggregateCacheMetadata(allMetadata)
fmt.Printf("Overall cache performance: %.2f%% hit rate\n", 
    aggregated.CacheHitRate*100)
```

#### Query Optimization

**Time Range Queries:**
- Use specific time ranges instead of open-ended queries when possible
- Note: `FrozenAt` parameter bypasses cache - use for consistent historical data when cache freshness is not suitable
- Consider pagination for large datasets

**Index Operations:**
- Note: `BaseDate` parameter bypasses cache - use when precise index calculations are required
- For frequently accessed base dates, consider working with node operators to ensure proper caching
- Monitor cache metadata to understand data freshness for your use case

### Best Practices

1. **Always handle errors**
2. **Use context with appropriate timeouts**
3. **Validate wallet addresses**
4. **Log permission changes**
5. **Implement retry mechanisms**
6. **Use caching strategically for improved performance**
7. **Monitor cache hit rates and data freshness**
8. **Aggregate cache metadata for batch operations**

### Considerations

- Visibility changes are blockchain transactions
- Cache metadata is always returned, even when caching is disabled
- Cache refresh intervals are configured by node operators
- Cache is bypassed when `FrozenAt` or `BaseDate` parameters are used

## Primitive Stream Interface

### Overview

Primitive streams are the foundational data sources in the TRUF.NETWORK ecosystem. They represent raw, unprocessed data points that can be used directly or as components in more complex composed streams.

### Key Characteristics

- Direct data input mechanism
- Immutable record storage

### Record Insertion

#### `InsertRecords`

```go
InsertRecords(ctx context.Context, inputs []types.InsertRecordInput) (transactions.TxHash, error)
```

Allows insertion of one or multiple records into a primitive stream.

##### Record Input Structure

```go
type InsertRecordInput struct {
	DataProvider string    // Address of the data provider
	StreamId     string    // Unique stream identifier
	EventTime    int       // Unix timestamp of the record
	Value        float64   // Numeric value of the record
}
```

##### Example Usage

```go
// Insert a single record
records := []types.InsertRecordInput{
	{
		DataProvider: myAddress,
		StreamId:     "my-economic-stream",
		EventTime:    int(time.Now().Unix()),
		Value:        105.75,  // Economic indicator value
	},
}

txHash, err := primitiveStream.InsertRecords(ctx, records)
```

### Best Practices

1. **Consistent Timestamps**

   - Use UTC timestamps
   - Handle potential time zone complexities

2. **Data Validation**

   - Validate input values before insertion

3. **Error Handling**
   - Implement retry mechanisms
   - Log insertion failures

### Performance Considerations

- Batch record insertions when possible

## Composed Stream Interface

### Overview

The Composed Stream interface provides advanced capabilities for creating and managing aggregated data streams in the TRUF.NETWORK ecosystem.

### Taxonomy Concept

A taxonomy defines how multiple primitive or composed streams are combined to create a new, more complex stream. Key components include:

- **Parent Stream**: The new composed stream being created
- **Child Streams**: Source streams used for aggregation
- **Weights**: Relative importance of each child stream

#### Taxonomy Example

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

### Methods

#### `DescribeTaxonomies`

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

#### `SetTaxonomy`

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

### Best Practices

1. Carefully design taxonomy weights

### Error Handling

Always check for errors when working with composed streams:

- Validate taxonomy before setting
- Handle potential child stream access issues
- Manage weight distribution carefully

### Example Usage

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
