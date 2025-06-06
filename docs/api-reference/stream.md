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

Retrieves records from the stream based on the input criteria.

**Parameters:**
- `ctx`: The context for the operation.
- `input`: The input criteria for retrieving records.

**Returns:**
- `[]types.StreamRecord`: The retrieved records.
- `error`: An error if the retrieval fails.

### `GetIndex`

```go
GetIndex(ctx context.Context, input types.GetIndexInput) ([]types.StreamIndex, error)
```

Retrieves the index of the stream based on the input criteria.

**Parameters:**
- `ctx`: The context for the operation.
- `input`: The input criteria for retrieving the indices.

**Returns:**
- `[]types.StreamIndex`: The retrieved indices.
- `error`: An error if the retrieval fails.

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

## Best Practices

1. **Always handle errors**
2. **Use context with appropriate timeouts**
3. **Validate wallet addresses**
4. **Log permission changes**
5. **Implement retry mechanisms**

## Considerations

- Visibility changes are blockchain transactions