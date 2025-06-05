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

   **Note:** Setting up a local node as described above will initialize an empty database. This setup is primarily for testing the technology or development purposes. If you are a node operator and wish to sync with the Truf Network to access real data, please follow the [Node Operator Guide](https://github.com/trufnetwork/node/blob/main/docs/node-operator-guide.md) for instructions on connecting to the network and syncing data.

4. **Verify Node Synchronization**

When running a local node, it's crucial to ensure it's fully synchronized before querying data. Use the following example to check node status:

```bash
kwild admin status
```

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
	fmt.Println("Connected to Truf Network Mainnet")
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

Stream IDs are unique identifiers generated for each stream. They ensure consistent referencing across the network. It's used as the contract name. A contract identifier is a hash over the deployer address (data provider) and the stream ID.

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

