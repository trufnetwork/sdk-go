# Client Interface

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