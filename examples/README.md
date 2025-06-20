# Simple Stream Retrieval Example

## Overview

This example demonstrates how to retrieve records from an existing stream in the TRUF.NETWORK (TN) SDK.

## Purpose

The simple example shows:
- Connecting to a TN node (local or mainnet)
- Retrieving records from a predefined stream
- Basic error handling
- Displaying stream data

## Key Concepts

- Initializing a TN client
- Connecting to a stream
- Fetching stream records
- Handling time-based record retrieval

## Prerequisites

- Go 1.20 or later
- TRUF.NETWORK SDK
- Access to a TN node (local or mainnet)
- Existing stream to retrieve data from

## Configuration

Before running the example:
1. Replace `"your-private-key"` with a valid private key
2. Adjust the `endpoint` to match your TN node
3. Ensure the stream ID and data provider are correct

## Running the Example

```bash
go mod tidy
go run main.go
```

## Important Notes

- The example uses a predefined AI Index stream
- Modify stream details to match your specific use case
- Always handle potential errors in production code

## Customization

You can adapt this example to:
- Retrieve records from different streams
- Modify time ranges for record retrieval
- Add more complex data processing logic 