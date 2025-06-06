# API Reference

## TRUF.NETWORK SDK Overview

The TRUF.NETWORK SDK provides a comprehensive toolkit for developers to interact with decentralized data streams. It enables seamless creation, management, and consumption of economic and financial data streams.

### Key Features
- Stream creation and management
- Primitive and composed stream support
- Flexible data retrieval
- Advanced permission management
- Secure, blockchain-backed data streams

## Interfaces

The SDK is structured around several key interfaces:

- [Client](client.md): Primary entry point for network interactions
- [Primitive Stream](primitive-stream.md): Raw data stream management
- [Composed Stream](composed-stream.md): Aggregated data stream handling

## Core Concepts

### Streams
- **Primitive Streams**: Direct data sources with raw data points
- **Composed Streams**: Aggregated streams combining multiple data sources

### Data Management
- Secure, immutable data recording
- Flexible querying and indexing
- Granular access control

## Example Usage

```go
package main

import (
	"context"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

func main() {
	ctx := context.Background()

	// Initialize client with mainnet endpoint
	tnClient, err := tnclient.NewClient(
		ctx, 
		"https://gateway.mainnet.truf.network",
		tnclient.WithSigner(mySigner),
	)
	if err != nil {
		// Handle client initialization error
	}

	// Deploy a primitive stream
	streamId := util.GenerateStreamId("my-economic-stream")
	deployTx, err := tnClient.DeployStream(
		ctx, 
		streamId, 
		types.StreamTypePrimitive,
	)
	// Handle deployment and further stream operations
}
```

## Getting Started

1. Install the SDK
2. Configure your network endpoint
3. Initialize a client
4. Create and manage streams

## Support and Community

- [GitHub Repository](https://github.com/trufnetwork/sdk-go)
- [Issue Tracker](https://github.com/trufnetwork/sdk-go/issues)
- [Documentation](https://docs.truf.network)