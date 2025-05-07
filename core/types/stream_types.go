package types

import "github.com/trufnetwork/sdk-go/core/util"

// StreamLocator is a struct that contains the StreamId and the DataProvider
type StreamLocator struct {
	// StreamId is the unique identifier of the stream, used as name of the deployed contract
	StreamId util.StreamId
	// DataProvider is the address of the data provider, it's the deployer of the stream
	DataProvider util.EthereumAddress
}

type ListStreamsInput struct {
	DataProvider string
	Limit        int
	Offset       int
	OrderBy      string
}

type ListStreamsOutput struct {
	DataProvider string `json:"data_provider"`
	StreamId     string `json:"stream_id"`
	StreamType   string `json:"stream_type"`
	CreatedAt    string `json:"created_at"`
}

// StreamDefinition defines the necessary information to deploy a stream.
type StreamDefinition struct {
	StreamId   util.StreamId // User-defined identifier for the stream
	StreamType StreamType    // Type of the stream (Primitive, Composed, etc.)
}
