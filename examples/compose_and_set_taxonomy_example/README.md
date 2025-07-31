# Compose and Set Taxonomy Example

## Overview

This example demonstrates how to use the `DeployComposedStreamWithTaxonomy` method from the TRUF.NETWORK (TN) SDK. This method combines stream deployment and taxonomy setup into a single operation, streamlining the process of creating composed streams.

## Key Features Demonstrated

- Using `DeployComposedStreamWithTaxonomy` for efficient stream creation
- Setting up taxonomy with predefined stream IDs and weights
- Verifying deployment by retrieving taxonomy information
- Attempting to read composed stream data
- Proper resource cleanup

## Scenario

The example creates a market composite index that aggregates data from three existing streams in the network:

1. **Stream 1**: `st96047782bbd4f43be169e58f7d051e` (Weight: 0.33)
   - Data Provider: `0x7f573e177ee7ec50eb5dee59478285054e4e74e7`

2. **Stream 2**: `st42ec7fe9e03d7e1369b8161adbde37` (Weight: 0.33)
   - Data Provider: `0xf3c816dc0576ec011e5d28367d7fa8c17bb8c6b7`

3. **Stream 3**: `stefa4eff1d1ea28db2b8c41af81e8ef` (Weight: 0.34)
   - Data Provider: `0xf3c816dc0576ec011e5d28367d7fa8c17bb8c6b7`

## Key Method: DeployComposedStreamWithTaxonomy

This method performs the following operations automatically:

1. Deploys a new composed stream
2. Waits for deployment confirmation
3. Loads composed actions
4. Sets the taxonomy for the stream
5. Waits for taxonomy transaction confirmation

```go
err = tnClient.DeployComposedStreamWithTaxonomy(ctx, composedStreamId, taxonomy)
```

## Taxonomy Structure

The taxonomy defines how child streams contribute to the composed stream:

```go
taxonomy := types.Taxonomy{
    ParentStream: tnClient.OwnStreamLocator(composedStreamId),
    TaxonomyItems: []types.TaxonomyItem{
        {
            ChildStream: types.StreamLocator{
                StreamId:     util.StreamIdFromString("st96047782bbd4f43be169e58f7d051e"),
                DataProvider: util.EthereumAddressFromString("0x7f573e177ee7ec50eb5dee59478285054e4e74e7"),
            },
            Weight: 0.33,
        },
        // ... more items
    },
}
```

## Prerequisites

- Go 1.24.1 or later
- TRUF.NETWORK SDK
- Access to a TN node (local or mainnet)
- Valid private key for transaction signing

## Running the Example

1. Replace `"your-private-key"` with your actual private key
2. Adjust the `endpoint` to match your TN node:
   - Local: `"http://localhost:8484"`
   - Mainnet: Use appropriate mainnet URL
3. Run the example:
   ```bash
   go mod tidy
   go run main.go
   ```

## Expected Output

The example will:
1. Deploy the composed stream with taxonomy
2. Display deployment confirmation
3. Retrieve and show the taxonomy details
4. Attempt to read composed data (may fail if child streams are empty)
5. Clean up by destroying the created stream

## Important Notes

- **Existing Streams**: This example references existing streams in the network. Ensure these streams exist and are accessible.
- **Permissions**: You need appropriate permissions to read from the referenced child streams.
- **Data Availability**: The data retrieval section may fail if the child streams don't contain data in the expected date range.
- **Resource Cleanup**: The example automatically destroys the created stream upon completion.

## Error Handling

The example includes comprehensive error handling for:
- Stream deployment failures
- Taxonomy setup errors
- Data retrieval issues (treated as warnings)
- Stream cleanup problems

## Comparison with Manual Approach

Instead of manually calling:
1. `DeployStream()`
2. `WaitForTx()`
3. `LoadComposedActions()`
4. `InsertTaxonomy()`
5. `WaitForTx()`

This example uses the convenient `DeployComposedStreamWithTaxonomy()` method that handles all these steps internally.

## Learning Outcomes

After running this example, you'll understand:
- How to use the streamlined deployment method
- How to structure taxonomy data for composed streams
- How to work with existing streams in the network
- Best practices for error handling and resource cleanup
- How to verify successful deployment and taxonomy setup