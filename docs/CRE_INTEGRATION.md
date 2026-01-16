# Chainlink Runtime Environment (CRE) Integration

This guide explains how to use the TRUF.NETWORK SDK in Chainlink Runtime Environment (CRE) workflows for building decentralized applications with consensus-backed data retrieval.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [API Reference](#api-reference)

---

## Overview

### What is Chainlink Runtime Environment (CRE)?

Chainlink Runtime Environment (CRE) is a secure, deterministic execution environment for building decentralized workflows. CRE workflows:

- Run as WebAssembly (WASM) for security and portability
- Execute across a Decentralized Oracle Network (DON) with Byzantine fault tolerance
- Use consensus mechanisms to ensure agreement on results
- Provide specialized capabilities like HTTP requests, scheduling, and compute

### Why Use TRUF.NETWORK SDK with CRE?

The TRUF.NETWORK SDK enables CRE workflows to:

- **Fetch verified data streams** from TRUF.NETWORK with DON consensus
- **Write data back** to streams with authenticated transactions
- **Build decentralized applications** combining on-chain and off-chain data
- **Leverage TRUF's stream infrastructure** for economic and financial data

### Use Cases

- **Decentralized funds:** Bitcoin/crypto funds using TRUF data streams (e.g., QuantAMM)
- **Data aggregation:** Combining multiple TRUF streams with consensus
- **Cross-chain oracles:** Bridging TRUF data to multiple blockchains
- **Automated strategies:** Scheduled workflows reading and writing stream data

---

## Prerequisites

### Required Software

1. **Go 1.25.3 or later**
```bash
go version  # Should show 1.25.3+
```

2. **CRE SDK**
```bash
go get github.com/smartcontractkit/cre-sdk-go@latest
```

3. **TRUF.NETWORK SDK**
```bash
go get github.com/trufnetwork/sdk-go
```

4. **WebAssembly toolchain**
```bash
# Verify WASM support
GOOS=wasip1 GOARCH=wasm go version
```

---

## Quick Start

**For general CRE setup**, see [Chainlink CRE Documentation](https://docs.chain.link/cre).

**Key TRUF.NETWORK integration:**

1. **Install the SDK:**
```bash
go get github.com/trufnetwork/sdk-go@latest
```

2. **Use CRE transport in your workflow:**
```go
//go:build wasip1

return cre.RunInNodeMode(config, runtime,
    func(config *Config, nodeRuntime cre.NodeRuntime) (*Result, error) {
        // Create TRUF client with CRE transport
        client, err := tnclient.NewClient(ctx, config.TRUFEndpoint,
            tnclient.WithCRETransport(nodeRuntime, config.TRUFEndpoint),
        )

        // Use any SDK method
        streams, err := client.ListStreams(ctx, types.ListStreamsInput{})
        return &Result{Streams: streams}, nil
    },
    cre.ConsensusAggregationFromTags[*Result](),
).Await()
```

3. **Build for WASM:**
```bash
GOOS=wasip1 GOARCH=wasm go build -o workflow.wasm
```

**For complete working code**, see [TRUF + CRE Demo](../examples/truf-cre-demo/)

---

## Configuration

### Build Tags

**CRITICAL:** All CRE-specific code must use the `wasip1` build tag:

```go
//go:build wasip1

package main
```

This ensures:
- CRE code only compiles for WASM target
- Regular builds exclude CRE dependencies
- No conflicts with standard `net/http`

### Client Configuration

#### Read-Only Access (No Signer)

```go
client, err := tnclient.NewClient(ctx, endpoint,
    tnclient.WithCRETransport(nodeRuntime, endpoint),
)
```

Use for:
- Listing streams
- Reading records
- Querying data

#### Write Access (With Signer)

```go
import "github.com/trufnetwork/kwil-db/core/crypto/auth"

// Create signer from private key
signer := &auth.EthPersonalSigner{Key: privateKey}

// Option 1: Use convenience function
client, err := tnclient.NewClient(ctx, endpoint,
    tnclient.WithCRETransportAndSigner(nodeRuntime, endpoint, signer),
)

// Option 2: Explicit options (same result)
client, err := tnclient.NewClient(ctx, endpoint,
    tnclient.WithSigner(signer),
    tnclient.WithCRETransport(nodeRuntime, endpoint),
)
```

Use for:
- Inserting records
- Deploying streams
- Any write operations

**Important:** Option ordering matters! `WithSigner` must come before `WithCRETransport` if using separate options.

---

### CRE HTTP caching (recommended for non-idempotent writes)

CRE executes workflow logic across multiple DON nodes. Without additional controls, **each node may independently issue the same HTTP request**, including requests that have side effects. For non-idempotent operations (for example, submitting transactions or creating resources), this can result in duplicate external calls.

CRE’s Go HTTP client supports best-effort request de-duplication via `CacheSettings` on `http.Request`. When enabled, one node performs the request and stores the response; other nodes can reuse it if it is still fresh (`MaxAge`). This is most appropriate for **POST/PUT/PATCH/DELETE-style** operations that should not be executed multiple times.

Example (raw CRE HTTP request):

```go
import (
    "time"
    "google.golang.org/protobuf/types/known/durationpb"
    crehttp "github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http"
)

// ...
req := &crehttp.Request{
    Url:    "https://example.com/api",
    Method: "POST",
    Body:   payloadBytes,
    CacheSettings: &crehttp.CacheSettings{
        Store:  true,
        MaxAge: durationpb.New(2 * time.Minute),
    },
}
resp, err := client.SendRequest(nodeRuntime, req).Await()

```
---

## API Reference

For detailed API documentation including function signatures, parameters, and usage examples, see:

**📖 [CRE Transport Options - API Reference](./api-reference.md#chainlink-runtime-environment-cre)**

**Quick Reference:**
- `WithCRETransport(runtime, endpoint)` - Read-only access to TRUF streams
- `WithCRETransportAndSigner(runtime, endpoint, signer)` - Read + Write access (insert records, deploy streams)

---

## Resources

### Documentation
- [Chainlink CRE Documentation](https://docs.chain.link/cre)
- [TRUF.NETWORK SDK Documentation](./api-reference.md)
- [CRE SDK Go Documentation](https://pkg.go.dev/github.com/smartcontractkit/cre-sdk-go)
- [CRE HTTP documentation](https://docs.chain.link/cre/guides/workflow/using-http-client/post-request-go)

### Examples
- [TRUF + CRE Complete Demo](../examples/truf-cre-demo/) - 3-workflow pattern demonstrating full CRUD lifecycle

### Support
- [TRUF.NETWORK Discord](https://discord.gg/trufnetwork)
- [Chainlink Discord](https://discord.gg/chainlink)
- [GitHub Issues](https://github.com/trufnetwork/sdk-go/issues)