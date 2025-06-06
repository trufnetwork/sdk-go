# Complex Stream Creation Example

## Overview

This example demonstrates the complete workflow of creating and managing streams in the TRUF.NETWORK (TN) SDK. It showcases:

- Deploying primitive streams
- Inserting records into primitive streams
- Creating a composed stream
- Setting up stream taxonomy
- Retrieving composed stream records
- Stream deletion and resource cleanup

## Scenario

The example creates a composite market index by combining two primitive streams:
1. Market Sentiment Stream
2. Economic Indicator Stream

The composed stream aggregates these streams with different weights to create a comprehensive market index.

## Key Concepts Demonstrated

- Stream ID generation
- Stream deployment (primitive and composed)
- Record insertion
- Taxonomy setup
- Record retrieval
- Stream deletion and resource management

## Stream Deletion

The example includes a deferred stream deletion mechanism to ensure proper resource cleanup:

```go
defer func() {
    // Destroy streams in reverse order of creation
    streamIds := []util.StreamId{
        composedStreamId,
        primitiveStreamId1,
        primitiveStreamId2,
    }

    for _, streamId := range streamIds {
        destroyTx, err := tnClient.DestroyStream(ctx, streamId)
        if err != nil {
            log.Printf("Failed to destroy stream %s: %v", streamId, err)
            continue
        }

        // Wait for the destroy transaction to be mined
        _, err = tnClient.WaitForTx(ctx, destroyTx, time.Second*5)
        if err != nil {
            log.Printf("Error waiting for destroy transaction: %v", err)
        }
    }
}()
```

### Best Practices for Stream Deletion

- Always destroy streams when they are no longer needed
- Destroy composed streams before their child primitive streams
- Handle potential errors during stream deletion
- Wait for transaction confirmation after initiating stream destruction

## Prerequisites

- Go 1.20 or later
- TRUF.NETWORK SDK
- Access to a local TN node or mainnet endpoint

## Running the Example

1. Replace `"your-private-key"` with your actual private key
2. Adjust the `endpoint` to match your TN node (local or mainnet)
3. Run the example:
   ```bash
   go run main.go
   ```

## Important Notes

- Ensure you have the necessary permissions and credentials
- The example uses a local node by default; modify the endpoint as needed
- Always handle errors and transaction confirmations in production code
- Properly manage stream lifecycle, including deletion

## Learning Outcomes

After running this example, you'll understand:
- How to create different types of streams
- How to insert data into streams
- How to compose streams using taxonomies
- How to retrieve and process stream data
- How to properly delete and clean up streams

## Customization

Feel free to modify the example to:
- Use different stream types
- Adjust weights in the taxonomy
- Add more complex data processing logic
- Implement custom stream deletion strategies