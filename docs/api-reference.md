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
- [Composed Stream](#composed-stream-interface): Aggregated data stream handling and taxonomy management
- [Transaction Actions](#transaction-actions-interface): Query transaction history, fees, and distributions
- [Attestation Actions](#attestation-actions-interface): Request and parse cryptographically signed attestations for on-chain verification

## Core Concepts

### Streams

- **Primitive Streams**: Direct data sources with raw data points
- **Composed Streams**: Aggregated streams combining multiple data sources

### Data Management

- Secure, immutable data recording
- Flexible querying and indexing
- Granular access control

### Cache Support

The SDK supports transparent caching through an optional `useCache` parameter on data retrieval methods (`GetRecord`, `GetIndex`, `GetFirstRecord`, `GetIndexChange`). When enabled, queries can leverage node-side caching for improved performance, with detailed cache metadata returned in all responses.

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

### Configuration Options

The SDK provides flexible configuration through functional options:

#### Standard Configuration

```go
client, err := tnclient.NewClient(
	ctx,
	"https://gateway.mainnet.truf.network",
	tnclient.WithSigner(signer),        // Required: Authentication signer
	tnclient.WithLogger(logger),        // Optional: Custom logger
)
```

#### Advanced Configuration

**WithTransport** - Use custom transport implementation:

```go
// For specialized environments (e.g., Chainlink Runtime Environment)
customTransport, err := NewCustomTransport(...)
if err != nil {
	return err
}

client, err := tnclient.NewClient(ctx, endpoint,
	tnclient.WithTransport(customTransport),
	tnclient.WithSigner(signer),
)
```

**Use cases for custom transports:**
- Chainlink Runtime Environment (CRE) workflows
- Testing with mock transports
- Custom HTTP client requirements
- Alternative communication protocols

#### Chainlink Runtime Environment (CRE)

The SDK provides specialized options for Chainlink Runtime Environment workflows.

##### WithCRETransport

Configures the client to use CRE's HTTP client instead of standard `net/http`.

```go
func WithCRETransport(runtime cre.NodeRuntime, endpoint string) Option
```

**Parameters:**
- `runtime` (cre.NodeRuntime): The NodeRuntime from `cre.RunInNodeMode()`
- `endpoint` (string): TRUF.NETWORK gateway URL

**Build Requirement:** Must use `//go:build wasip1` tag

**Example:**

```go
//go:build wasip1

package main

import (
    "github.com/smartcontractkit/cre-sdk-go/cre"
    "github.com/trufnetwork/sdk-go/core/tnclient"
)

func onTrigger(config *Config, runtime cre.Runtime) (*Result, error) {
    return cre.RunInNodeMode(config, runtime,
        func(config *Config, nodeRuntime cre.NodeRuntime) (*Result, error) {
            client, err := tnclient.NewClient(ctx, config.Endpoint,
                tnclient.WithCRETransport(nodeRuntime, config.Endpoint),
            )
            if err != nil {
                return nil, err
            }

            // All read operations work
            streams, err := client.ListStreams(ctx, types.ListStreamsInput{})
            actions, err := client.LoadActions()
            records, err := actions.GetRecord(ctx, getRecordInput)

            return &Result{Records: records}, nil
        },
        cre.ConsensusAggregationFromTags[*Result](),
    ).Await()
}
```

**When to use:**
- CRE workflows requiring read-only access
- Listing streams
- Reading records
- Querying data

---

##### WithCRETransportAndSigner

Convenience function combining signer and CRE transport configuration for write operations.

```go
func WithCRETransportAndSigner(runtime cre.NodeRuntime, endpoint string, signer auth.Signer) Option
```

**Parameters:**
- `runtime` (cre.NodeRuntime): The NodeRuntime from `cre.RunInNodeMode()`
- `endpoint` (string): TRUF.NETWORK gateway URL
- `signer` (auth.Signer): Cryptographic signer for transactions

**Build Requirement:** Must use `//go:build wasip1` tag

**Example:**

```go
//go:build wasip1

package main

import (
    "github.com/smartcontractkit/cre-sdk-go/cre"
    "github.com/trufnetwork/kwil-db/core/crypto/auth"
    "github.com/trufnetwork/sdk-go/core/tnclient"
)

func onTrigger(config *Config, runtime cre.Runtime) (*Result, error) {
    return cre.RunInNodeMode(config, runtime,
        func(config *Config, nodeRuntime cre.NodeRuntime) (*Result, error) {
            // Create signer
            signer := &auth.EthPersonalSigner{Key: privateKey}

            // Create client with both transport and signer
            client, err := tnclient.NewClient(ctx, config.Endpoint,
                tnclient.WithCRETransportAndSigner(nodeRuntime, config.Endpoint, signer),
            )
            if err != nil {
                return nil, err
            }

            // Now you can perform write operations
            actions, err := client.LoadActions()

            // Insert records
            txHash, err := actions.InsertRecords(ctx, types.InsertRecordsInput{
                DataProvider: config.DataProvider,
                StreamId:     config.StreamId,
                Records: [][]interface{}{
                    {"value1", "value2"},
                },
            })

            // Wait for transaction confirmation
            result, err := client.WaitTx(ctx, txHash, 2*time.Second)

            return &Result{TxHash: txHash}, nil
        },
        cre.ConsensusAggregationFromTags[*Result](),
    ).Await()
}
```

**When to use:**
- CRE workflows requiring write access
- Inserting records
- Deploying streams
- Any operation requiring transaction signing

**Equivalent to:**

```go
client, err := tnclient.NewClient(ctx, endpoint,
    tnclient.WithSigner(signer),
    tnclient.WithCRETransport(nodeRuntime, endpoint),
)
```

---

##### CRE Build Requirements

All CRE-specific code must include the build tag:

```go
//go:build wasip1

package main
```

**Compilation:**

```bash
# Build for CRE (WASM)
GOOS=wasip1 GOARCH=wasm go build -o workflow.wasm

# Regular build (excludes CRE code)
go build
```

---

##### CRE Limitations

- **Build tag required**: All files using CRE transport must have `//go:build wasip1`
- **No net/http**: Standard HTTP client not available in WASM
- **Context handling**: Use `context.WithTimeout` for all operations
- **Error handling**: Implement robust error handling for network operations

---

##### CRE Resources

📖 **Complete Guide:** [CRE Integration Guide](./CRE_INTEGRATION.md)

🎯 **Working Example:** [examples/truf-cre-demo/](../examples/truf-cre-demo/)

🔗 **CRE Documentation:** [docs.chain.link/cre](https://docs.chain.link/cre)

---

**GetKwilClient()** - Access underlying GatewayClient (HTTP transport only):

```go
// For advanced use cases requiring low-level control
if gwClient := client.GetKwilClient(); gwClient != nil {
	// Direct GatewayClient access for advanced scenarios
	result, err := gwClient.Call(ctx, "", "custom_action", args)
}
// Returns nil for non-HTTP transports
```

> **Important**: `GetKwilClient()` is provided for advanced use cases that require direct low-level access. For most scenarios, prefer using the high-level Client methods which are transport-agnostic.

### Transport Abstraction

The SDK uses a pluggable transport layer that allows different communication implementations:

- **HTTPTransport** (default): Standard `net/http` communication with the TRUF.NETWORK
- **Custom transports**: For specialized runtime environments (e.g., Chainlink CRE)
- **Mock transports**: For testing without network dependencies

This abstraction enables the SDK to work in various runtime environments while maintaining a consistent, high-level API. All Client methods work transparently with any transport implementation.

### Core Methods

#### Transaction Management

##### Understanding Async Operations and Race Conditions

**Critical**: All TN operations are asynchronous by default. They return success when transactions enter the mempool, NOT when they're executed on-chain.

> 📚 **Complete Example**: See [`examples/transaction-lifecycle-example/main.go`](../examples/transaction-lifecycle-example/main.go) for a comprehensive demonstration of safe transaction patterns with detailed explanations.

**Common Race Condition:**
```go
// ❌ DANGEROUS - Race condition
deployTx, _ := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
insertTx, _ := primitiveActions.InsertRecord(ctx, input) // Might fail!
```

**Two Solutions for Safe Operations:**

##### `WaitForTx` (Recommended for Critical Operations)

Waits for a transaction to be mined and confirmed. **Always check the `Result.Code`** to detect failures:

```go
import (
    kwiltypes "github.com/trufnetwork/kwil-db/core/types"
    // ... other imports
)

// Deploy stream
deployTx, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
if err != nil {
    return err
}

// Wait for deployment to complete
txResponse, err := tnClient.WaitForTx(ctx, deployTx, time.Second*5)
if err != nil {
    return err
} else if txResponse.Result.Code != uint32(kwiltypes.CodeOk) {
    return fmt.Errorf("deployment failed: %d", txResponse.Result.Code)
}

// Now safe to proceed
insertTx, err := primitiveActions.InsertRecord(ctx, input)
```

##### `WithSyncBroadcast` Option (For TxOpt-Enabled Operations)

For operations that support TxOpt parameters, use `WithSyncBroadcast(true)`:

```go
import (
    client "github.com/trufnetwork/kwil-db/core/client/types"
    // ... other imports
)

// Synchronous record insertion (waits for mining)
insertTx, err := primitiveActions.InsertRecord(ctx, input,
    client.WithSyncBroadcast(true))

// Synchronous taxonomy update (waits for mining)  
taxonomyTx, err := composedActions.InsertTaxonomy(ctx, taxonomy,
    client.WithSyncBroadcast(true))
```

**Note**: `DeployStream` and `DestroyStream` don't support TxOpt, so use `WaitForTx` with them.

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
	txRes, err := tnClient.WaitForTx(ctx, deployTx, time.Second * 5)
	if err != nil {
		return fmt.Errorf("deployment confirmation failed: %v", err)
	} else if txRes.Result.Code != uint32(kwiltypes.CodeOk) {
		return fmt.Errorf("deployment failed: %s", txRes.Result.Log)
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
    
    // Cache height information
    CacheHeight   *int64 `json:"cache_height"`    // Block height when data was cached
    
    // Query context (populated by SDK)
    StreamId      string `json:"stream_id"`       // Stream identifier
    DataProvider  string `json:"data_provider"`   // Data provider address
    From          *int64 `json:"from"`           // Query start time
    To            *int64 `json:"to"`             // Query end time
    FrozenAt      *int64 `json:"frozen_at"`      // Time-travel timestamp
    RowsServed    int    `json:"rows_served"`    // Number of rows returned
}
```

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
    if result.Metadata.CacheHeight != nil {
        fmt.Printf("Cache height: %d\n", *result.Metadata.CacheHeight)
    }
} else {
    fmt.Printf("Cache miss - data retrieved from database\n")
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
    if result.Metadata.CacheHeight != nil {
        fmt.Printf("Cache height: %d\n", *result.Metadata.CacheHeight)
    }
} else {
    fmt.Printf("○ Cache miss - data retrieved from source\n")
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

// 3. Analyze cache height for data consistency
if result.Metadata.CacheHeight != nil {
    fmt.Printf("Data cached at block height: %d\n", *result.Metadata.CacheHeight)
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

#### `DescribeTaxonomies` 🔍

```go
DescribeTaxonomies(ctx context.Context, params types.DescribeTaxonomiesParams) ([]types.TaxonomyItem, error)
```

Retrieves the current taxonomy configuration for a composed stream. This is the key method for discovering how composed streams aggregate their child streams.

**Parameters:**

- `ctx`: Operation context
- `params`: Taxonomy description parameters
  - `Stream`: Stream locator (identifies the composed stream)
  - `LatestVersion`: Flag to return only the most recent taxonomy version

**Returns:**

- List of `TaxonomyItem` objects containing:
  - `ChildStream`: Locator of each child stream
  - `Weight`: Weight/contribution of each child stream (0.0 to 1.0)
- Error if retrieval fails

**Example Usage:**
```go
// Get the latest taxonomy for a composed stream
params := types.DescribeTaxonomiesParams{
    Stream:        tnClient.OwnStreamLocator(composedStreamId),
    LatestVersion: true,
}
taxonomyItems, err := composedActions.DescribeTaxonomies(ctx, params)
if err != nil {
    log.Printf("Failed to describe taxonomies: %v", err)
    return
}

fmt.Printf("Taxonomy for stream %s:\n", composedStreamId.String())
for _, item := range taxonomyItems {
    fmt.Printf("  Child: %s (Weight: %.2f)\n", 
        item.ChildStream.StreamId.String(), item.Weight)
}
```

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

## Transaction Actions Interface

### Overview

The Transaction Actions Interface provides methods for querying transaction history, fees, and distributions from the TRUF.NETWORK ledger. This interface is essential for auditing, analytics, and tracking fee distributions across the network.

### Key Features

- Query detailed transaction information by hash
- List transactions by wallet with flexible filtering
- Track fee distributions to validators and proposers
- Pagination support for large result sets
- Filter by transaction type (paid, received, or both)

### Initialization

#### `LoadTransactionActions`

```go
func (c *Client) LoadTransactionActions() (*contractsapi.TransactionActions, error)
```

Initializes the transaction actions interface for querying transaction data.

**Returns:**
- `*TransactionActions`: Interface for transaction queries
- `error`: Error if initialization fails

**Example:**
```go
txActions, err := client.LoadTransactionActions()
if err != nil {
    log.Fatalf("Failed to load transaction actions: %v", err)
}
```

### Core Methods

#### `GetTransactionEvent`

```go
func (a *TransactionActions) GetTransactionEvent(
    ctx context.Context,
    input types.GetTransactionEventInput,
) (*types.TransactionEvent, error)
```

Retrieves detailed information about a specific transaction by its hash.

**Parameters:**
- `ctx`: Context for the operation
- `input`: Input containing:
  - `TxID`: Transaction hash (with or without `0x` prefix)

**Returns:**
- `*TransactionEvent`: Complete transaction details including:
  - `TxID`: Transaction hash (0x-prefixed)
  - `BlockHeight`: Block number where transaction was included
  - `Method`: Method name (e.g., "deployStream", "insertRecords")
  - `Caller`: Ethereum address of the caller (lowercase, 0x-prefixed)
  - `FeeAmount`: Total fee amount as string (handles large numbers)
  - `FeeRecipient`: Primary fee recipient address (nullable)
  - `Metadata`: Optional metadata JSON (nullable)
  - `FeeDistributions`: Array of fee distributions showing who received what amount
- `error`: Error if query fails or transaction not found

**Example:**
```go
txEvent, err := txActions.GetTransactionEvent(ctx, types.GetTransactionEventInput{
    TxID: "0xabcdef123456...",
})
if err != nil {
    log.Fatalf("Failed to get transaction: %v", err)
}

fmt.Printf("Method: %s\n", txEvent.Method)
fmt.Printf("Caller: %s\n", txEvent.Caller)
fmt.Printf("Fee: %s wei\n", txEvent.FeeAmount)
fmt.Printf("Block: %d\n", txEvent.BlockHeight)

// Check fee distributions
for _, dist := range txEvent.FeeDistributions {
    fmt.Printf("  → %s: %s wei\n", dist.Recipient, dist.Amount)
}
```

#### `ListTransactionFees`

```go
func (a *TransactionActions) ListTransactionFees(
    ctx context.Context,
    input types.ListTransactionFeesInput,
) ([]types.TransactionFeeEntry, error)
```

Lists transactions filtered by wallet address and mode, with pagination support.

**Parameters:**
- `ctx`: Context for the operation
- `input`: Input containing:
  - `Wallet`: Ethereum address to query (required)
  - `Mode`: Filter mode - one of:
    - `types.TransactionFeeModePaid`: Transactions where wallet paid fees
    - `types.TransactionFeeModeReceived`: Transactions where wallet received fee distributions
    - `types.TransactionFeeModeBoth`: All transactions involving the wallet
  - `Limit`: Maximum results to return (optional, default: 20, max: 1000)
  - `Offset`: Pagination offset (optional, default: 0)

**Returns:**
- `[]TransactionFeeEntry`: Array of transaction entries, each containing:
  - `TxID`: Transaction hash
  - `BlockHeight`: Block number
  - `Method`: Method name
  - `Caller`: Caller address
  - `TotalFee`: Total fee amount
  - `FeeRecipient`: Primary recipient (nullable)
  - `Metadata`: Optional metadata (nullable)
  - `DistributionSequence`: Distribution index (for multiple distributions)
  - `DistributionRecipient`: Recipient address for this distribution (nullable)
  - `DistributionAmount`: Amount for this distribution (nullable)
- `error`: Error if query fails

**Note:** This method returns one row per fee distribution. If a transaction has multiple distributions, it will appear multiple times with different `DistributionSequence` values.

**Example - List Fees Paid:**
```go
wallet := client.Address().Address()
limit := 10

entries, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
    Wallet: wallet,
    Mode:   types.TransactionFeeModePaid,
    Limit:  &limit,
})
if err != nil {
    log.Fatalf("Failed to list fees: %v", err)
}

for _, entry := range entries {
    fmt.Printf("%s: %s wei (block %d)\n",
        entry.Method, entry.TotalFee, entry.BlockHeight)
}
```

**Example - Pagination:**
```go
limit := 20
offset := 0

// Get first page
page1, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
    Wallet: wallet,
    Mode:   types.TransactionFeeModeBoth,
    Limit:  &limit,
    Offset: &offset,
})

// Get second page
offset = 20
page2, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
    Wallet: wallet,
    Mode:   types.TransactionFeeModeBoth,
    Limit:  &limit,
    Offset: &offset,
})
```

**Example - Fees Received:**
```go
// Track fee distributions received by a validator
entries, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
    Wallet: validatorAddress,
    Mode:   types.TransactionFeeModeReceived,
    Limit:  &limit,
})

totalReceived := big.NewInt(0)
for _, entry := range entries {
    if entry.DistributionAmount != nil {
        amount, _ := new(big.Int).SetString(*entry.DistributionAmount, 10)
        totalReceived.Add(totalReceived, amount)
    }
}
fmt.Printf("Total fees received: %s wei\n", totalReceived.String())
```

### Types

#### `TransactionEvent`

```go
type TransactionEvent struct {
    TxID             string
    BlockHeight      int64
    Method           string
    Caller           string
    FeeAmount        string
    FeeRecipient     *string
    Metadata         *string
    FeeDistributions []FeeDistribution
}
```

#### `FeeDistribution`

```go
type FeeDistribution struct {
    Recipient string `json:"recipient"`
    Amount    string `json:"amount"`
}
```

#### `TransactionFeeEntry`

```go
type TransactionFeeEntry struct {
    TxID                   string
    BlockHeight            int64
    Method                 string
    Caller                 string
    TotalFee               string
    FeeRecipient           *string
    Metadata               *string
    DistributionSequence   int
    DistributionRecipient  *string
    DistributionAmount     *string
}
```

#### `TransactionFeeMode`

```go
type TransactionFeeMode string

const (
    TransactionFeeModePaid     TransactionFeeMode = "paid"
    TransactionFeeModeReceived TransactionFeeMode = "received"
    TransactionFeeModeBoth     TransactionFeeMode = "both"
)
```

### Use Cases

#### Auditing: Track Monthly Spending

```go
// Calculate total fees paid by wallet in last 30 days
entries, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
    Wallet: myWallet,
    Mode:   types.TransactionFeeModePaid,
})

totalSpent := big.NewInt(0)
for _, entry := range entries {
    amount, _ := new(big.Int).SetString(entry.TotalFee, 10)
    totalSpent.Add(totalSpent, amount)
}

fmt.Printf("Total spent: %s wei\n", totalSpent.String())
```

#### Analytics: Transaction Patterns

```go
// Analyze transaction types and their costs
methodCounts := make(map[string]int)
methodCosts := make(map[string]*big.Int)

entries, err := txActions.ListTransactionFees(ctx, types.ListTransactionFeesInput{
    Wallet: myWallet,
    Mode:   types.TransactionFeeModePaid,
})

for _, entry := range entries {
    methodCounts[entry.Method]++

    if _, ok := methodCosts[entry.Method]; !ok {
        methodCosts[entry.Method] = big.NewInt(0)
    }

    amount, _ := new(big.Int).SetString(entry.TotalFee, 10)
    methodCosts[entry.Method].Add(methodCosts[entry.Method], amount)
}

for method, count := range methodCounts {
    avgCost := new(big.Int).Div(methodCosts[method], big.NewInt(int64(count)))
    fmt.Printf("%s: %d calls, avg cost %s wei\n", method, count, avgCost.String())
}
```

#### Fee Distribution Tracking

```go
// Monitor where your fees are going
txEvent, err := txActions.GetTransactionEvent(ctx, types.GetTransactionEventInput{
    TxID: deployTxHash,
})

fmt.Printf("Transaction: %s\n", txEvent.TxID)
fmt.Printf("Total Fee: %s wei\n", txEvent.FeeAmount)
fmt.Println("\nFee Distributions:")

for i, dist := range txEvent.FeeDistributions {
    fmt.Printf("  %d. %s: %s wei\n", i+1, dist.Recipient, dist.Amount)
}
```

### Best Practices

1. **Error Handling**: Always check for errors, especially for transaction not found
   ```go
   txEvent, err := txActions.GetTransactionEvent(ctx, input)
   if err != nil {
       if strings.Contains(err.Error(), "not found") {
           // Handle missing transaction
       }
       return err
   }
   ```

2. **Pagination**: Use reasonable page sizes to avoid overwhelming the API
   ```go
   limit := 100 // Good balance between API calls and memory
   ```

3. **Large Numbers**: Use `big.Int` for fee calculations to avoid overflow
   ```go
   amount, ok := new(big.Int).SetString(entry.TotalFee, 10)
   if !ok {
       return fmt.Errorf("invalid fee amount: %s", entry.TotalFee)
   }
   ```

4. **Context Timeout**: Set reasonable timeouts for large queries
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   ```

### Error Handling

Common errors and how to handle them:

```go
entries, err := txActions.ListTransactionFees(ctx, input)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "invalid wallet"):
        // Handle invalid wallet address
    case strings.Contains(err.Error(), "invalid mode"):
        // Handle invalid mode value
    case strings.Contains(err.Error(), "limit"):
        // Handle limit out of range
    default:
        // Handle other errors
    }
}
```

## Attestation Actions Interface

### Overview

The Attestation Actions Interface enables users to request cryptographically signed attestations of query results from TRUF.NETWORK validators. These signed attestations can be verified on-chain (e.g., in EVM smart contracts) to trustlessly prove that specific data existed at a particular block height.

### Key Features

- **Cryptographic Verification**: Signed by network validators using secp256k1
- **Tamper-Proof**: Immutable attestations linked to specific block heights  
- **EVM-Compatible**: Can be verified in Solidity smart contracts
- **Payload Parsing**: Decode attestation data including timestamps and values
- **Signature Recovery**: Extract validator addresses from signatures

### Use Cases

- **DeFi Protocols**: Verify off-chain data on-chain (oracle alternative)
- **Prediction Markets**: Settle bets with cryptographically verified results
- **Insurance**: Trigger payouts based on attested data
- **Auditing**: Prove data provenance and integrity
- **Cross-Chain Bridges**: Verify state across networks

### Initialization

#### `LoadAttestationActions`

Creates an attestation action handler for requesting and retrieving signed attestations.

**Signature:**
```go
func (c *TNClient) LoadAttestationActions() (types.IAttestationAction, error)
```

**Returns:**
- `types.IAttestationAction`: Attestation action handler
- `error`: Error if initialization fails

**Example:**
```go
attestationActions, err := tnClient.LoadAttestationActions()
if err != nil {
    log.Fatalf("Failed to load attestation actions: %v", err)
}
```

---

### Core Methods

The following methods are part of the `types.IAttestationAction` interface returned by `LoadAttestationActions()`. Call these methods on the attestation action handler.

#### `RequestAttestation`

Requests a signed attestation for a specific query. The validator will execute the query at the current block height and sign the results.

**Signature:**
```go
func RequestAttestation(ctx context.Context, input types.RequestAttestationInput) (*types.RequestAttestationResult, error)
```

**Parameters:**

`types.RequestAttestationInput`:
- `DataProvider` (string): Data provider address (0x-prefixed, 42 chars)
- `StreamID` (string): Stream identifier (32 characters)  
- `ActionName` (string): Action to attest (e.g., "get_record")
- `Args` ([]any): Action arguments (will be canonically encoded)
- `EncryptSig` (bool): Must be `false` (encryption not supported in MVP)
- `MaxFee` (string): Maximum fee willing to pay in wei (NUMERIC(78,0) as string)

**Returns:**

`types.RequestAttestationResult`:
- `RequestTxID` (string): Transaction ID for this attestation request

**Example:**
```go
// Request attestation for AI Index data from last 7 days
now := time.Now()
weekAgo := now.AddDate(0, 0, -7)

result, err := attestationActions.RequestAttestation(ctx, types.RequestAttestationInput{
    DataProvider: "0x4710a8d8f0d845da110086812a32de6d90d7ff5c",
    StreamID:     "stai0000000000000000000000000000",
    ActionName:   "get_record",
    Args: []any{
        "0x4710a8d8f0d845da110086812a32de6d90d7ff5c",
        "stai0000000000000000000000000000",
        int64(weekAgo.Unix()),
        int64(now.Unix()),
        nil,   // frozen_at (optional)
        false, // use_cache (forced to false for attestations)
    },
    EncryptSig: false,
    MaxFee:     "100000000000000000000", // 100 TRUF
})

if err != nil {
    log.Fatalf("Failed to request attestation: %v", err)
}

fmt.Printf("Request TX ID: %s\n", result.RequestTxID)
```

**Notes:**
- Attestation requests require sufficient TRUF balance for fees
- The validator signs asynchronously (typically 1-2 blocks)
- Use `GetSignedAttestation()` to retrieve the signed payload

---

#### `GetSignedAttestation`

Retrieves a complete signed attestation payload for a previous attestation request.

**Signature:**
```go
func GetSignedAttestation(ctx context.Context, input types.GetSignedAttestationInput) (*types.SignedAttestationResult, error)
```

**Parameters:**

`types.GetSignedAttestationInput`:
- `RequestTxID` (string): Transaction ID from `RequestAttestation()`

**Returns:**

`types.SignedAttestationResult`:
- `Payload` ([]byte): Canonical payload + 65-byte secp256k1 signature

**Payload Format:**

The payload consists of:
1. **Canonical Fields** (variable length):
   - Version (1 byte)
   - Algorithm (1 byte, 0 = secp256k1)
   - Block Height (8 bytes, big-endian uint64)
   - Data Provider (length-prefixed, big-endian uint32 + bytes)
   - Stream ID (length-prefixed, big-endian uint32 + UTF-8)
   - Action ID (2 bytes, big-endian uint16)
   - Arguments (length-prefixed, big-endian uint32 + canonical encoding)
   - Result (length-prefixed, big-endian uint32 + ABI-encoded data)

2. **Signature** (last 65 bytes):
   - R component (32 bytes)
   - S component (32 bytes)
   - V recovery ID (1 byte, typically 27 or 28)

**Example:**
```go
// Poll for signed attestation (max 30 seconds)
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

ticker := time.NewTicker(2 * time.Second)
defer ticker.Stop()

var signedResult *types.SignedAttestationResult
for {
    select {
    case <-ctx.Done():
        log.Println("Timeout waiting for signature")
        goto afterPoll
    case <-ticker.C:
        signed, err := attestationActions.GetSignedAttestation(ctx, types.GetSignedAttestationInput{
            RequestTxID: result.RequestTxID,
        })
        if err == nil && signed != nil && len(signed.Payload) > 0 {
            signedResult = signed
            goto afterPoll
        }
    }
}

afterPoll:
if signedResult != nil {
    fmt.Printf("Payload size: %d bytes\n", len(signedResult.Payload))
}
```

---

#### `ListAttestations`

Returns metadata for attestations, optionally filtered by requester address.

**Signature:**
```go
func ListAttestations(ctx context.Context, input types.ListAttestationsInput) ([]types.AttestationMetadata, error)
```

**Parameters:**

`types.ListAttestationsInput`:
- `Requester` ([]byte, optional): Filter by requester address (20 bytes)
- `Limit` (*int, optional): Max results (default/max 5000)
- `Offset` (*int, optional): Pagination offset (default 0)
- `OrderBy` (*string, optional): Sort order (see below)

**Valid `OrderBy` values:**
- `"created_height ASC"` / `"created_height DESC"`
- `"signed_height ASC"` / `"signed_height DESC"`

**Returns:**

Array of `types.AttestationMetadata`:
- `RequestTxID` (string): Transaction ID of the attestation request
- `AttestationHash` ([]byte): Hash of the attestation
- `Requester` ([]byte): Address that requested the attestation (20 bytes)
- `CreatedHeight` (int64): Block height when requested
- `SignedHeight` (*int64): Block height when signed (nil if not yet signed)
- `EncryptSig` (bool): Whether signature is encrypted

**Example:**
```go
// List recent attestations for current wallet
myAddress := tnClient.Address()
addressBytes, _ := hex.DecodeString(myAddress.Address()[2:])

limit := 10
attestations, err := attestationActions.ListAttestations(ctx, types.ListAttestationsInput{
    Requester: addressBytes,
    Limit:     &limit,
    OrderBy:   strPtr("created_height desc"),
})

if err != nil {
    log.Fatalf("Failed to list attestations: %v", err)
}

fmt.Printf("Found %d attestations\n", len(attestations))
for i, att := range attestations {
    status := "unsigned"
    if att.SignedHeight != nil {
        status = fmt.Sprintf("signed at height %d", *att.SignedHeight)
    }
    fmt.Printf("%d. TX: %s, Status: %s\n", i+1, att.RequestTxID, status)
}
```

---

### Payload Parsing

#### `ParseAttestationPayload`

Parses a canonical attestation payload (without signature) into structured data.

**Package:** `github.com/trufnetwork/sdk-go/core/contractsapi`

**Signature:**
```go
func ParseAttestationPayload(payload []byte) (*types.ParsedAttestationPayload, error)
```

**Parameters:**
- `payload` ([]byte): Canonical payload **without** the 65-byte signature

**Returns:**

`types.ParsedAttestationPayload`:
- `Version` (uint8): Payload format version
- `Algorithm` (uint8): Signature algorithm (0 = secp256k1)
- `BlockHeight` (uint64): Block height when attested
- `DataProvider` (string): Data provider address (0x-prefixed hex)
- `StreamID` (string): Stream identifier
- `ActionID` (uint16): Action identifier
- `Arguments` ([]any): Decoded action arguments
- `Result` ([]types.DecodedRow): Decoded query results

`types.DecodedRow`:
- `Values` ([]any): Array of decoded column values
  - For attestation results: `Values[0]` is timestamp (string), `Values[1]` is value (string)

**Example:**
```go
import (
    "crypto/sha256"
    "github.com/trufnetwork/kwil-db/core/crypto"
    "github.com/trufnetwork/sdk-go/core/contractsapi"
)

// Split payload into canonical part and signature
signedPayload := signedResult.Payload
canonicalPayload := signedPayload[:len(signedPayload)-65]
signature := signedPayload[len(signedPayload)-65:]

// Parse the canonical payload
parsed, err := contractsapi.ParseAttestationPayload(canonicalPayload)
if err != nil {
    log.Fatalf("Failed to parse payload: %v", err)
}

// Access parsed fields
fmt.Printf("Version: %d\n", parsed.Version)
fmt.Printf("Block Height: %d\n", parsed.BlockHeight)
fmt.Printf("Data Provider: %s\n", parsed.DataProvider)
fmt.Printf("Stream ID: %s\n", parsed.StreamID)

// Access query results
fmt.Printf("Found %d rows:\n", len(parsed.Result))
for i, row := range parsed.Result {
    timestamp := row.Values[0]  // Unix timestamp as string
    value := row.Values[1]      // 18-decimal value as string
    fmt.Printf("Row %d: Timestamp=%v, Value=%v\n", i+1, timestamp, value)
}
```

---

### Signature Verification

To verify the attestation signature and recover the validator's address:

```go
import (
    "crypto/sha256"
    "github.com/trufnetwork/kwil-db/core/crypto"
)

// Extract canonical payload and signature
canonicalPayload := signedPayload[:len(signedPayload)-65]
signature := signedPayload[len(signedPayload)-65:]

// Hash the canonical payload with SHA256
hash := sha256.Sum256(canonicalPayload)

// Adjust signature format for recovery
// Attestation signatures use Ethereum format (V=27/28)
// kwil-db expects raw format (V=0-3)
adjustedSig := make([]byte, 65)
copy(adjustedSig, signature)
if signature[64] >= 27 {
    adjustedSig[64] = signature[64] - 27
}

// Recover validator public key
pubKey, err := crypto.RecoverSecp256k1KeyFromSigHash(hash[:], adjustedSig)
if err != nil {
    log.Fatalf("Failed to recover public key: %v", err)
}

// Derive Ethereum address
validatorAddr := crypto.EthereumAddressFromPubKey(pubKey)
fmt.Printf("Validator Address: 0x%x\n", validatorAddr)
```

**Important Notes:**
- Attestation signatures use **Ethereum format** with V=27/28
- kwil-db's `RecoverSecp256k1KeyFromSigHash` expects V=0-3 (raw format)
- You must subtract 27 from V before calling the recovery function
- The recovered address identifies which validator signed the attestation

---

### Types

#### `ParsedAttestationPayload`

Decoded attestation payload structure.

```go
type ParsedAttestationPayload struct {
    Version      uint8        `json:"version"`
    Algorithm    uint8        `json:"algorithm"`     // 0 = secp256k1
    BlockHeight  uint64       `json:"blockHeight"`
    DataProvider string       `json:"dataProvider"`  // 0x-prefixed hex
    StreamID     string       `json:"streamId"`
    ActionID     uint16       `json:"actionId"`
    Arguments    []any        `json:"arguments"`
    Result       []DecodedRow `json:"result"`
}
```

#### `DecodedRow`

Represents a decoded row from attestation query results.

```go
type DecodedRow struct {
    Values []any `json:"values"`
}
```

**For attestation results:**
- `Values[0]`: Unix timestamp as string (e.g., "1704067200")
- `Values[1]`: 18-decimal fixed-point value as string (e.g., "77.051806494788211665")

---

### Result Encoding Format

Attestation results use **ABI encoding** (Ethereum format):

```solidity
abi.encode(uint256[] timestamps, int256[] values)
```

**Details:**
- `timestamps`: Array of Unix timestamps as uint256
- `values`: Array of 18-decimal fixed-point integers as int256
- Negative values are properly handled (two's complement)

**Example decoded result:**
```go
// Raw ABI bytes → Decoded rows
[
    {Values: ["1704067200", "77.051806494788211665"]},
    {Values: ["1704153600", "80.0"]},
    {Values: ["1704240000", "75.5"]},
]
```

---

### Complete Example

See [`examples/attestation_example/main.go`](../examples/attestation_example/main.go) for a complete working example demonstrating:

1. **Request Attestation**: Submit attestation request for AI Index data
2. **Poll for Signature**: Wait for validator to sign (1-2 blocks)
3. **Retrieve Payload**: Get the complete signed attestation
4. **Verify Signature**: Recover validator address from signature
5. **Parse Payload**: Decode attestation fields and query results
6. **Display Results**: Show all attested datapoints with full precision

**Key Code Snippets:**

```go
// 1. Request attestation
result, err := attestationActions.RequestAttestation(ctx, types.RequestAttestationInput{
    DataProvider: "0x4710a8d8f0d845da110086812a32de6d90d7ff5c",
    StreamID:     "stai0000000000000000000000000000",
    ActionName:   "get_record",
    Args:         args,
    EncryptSig:   false,
    MaxFee:       "100000000000000000000",
})

// 2. Wait for signing (poll with timeout)
signed, err := attestationActions.GetSignedAttestation(ctx, types.GetSignedAttestationInput{
    RequestTxID: result.RequestTxID,
})

// 3. Split payload
canonicalPayload := signed.Payload[:len(signed.Payload)-65]
signature := signed.Payload[len(signed.Payload)-65:]

// 4. Verify signature
hash := sha256.Sum256(canonicalPayload)
adjustedSig := make([]byte, 65)
copy(adjustedSig, signature)
if signature[64] >= 27 {
    adjustedSig[64] = signature[64] - 27
}
pubKey, _ := crypto.RecoverSecp256k1KeyFromSigHash(hash[:], adjustedSig)
validatorAddr := crypto.EthereumAddressFromPubKey(pubKey)

// 5. Parse payload
parsed, _ := contractsapi.ParseAttestationPayload(canonicalPayload)

// 6. Display results
for i, row := range parsed.Result {
    fmt.Printf("Row %d: Timestamp=%v, Value=%v\n", 
        i+1, row.Values[0], row.Values[1])
}
```

---

### EVM Integration

To verify attestations in Solidity smart contracts:

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract AttestationVerifier {
    address public validatorAddress;
    
    function verifyAttestation(
        bytes memory canonicalPayload,
        bytes memory signature
    ) public view returns (bool) {
        require(signature.length == 65, "Invalid signature length");

        // Hash the canonical payload
        bytes32 digest = sha256(canonicalPayload);

        // Extract r, s, v from signature using assembly
        bytes32 r;
        bytes32 s;
        uint8 v;
        assembly {
            r := mload(add(signature, 32))
            s := mload(add(signature, 64))
            v := byte(0, mload(add(signature, 96)))
        }

        // Recover signer address
        address signer = ecrecover(digest, v, r, s);

        // Verify it matches the known validator
        return signer == validatorAddress;
    }
    
    function parseValue(bytes memory payload) public pure returns (uint256) {
        // Parse and extract specific fields from canonical payload
        // Implementation depends on your use case
    }
}
```

**Usage Pattern:**
1. User requests attestation off-chain
2. Validator signs the query results
3. User submits signed payload to smart contract
4. Contract verifies signature using `ecrecover`
5. Contract parses payload to extract attested data
6. Contract executes logic based on verified data

---

### Best Practices

1. **Always Verify Signatures**
   - Never trust attestation payloads without verifying the validator signature
   - Check that the recovered address matches a known validator

2. **Handle Async Signing**
   - Poll with timeout (typically 30 seconds is sufficient)
   - Check for errors during polling (attestation may fail)

3. **Fee Management**
   - Ensure sufficient TRUF balance before requesting attestations
   - Set reasonable `MaxFee` values to avoid overpaying

4. **Parse Results Carefully**
   - Timestamps are Unix seconds as strings
   - Values are 18-decimal fixed-point as strings
   - Convert to appropriate types for your use case

5. **Store Request IDs**
   - Keep track of `RequestTxID` for later retrieval
   - Use `ListAttestations()` to view attestation history

6. **Test Locally First**
   - Use local node for development
   - Test with mainnet only when ready

---

### Error Handling

Common errors and how to handle them:

```go
// Requesting attestation
result, err := attestationActions.RequestAttestation(ctx, input)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "Insufficient balance"):
        // User needs more TRUF tokens
        log.Println("Please fund your wallet with TRUF tokens")
    case strings.Contains(err.Error(), "invalid"):
        // Input validation failed
        log.Println("Check input parameters")
    default:
        log.Printf("Attestation request failed: %v", err)
    }
}

// Retrieving signed attestation
signed, err := attestationActions.GetSignedAttestation(ctx, input)
if err != nil || len(signed.Payload) < 66 {
    // Attestation not ready or invalid
    log.Println("Attestation not yet signed, try again later")
}

// Parsing payload
parsed, err := contractsapi.ParseAttestationPayload(canonicalPayload)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "too short"):
        // Payload truncated or invalid
        log.Println("Invalid payload format")
    case strings.Contains(err.Error(), "version"):
        // Unsupported payload version
        log.Println("Unsupported attestation version")
    default:
        log.Printf("Parse error: %v", err)
    }
}

// Signature verification
pubKey, err := crypto.RecoverSecp256k1KeyFromSigHash(hash[:], adjustedSig)
if err != nil {
    // Invalid signature or tampering detected
    log.Println("Signature verification failed - payload may be tampered")
}
```

---

### Performance Considerations

- **Attestation Latency**: Typically 1-2 blocks (2-4 seconds) for signing
- **Payload Size**: Varies with result data (typically 1KB-100KB)
- **Fee Costs**: Depends on query complexity and data size
- **Polling Frequency**: Recommended 2-second intervals to balance latency and API load

---

### Security Considerations

1. **Signature Verification**: Always verify signatures before trusting attestation data
2. **Replay Protection**: Check block height to prevent replay attacks
3. **Validator Trust**: Only accept attestations from known validator addresses
4. **Payload Integrity**: Hash payload before verification; detect tampering
5. **Fee Limits**: Set appropriate `MaxFee` to prevent unexpected charges
