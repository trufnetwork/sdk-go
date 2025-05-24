# Truf Network (TN) SDK

The Truf Node SDK provides developers with tools to interact with the Truf Network, a decentralized platform for publishing, composing, and consuming economic data streams.

## Support

This documentation is a work in progress. If you need help, don't hesitate to [open an issue](https://github.com/trufnetwork/sdk-go/issues).

## Quick Start

### Prerequisites

- Go 1.20 or later

### Installation

```bash
go get github.com/trufnetwork/sdk-go

```

### Example Usage

#### 1. Creating and Using a Primitive Stream
```go
package main

import (
	"context"
	"fmt"
	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
	"time"
)

func main() {
	ctx := context.Background()

	// Create TN client with Ethereum signer
	pk, _ := crypto.Secp256k1PrivateKeyFromHex("<your-private-key-hex>")
	signer := &auth.EthPersonalSigner{Key: *pk}
	tnClient, err := tnclient.NewClient(ctx, "https://tsn-provider-url.com", 
		tnclient.WithSigner(signer))
	if err != nil {
		panic(err)
	}

	// Create and deploy a new stream
	streamId := util.GenerateStreamId("price-eth-usd")
	txHash, err := tnClient.DeployStream(ctx, streamId, types.StreamTypePrimitive)
	if err != nil {
		panic(err)
	}

	// Wait for transaction confirmation
	_, err = tnClient.WaitForTx(ctx, txHash, 10*time.Second)
	if err != nil {
		panic(err)
	}

	// Load the primitive stream
	primitive, err := tnClient.LoadPrimitiveActions()
	if err != nil {
		panic(err)
	}

	// Insert price data with Unix timestamp
	_, err = primitive.InsertRecord(ctx, types.InsertRecordInput{
		DataProvider: tnClient.Address().Address(),
		StreamId:     streamId.String(),
		EventTime:    int(time.Now().Unix()),
		Value:        3500.50,
	})
	if err != nil {
		panic(err)
	}

	// Query stream data using Unix timestamps
	streamLocator := tnClient.OwnStreamLocator(streamId)
	records, err := primitive.GetRecord(ctx, types.GetRecordInput{
		From: util.Ptr(int(time.Now().AddDate(0, 0, -7).Unix())),
		To:   util.Ptr(int(time.Now().Unix())),
	})
	if err != nil {
		panic(err)
	}

	for _, record := range records {
		fmt.Printf("Timestamp: %d, Value: %s\n", 
			record.EventTime, 
			record.Value.String())
	}
}
```

#### 2. Querying a Composed Stream (AI Index Example)
```go
package main

import (
	"context"
	"fmt"
	"time"
	
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

func main() {
	ctx := context.Background()
	
	// Initialize client with mainnet gateway
	pk, _ := crypto.Secp256k1PrivateKeyFromHex("<your-private-key-hex>")
	signer := &auth.EthPersonalSigner{Key: *pk}
	tnClient, err := tnclient.NewClient(
		ctx,
		"https://gateway.infra.truf.network", // Mainnet endpoint
		tnclient.WithSigner(signer),
	)
	if err != nil {
		panic(err)
	}

	// Configure AI Index stream parameters
	dataProvider, _ := util.NewEthereumAddressFromString("0x4710a8d8f0d845da110086812a32de6d90d7ff5c")
	streamId, _ := util.NewStreamId("st527bf3897aa3d6f5ae15a0af846db6")
	
	// Query last week's data
	now := time.Now().Unix()
	weekAgo := time.Now().AddDate(0, 0, -7).Unix()
	
	records, err := tnClient.LoadActions().GetRecord(ctx, types.GetRecordInput{
		DataProvider: dataProvider.Address(),
		StreamId:     streamId.String(),
		From:         util.Ptr(int(weekAgo)),
		To:           util.Ptr(int(now)),
	})
	if err != nil {
		panic(err)
	}

	// Display results with explorer link
	fmt.Printf("\nAI Index Historical Values (View on explorer: %s)\n",
		"https://truf.network/explorer/0x4710a8d8f0d845da110086812a32de6d90d7ff5c/st527bf3897aa3d6f5ae15a0af846db6")
		
	for _, record := range records {
		fmt.Printf("%s: %s\n",
			time.Unix(int64(record.EventTime), 0).UTC().Format("2006-01-02"),
			record.Value.String())
	}
}
```

For more comprehensive examples and usage patterns, please refer to the test files in the SDK repository. These tests provide detailed examples of various stream operations and error-handling scenarios.

## Mainnet Network

We have a mainnet network accessible at https://gateway.infra.truf.network. You can interact with it to test and experiment with the TN SDK. Please use it responsibly, as TN is currently in an experimental phase. Any contributions and feedback are welcome.

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

