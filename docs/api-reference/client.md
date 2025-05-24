# Client Interface

The `Client` interface is the primary entry point for interacting with the TN. It provides methods for managing streams, handling transactions, and interacting with the underlying Kwil client.

## Methods

### `WaitForTx`

```go
WaitForTx(ctx context.Context, txHash transactions.TxHash, interval time.Duration) (*transactions.TcTxQueryResponse, error)
```

Waits for a transaction to be mined and returns the transaction query response.

**Parameters:**
- `ctx`: The context for the operation.
- `txHash`: The transaction hash.
- `interval`: The polling interval for checking the transaction status.

**Returns:**
- `*transactions.TcTxQueryResponse`: The transaction query response.
- `error`: An error if the transaction fails or an issue occurs.

### `DeployStream`

```go
DeployStream(ctx context.Context, streamId util.StreamId, streamType types.StreamType) (transactions.TxHash, error)
```

Deploys a new stream of the specified type.

**Parameters:**
- `ctx`: The context for the operation.
- `streamId`: The unique identifier for the stream.
- `streamType`: The type of the stream (`Primitive`, `Composed`).

**Returns:**
- `transactions.TxHash`: The transaction hash for the deployment.
- `error`: An error if the deployment fails.

### `DestroyStream`

```go
DestroyStream(ctx context.Context, streamId util.StreamId) (transactions.TxHash, error)
```

Destroys an existing stream.

**Parameters:**
- `ctx`: The context for the operation.
- `streamId`: The unique identifier for the stream.

**Returns:**
- `transactions.TxHash`: The transaction hash for the destruction.
- `error`: An error if the destruction fails.

### `LoadPrimitiveStream`

```go
LoadPrimitiveStream(stream StreamLocator) (IPrimitiveStream, error)
```

Loads an existing primitive stream.

**Parameters:**
- `stream`: The locator for the stream.

**Returns:**
- `IPrimitiveStream`: The primitive stream interface.
- `error`: An error if the stream fails to load.

### `LoadComposedStream`

```go
LoadComposedStream(stream StreamLocator) (IComposedStream, error)
```

Loads an existing composed stream.

**Parameters:**
- `stream`: The locator for the stream.

**Returns:**
- `IComposedStream`: The composed stream interface.
- `error`: An error if the stream fails to load.

### `OwnStreamLocator`

```go
OwnStreamLocator(streamId util.StreamId) StreamLocator
```

Generates a stream locator for the given stream ID, using the current signer as the owner of the stream.

**Parameters:**
- `streamId`: The unique identifier for the stream.

**Returns:**
- `StreamLocator`: The stream locator.

### `Address`

```go
Address() util.EthereumAddress
```

Gets the Ethereum address of the client.

**Returns:**
- `util.EthereumAddress`: The Ethereum address.

### `NewClient`

```go
tnclient.NewClient(ctx, "https://gateway.infra.truf.network", opts)
```