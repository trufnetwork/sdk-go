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
- [Primitive Stream](#primitive-stream-interface): Raw data stream management
- [Composed Stream](#composed-stream-interface): Aggregated data stream handling
- [Stream](#stream-interface): Core stream operations and access control

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

## Overview

The Client Interface is the primary entry point for interacting with the TRUF.NETWORK ecosystem. It provides a comprehensive set of methods for managing streams, handling transactions, and interfacing with the underlying blockchain infrastructure.

## Key Features

- Stream lifecycle management
- Transaction handling
- Network interaction
- Address and identity management

## Initialization

### `NewClient`

Create a client connection to the TRUF.NETWORK:

```go
tnClient, err := tnclient.NewClient(
	ctx, 
	"https://gateway.mainnet.truf.network", 
	tnclient.WithSigner(mySigner),
	// Optional configuration options
)
```

## Core Methods

### Transaction Management

#### `WaitForTx`
Waits for a transaction to be mined and confirmed.

```go
txResponse, err := tnClient.WaitForTx(
	ctx, 
	txHash, 
	time.Second * 5  // Polling interval
)
```

### Stream Lifecycle

#### `DeployStream`
Deploy a new stream (primitive or composed):

```go
streamId := util.GenerateStreamId("my-economic-stream")
txHash, err := tnClient.DeployStream(
	ctx, 
	streamId, 
	types.StreamTypePrimitive
)
```

#### `DestroyStream`
Remove an existing stream:

```go
txHash, err := tnClient.DestroyStream(ctx, streamId)
```

### Stream Loading

#### `LoadPrimitiveStream`
Load an existing primitive stream:

```go
primitiveStream, err := tnClient.LoadPrimitiveStream(
	tnClient.OwnStreamLocator(streamId)
)
```

#### `LoadComposedStream`
Load an existing composed stream:

```go
composedStream, err := tnClient.LoadComposedStream(
	tnClient.OwnStreamLocator(streamId)
)
```

### Identity Management

#### `OwnStreamLocator`
Generate a stream locator using the current client's address:

```go
streamLocator := tnClient.OwnStreamLocator(streamId)
```

#### `Address`
Retrieve the client's Ethereum address:

```go
clientAddress := tnClient.Address()
addressString := clientAddress.String()
```

## Example: Complete Stream Workflow

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

## Best Practices

1. **Always handle errors**
2. **Use appropriate context timeouts**
3. **Log important transactions**
4. **Implement retry mechanisms**

## Considerations

- Ensure proper error handling and logging

## Stream Interface

## Overview

The Stream Interface is the core abstraction for data streams in the TRUF.NETWORK ecosystem. It provides a comprehensive set of methods for managing stream lifecycle, visibility, and access control.

## Key Concepts

- **Immutable Data**: Streams store data points that cannot be altered once recorded
- **Visibility Control**: Fine-grained access management
- **Flexible Querying**: Multiple methods for data retrieval
- **Permissions Management**: Granular control over stream access

## Methods

### `GetRecord`

```go
GetRecord(ctx context.Context, input types.GetRecordInput) ([]types.StreamRecord, error)
```

Retrieves the **raw time-series data** for the specified stream. Internally the SDK calls the on-chain action `get_record`, which automatically delegates to either `get_record_primitive` or `get_record_composed` depending on the type of the stream.

**Behaviour**
1. If both `From` and `To` are `nil`, the latest data-point (LOCF-filled for composed streams) is returned.
2. Gap-filling logic is applied to primitive streams so that the value immediately preceding `From` is included—this guarantees that visualisations can safely draw a continuous line.
3. For composed streams, the value is calculated recursively by aggregating the weighted values of all child primitives **at each point in time**.  All permission checks (`read`, `compose`) are enforced inside the SQL action.

**Input fields (types.GetRecordInput):**
- `DataProvider` (string)   Owner address of the stream.
- `StreamId`     (string)   ID of the stream (`stxxxxxxxxxxxxxxxxxxxxxxxxxxxx`).
- `From`, `To`   (*int)     Unix timestamp range (inclusive).  Pass `nil` to make the bound open-ended.
- `FrozenAt`     (*int)     Time-travel flag. Only events created **on or before** this block-timestamp are considered.

**Returned slice:** each `StreamRecord` contains
- `EventTime` (int)   Unix timestamp of the point.
- `Value`     (apd.Decimal) Raw numeric value.

### `GetIndex`

```go
GetIndex(ctx context.Context, input types.GetIndexInput) ([]types.StreamIndex, error)
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

The returned slice is identical to `GetRecord` but semantically represents an **index** instead of raw values.

### `GetIndexChange`

```go
GetIndexChange(ctx context.Context, input types.GetRecordInput, timeInterval int) ([]types.StreamIndex, error)
```

Computes the **percentage change** of the index over a fixed time interval. Internally the SDK obtains the indexed series via `get_index` and then, for every returned row whose timestamp is `t`, finds the closest index value **at or before** `t − timeInterval`.

Formula:
```
Δindex(t) = ( index(t) − index(t − Δ ) ) / index(t − Δ ) × 100
```
where `Δ = timeInterval` (in seconds).

Only rows for which a matching *previous* value exists and is non-zero are emitted. This is performed server-side by the SQL action `get_index_change`, ensuring minimal bandwidth usage.

Typical use-cases:
- **Day-over-day change**: pass `86400` seconds.
- **Year-on-year change**: pass `31 536 000` seconds.

**Extra parameter:**
- `timeInterval` (int)  Interval in seconds used for the delta computation (mandatory).

**Return value:** Same shape as `GetIndex` but each `Value` now represents **percentage change**, e.g. `2.5` means **+2.5 %**.

### `SetReadVisibility`

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

### `SetComposeVisibility`

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

### `AllowReadWallet`

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

### `DisableReadWallet`

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

### `AllowComposeStream`

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

### `DisableComposeStream`

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

### `CallProcedure`

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

#### Example: Calling a Custom Read-Only Procedure

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

## Best Practices

1. **Always handle errors**
2. **Use context with appropriate timeouts**
3. **Validate wallet addresses**
4. **Log permission changes**
5. **Implement retry mechanisms**

## Considerations

- Visibility changes are blockchain transactions

## Primitive Stream Interface

## Overview

Primitive streams are the foundational data sources in the TRUF.NETWORK ecosystem. They represent raw, unprocessed data points that can be used directly or as components in more complex composed streams.

## Key Characteristics

- Direct data input mechanism
- Immutable record storage

## Record Insertion

### `InsertRecords`

```go
InsertRecords(ctx context.Context, inputs []types.InsertRecordInput) (transactions.TxHash, error)
```

Allows insertion of one or multiple records into a primitive stream.

#### Record Input Structure

```go
type InsertRecordInput struct {
	DataProvider string    // Address of the data provider
	StreamId     string    // Unique stream identifier
	EventTime    int       // Unix timestamp of the record
	Value        float64   // Numeric value of the record
}
```

#### Example Usage

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

## Best Practices

1. **Consistent Timestamps**
   - Use UTC timestamps
   - Handle potential time zone complexities

2. **Data Validation**
   - Validate input values before insertion

3. **Error Handling**
   - Implement retry mechanisms
   - Log insertion failures

## Performance Considerations

- Batch record insertions when possible

## Composed Stream Interface

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