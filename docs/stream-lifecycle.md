# Stream Lifecycle

Understanding streams' lifecycle and associated transactions is crucial for effective interaction with the TRUF.NETWORK (TN). This document outlines the key stages in a stream's lifecycle and provides best practices for managing transaction dependencies.

## Transaction Lifecycle

TN operations rely on blockchain transactions. Each transaction must be included in a block and mined to be considered valid and stored, which introduces a delay between initiating an action and its completion.

### Key Points:

1. **Block Time**: In the current TN implementation, blocks are mined approximately every 6 seconds.
2. **Transaction Confirmation**: Always wait for transaction confirmation before performing dependent actions.
3. **Nonce Management**: Transactions from a single wallet must be processed in order. To prevent nonce errors, avoid parallelizing operations from a single wallet.

## Stream Lifecycle Stages

### 1. Deployment

The first step in creating a stream is deploying the contract.

```go
deployTxHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
if err != nil {
    // Handle error
}
// Wait for transaction to be mined
txRes, err := tnClient.WaitForTx(ctx, deployTxHash, time.Second)
if err != nil {
    // Handle error
} else if txRes.Result.Code != uint32(kwiltypes.CodeOk) {
    // Handle transaction failure: txRes.Result.Log contains error message
}
switch transactions.TxCode(txRes.TxResult.Code) {
case transactions.CodeOk:
    // Deployment successful
default:
    // Handle other transaction results
}
```

### 2. Configuration and Data Operations

Configuration and data operations can be executed in the same block, so there's no need to wait between them.

```go
// Set visibility
txHash1, err := stream.SetReadVisibility(ctx, util.PrivateVisibility)
if err != nil {
    // Handle error
}

// Insert records
dataProvider := tnClient.Address()
txHash2, err := stream.InsertRecords(ctx, []types.InsertRecordInput{
    {
        DataProvider: dataProvider.Address(),
        StreamId:     streamId.String(),
        EventTime:    int(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).Unix()),
        Value:        1.0,
    },
})
if err != nil {
    // Handle error
}

// Wait for transactions to be mined
// You can wait for both transactions concurrently if needed
```

### 3. Destruction (If Needed)

Streams can be destroyed when no longer needed.

```go
destroyResult, err := tnClient.DestroyStream(ctx, streamId)
if err != nil {
    // Handle error
}
// Wait for transaction to be mined
```

## Best Practices

1. **Wait for Confirmation**: Before proceeding to dependent actions, always wait for transaction confirmation.
2. **Error Handling**: Implement robust handling to manage transaction failures or network issues.
3. **Batch Operations**: When deploying multiple streams, you can initiate deployments in parallel without waiting for each to complete. This can speed up batch deployments.

    ```go
    deployTxHashes := make([]string, len(streamIds))
    for i, id := range streamIds {
        txHash, err := tnClient.DeployStream(ctx, id, types.StreamTypePrimitive)
        if err != nil {
            // Handle error
        }
        deployTxHashes[i] = txHash
    }
    
    // Wait for all deployments to complete
    for _, txHash := range deployTxHashes {
        // Use client.WaitForTx here
    }
    ```

4. **Transaction Ordering**: Maintain correct order for transactions from a single wallet. Use a single goroutine or implement a queue system for operations from the same wallet.

## Common Pitfalls

1. **Dependent Actions**: Attempting to initialize a stream before its deployment transaction is confirmed will result in an error.
2. **Nonce Errors**: Parallelizing operations from a single wallet can lead to nonce errors. Maintain sequential execution for transactions from the same wallet.

## Additional Resources

Please refer to the test files in the SDK repository for more detailed examples and usage patterns. These tests provide comprehensive examples of various stream operations and error-handling scenarios.
