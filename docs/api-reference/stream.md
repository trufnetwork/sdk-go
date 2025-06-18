# Stream Interface

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
Δindex(t) = ( index(t) − index(t − Δ) ) / index(t − Δ) × 100
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
- `locator`: The locator of the composed stream.

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