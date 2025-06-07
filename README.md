# TRUF.NETWORK (TN) SDK

The TRUF.NETWORK SDK provides developers with tools to interact with the TRUF.NETWORK, a decentralized platform for publishing, composing, and consuming economic data streams.

## Support

If you need help, don't hesitate to [open an issue](https://github.com/trufnetwork/sdk-go/issues).

## Quick Start

### Prerequisites

- Go 1.20 or later
- Docker (for local node setup)
- A local TN node (optional, for local testing)

### Installation

```bash
go get github.com/trufnetwork/sdk-go
```

## Local Node Testing

### Setting Up a Local Node

1. **Prerequisites:**
   - Docker
   - Docker Compose
   - Git

2. **Clone the TN Node Repository:**
   ```bash
   git clone https://github.com/trufnetwork/node.git
   cd node
   ```

3. **Start the Local Node:**
   ```bash
   # Start the node in development mode
   task single:start
   ```

   **Note:** Setting up a local node as described above will initialize an empty database. This setup is primarily for testing the technology or development purposes. If you are a node operator and wish to sync with the TRUF.NETWORK to access real data, please follow the [Node Operator Guide](https://github.com/trufnetwork/node/blob/main/docs/node-operator-guide.md) for instructions on connecting to the network and syncing data.

4. **Verify Node Synchronization**

When running a local node, it's crucial to ensure it's fully synchronized before querying data. If you are running as a node operator or are connected to the network, use the following command to check node status:

```bash
kwild admin status
```

**Note:** If you are running a setup without operating as a node operator or connecting to the network, this command is not needed.

### Querying Streams from Local Node

Here's an example of querying the AI Index stream from a local node:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

func main() {
	ctx := context.Background()

	// Set up local node connection
	pk, err := crypto.Secp256k1PrivateKeyFromHex("your-private-key")
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}
	signer := &auth.EthPersonalSigner{Key: *pk}

	// Connect to local node
	tnClient, err := tnclient.NewClient(
		ctx,
		"http://localhost:8484",  // Local node endpoint
		tnclient.WithSigner(signer),
	)
	if err != nil {
		log.Fatalf("Failed to create TN client: %v", err)
	}

	// AI Index stream details
	dataProvider := "0x4710a8d8f0d845da110086812a32de6d90d7ff5c"
	streamId := "stai0000000000000000000000000000"

	// Retrieve records from the last week
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	fromTime := int(weekAgo.Unix())
	toTime := int(now.Unix())

	primitiveActions, err := tnClient.LoadComposedActions()
	if err != nil {
		log.Fatalf("Failed to load primitive actions: %v", err)
	}

	records, err := primitiveActions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: dataProvider,
		StreamId:     streamId,
		From:         &fromTime,
		To:           &toTime,
	})
	if err != nil {
		log.Fatalf("Failed to retrieve records: %v", err)
	}

	// Display retrieved records
	fmt.Println("AI Index Records from Local Node:")
	for _, record := range records {
		fmt.Printf("Event Time: %d, Value: %s\n", 
			record.EventTime, 
			record.Value.String(),
		)
	}
}
```

### Troubleshooting

- Ensure your local node is fully synchronized
- Check network connectivity
- Verify private key and authentication
- Review node logs for any synchronization issues

## Mainnet Network

We have a mainnet network accessible at https://gateway.mainnet.truf.network. You can interact with it to test and experiment with the TN SDK. Please use it responsibly. Any contributions and feedback are welcome.

### Connecting to Mainnet

To connect to the mainnet, simply change the endpoint in your client initialization:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
)

func main() {
	ctx := context.Background()

	// Set up mainnet connection
	pk, err := crypto.Secp256k1PrivateKeyFromHex("your-private-key")
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}
	signer := &auth.EthPersonalSigner{Key: *pk}

	tnClient, err := tnclient.NewClient(
		ctx,
		"https://gateway.mainnet.truf.network",  // Mainnet endpoint
		tnclient.WithSigner(signer),
	)
	if err != nil {
		log.Fatalf("Failed to create TN client: %v", err)
	}

	// Now you can perform operations on the mainnet
	fmt.Println("Connected to TRUF.NETWORK Mainnet")
}
```

## Types of Streams

- **Primitive Streams**: Direct data sources from providers. Examples include indexes from known sources, aggregation output such as sentiment analysis, and off-chain/on-chain data.
- **Composed Streams**: Aggregate and process data from multiple streams.
- **System Streams**: Contract-managed streams audited and accepted by TN governance to ensure quality. 

See [type of streams](./docs/type-of-streams.md) and [default TN contracts](./docs/contracts.md) guides for more information.

## Roles and Responsibilities

- **Data Providers**: Publish and maintain data streams, taxonomies, and push primitives.
- **Consumers**: Access and utilize stream data. Examples include researchers, analysts, financial institutions, and DApp developers.
- **Node Operators**: Maintain network infrastructure and consensus. Note: The network is currently in a centralized phase during development. Decentralization is planned for future releases. This repository does not handle node operation.

## Key Concepts

### Stream ID Composition

Stream IDs are unique identifiers generated for each stream. They ensure consistent referencing across the network.

### Types of Data Points

- **Record**: Data points used to calculate indexes. If a stream is a primitive, records are the raw data points. If a stream is composed, records are the weighted values.
- **Index**: Calculated values derived from stream data, representing a value's growth compared to the stream's first record.
- **Primitives**: Raw data points provided by data sources.

### Transaction Lifecycle

TN operations rely on blockchain transactions. Some actions require waiting for previous transactions to be mined before proceeding. For detailed information on transaction dependencies and best practices, see [Stream Lifecycle](./docs/stream-lifecycle.md).

## Permissions and Privacy

TN supports granular control over stream access and visibility. Streams can be public or private, with read and write permissions configurable at the wallet level. Additionally, you can control whether other streams can compose data from your stream. For more details, refer to [Stream Permissions](./docs/stream-permissions.md).

## Caveats

- **Transaction Confirmation**: Always wait for transaction confirmation before performing dependent actions. For more information, see the [Stream Lifecycle](./docs/stream-lifecycle.md) section.

## Further Reading

- [TN-SDK Documentation](./docs/readme.md)
- [Truflation Whitepaper](https://whitepaper.truflation.com/)

For additional support or questions, please [open an issue](https://github.com/trufnetwork/sdk-go/issues) or contact our support team.

## License

The SDK-Go repository is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE.md) for more details.

## Stream Creation and Management

The TN SDK provides comprehensive support for creating and managing both primitive and composed streams. 

### Primitive Streams

Primitive streams are raw data sources that can represent various types of data points. To create a primitive stream:

```go
// Generate a unique stream ID
primitiveStreamId := util.GenerateStreamId("my-market-data-stream")

// Deploy the primitive stream
deployTx, err := tnClient.DeployStream(ctx, primitiveStreamId, types.StreamTypePrimitive)

// Insert records into the primitive stream
primitiveActions, err := tnClient.LoadPrimitiveActions()
insertTx, err := primitiveActions.InsertRecords(ctx, []types.InsertRecordInput{
    {
        DataProvider: dataProviderAddress,
        StreamId:     primitiveStreamId.String(),
        EventTime:    int(time.Now().Unix()),
        Value:        100.5,
    },
})
```

### Composed Streams

Composed streams aggregate and process data from multiple primitive or other composed streams. They use a taxonomy to define how child streams are combined:

```go
// Deploy a composed stream
composedStreamId := util.GenerateStreamId("my-composite-index")
deployTx, err := tnClient.DeployStream(ctx, composedStreamId, types.StreamTypeComposed)

// Load composed actions
composedActions, err := tnClient.LoadComposedActions()

// Set taxonomy (define how child streams are combined)
taxonomyTx, err := composedActions.InsertTaxonomy(ctx, types.Taxonomy{
    ParentStream: tnClient.OwnStreamLocator(composedStreamId),
    TaxonomyItems: []types.TaxonomyItem{
        {
            ChildStream: tnClient.OwnStreamLocator(primitiveStreamId1),
            Weight:      0.6, // 60% weight
        },
        {
            ChildStream: tnClient.OwnStreamLocator(primitiveStreamId2),
            Weight:      0.4, // 40% weight
        },
    },
})
```

### Complex Stream Creation Example

For a comprehensive example demonstrating stream creation, taxonomy setup, and data retrieval, see the `examples/complex_stream_example/main.go` file. This example shows:

- Deploying primitive streams
- Inserting records into primitive streams
- Creating a composed stream
- Setting up stream taxonomy
- Retrieving composed stream records

Key steps include:
1. Generating unique stream IDs
2. Deploying primitive and composed streams
3. Inserting records into primitive streams
4. Defining stream taxonomy
5. Retrieving composed stream records

This example provides a practical walkthrough of creating and managing streams in the TRUF.NETWORK ecosystem.

### Stream Locators and Data Providers

#### Stream Locators

A `StreamLocator` is a unique identifier for a stream that consists of two key components:
1. `StreamId`: A unique identifier for the stream
2. `DataProvider`: The Ethereum address of the stream's creator/owner

The `OwnStreamLocator()` method is a convenience function that automatically creates a `StreamLocator` using:
- The provided `StreamId`
- The current client's Ethereum address

Example:
```go
// Creates a StreamLocator with:
// - The given stream ID
// - The current client's address as the data provider
streamLocator := tnClient.OwnStreamLocator(myStreamId)
```

This is particularly useful when you're creating and managing your own streams, as it automatically uses your client's address.

#### Data Providers

A `DataProvider` is the Ethereum address responsible for creating and managing a stream. When inserting records or performing operations on a stream, you need to specify the data provider's address.

To get the current client's address, use:
```go
// Get the current client's Ethereum address
dataProviderAddress := tnClient.Address()

// Get the address as a string for use in stream operations
dataProviderAddressString := dataProviderAddress.Address()
```

Key differences:
- `tnClient.Address()` returns an `EthereumAddress` object
- `dataProviderAddress.Address()` returns the address as a string, which is used in stream operations

### Example of Stream Creation with Locators and Providers

```go
// Generate a stream ID
streamId := util.GenerateStreamId("my-stream")

// Deploy the stream using the current client's address
deployTx, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)

// Create a stream locator
streamLocator := tnClient.OwnStreamLocator(streamId)

// Get the data provider address
dataProvider := tnClient.Address()

// Insert a record using the data provider address
insertTx, err := primitiveActions.InsertRecords(ctx, []types.InsertRecordInput{
    {
        DataProvider: dataProvider.Address(),
        StreamId:     streamId.String(),
        EventTime:    int(time.Now().Unix()),
        Value:        100.5,
    },
})
```

This approach ensures that:
- Streams are uniquely identified
- Records are correctly attributed to their creator
- Stream operations are performed with the correct addressing

### Stream Deletion and Resource Management

#### Why Delete Streams?

Stream deletion is crucial for:
- Cleaning up unused or test streams
- Managing resource consumption
- Maintaining a clean and organized stream ecosystem

#### Stream Deletion Process

Streams can be deleted using the `DestroyStream()` method:

```go
// Destroy a specific stream
destroyTx, err := tnClient.DestroyStream(ctx, streamId)
if err != nil {
    // Handle deletion error
    log.Printf("Failed to destroy stream: %v", err)
}

// Wait for the destroy transaction to be mined
_, err = tnClient.WaitForTx(ctx, destroyTx, time.Second*5)
if err != nil {
    log.Printf("Error waiting for stream destruction: %v", err)
}
```

#### Best Practices for Stream Deletion

1. **Cleanup in Reverse Order**
   - Delete composed streams before their child primitive streams
   - Ensures proper resource management and prevents orphaned references

2. **Error Handling**
   - Always check for errors during stream deletion
   - Log and handle potential issues gracefully

3. **Deferred Deletion**
   - Use `defer` for automatic cleanup in test or example scenarios
   - Ensures resources are freed even if an error occurs

Example of Deferred Stream Deletion:
```go
func main() {
    // Defer stream destruction
    defer func() {
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

            // Wait for the destroy transaction
            _, err = tnClient.WaitForTx(ctx, destroyTx, time.Second*5)
            if err != nil {
                log.Printf("Error waiting for destroy transaction: %v", err)
            }
        }
    }()

    // Rest of the stream creation and management code
}
```

#### Considerations

- Stream deletion is a permanent action
- Deleted streams cannot be recovered
- Ensure you have the necessary permissions to delete a stream
- In production, implement additional safeguards before deletion

### When to Delete Streams

- After completing testing
- When streams are no longer needed
- To free up resources
- As part of a stream lifecycle management strategy

By following these guidelines, you can effectively manage stream resources in the TRUF.NETWORK ecosystem.

