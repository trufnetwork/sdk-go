# Attestation Example

This example demonstrates how to use the TN SDK to request and retrieve data attestations.

## Overview

Attestations provide cryptographically signed proof of query results from the Truf Network. This is useful for:

- Creating verifiable data feeds for smart contracts
- Providing tamper-proof records of historical data
- Enabling trustless data verification

## Features Demonstrated

1. **Request Attestation** - Submit a request for a signed attestation of query results
2. **Retrieve Signed Attestation** - Fetch the complete signed attestation payload
3. **List Attestations** - Query your recent attestation requests

## Prerequisites

- Go 1.24 or later
- A running TN node (local or remote)
- Private key with access to TN network
- `.env` file with configuration

## Setup

1. Create a `.env` file in the sdk-go root directory:

```bash
PRIVATE_KEY=your_private_key_here
PROVIDER_URL=http://localhost:8484  # Or your remote node URL
```

2. Install dependencies:

```bash
cd ../..
go mod download
```

## Running the Example

```bash
cd examples/attestation_example
go run main.go
```

## Expected Output

```text
Connected to TN network at http://localhost:8484
Using address: 0x...

=== Requesting Attestation ===
Data Provider: 0x4710a8d8f0d845da110086812a32de6d90d7ff5c
Stream ID: stai0000000000000000000000000000
Time Range: 2025-10-10T... to 2025-10-17T...

✓ Attestation requested successfully!
Request TX ID: 0x...
Attestation Hash: a1b2c3...

Waiting for attestation to be signed...

=== Retrieving Signed Attestation ===
✓ Retrieved signed attestation!
Payload size: 245 bytes
Payload (hex): 01000000...

=== Listing My Recent Attestations ===
Found 1 recent attestations:
1. TX: 0x..., Created: height 12345, Status: signed at height 12346

=== Example Complete ===
✓ Successfully demonstrated attestation workflow
```

## Understanding the Workflow

### 1. Request Attestation

```go
result, err := attestationActions.RequestAttestation(ctx, types.RequestAttestationInput{
    DataProvider: "0x...",         // Data provider address
    StreamID:     "stai000...",    // 32-char stream ID
    ActionName:   "get_record",    // Query action to attest
    Args:         []any{...},      // Action arguments
    EncryptSig:   false,           // Encryption not supported in MVP
    MaxFee:       1000000,         // Maximum fee
})
```

**Important**: The `use_cache` parameter (last argument) will be automatically forced to `false` by the node to ensure deterministic execution across all validators.

### 2. Wait for Signing

Attestations are signed asynchronously by validators. In production:

- Poll `GetSignedAttestation` periodically
- Use event listeners if available
- Typical signing time: 5-15 seconds

### 3. Retrieve Signed Attestation

```go
signedResult, err := attestationActions.GetSignedAttestation(ctx, types.GetSignedAttestationInput{
    RequestTxID: result.RequestTxID,
})
```

The payload contains:
1. **Canonical payload** (8 fields):
   - Version (uint8)
   - Algorithm (uint8) - 0 for secp256k1
   - Block height (uint64)
   - Data provider (20 bytes)
   - Stream ID (32 bytes)
   - Action ID (uint16)
   - Arguments (variable)
   - Result (variable, ABI-encoded)
2. **Signature** (65 bytes)

### 4. Using the Attestation

Once you have the signed attestation payload, you can:

- **Verify locally**: Parse payload and verify signature in Go
- **Submit to EVM**: Pass payload to smart contract for on-chain verification
- **Store for audit**: Keep as tamper-proof record

## Error Handling

Common errors and solutions:

| Error | Cause | Solution |
|-------|-------|----------|
| `data_provider must be 0x-prefixed 40 hex characters` | Invalid address format | Ensure address is lowercase 0x-prefixed |
| `stream_id must be 32 characters` | Wrong stream ID length | Verify stream ID is exactly 32 chars |
| `Attestation not yet signed` | Signing still in progress | Wait longer or poll periodically |
| `Action not allowed for attestation` | Action not in allowlist | Only query actions (IDs 1-5) can be attested |

## Next Steps

After running this example:

1. **Parse the payload** - Use the canonical payload parser to extract fields
2. **Verify signatures** - Implement local signature verification
3. **EVM integration** - Submit attestations to your smart contracts
4. **Production usage** - Implement polling/event listeners for signing status

## Related Documentation

- [Attestation Quick Start](../../ATTESTATION_QUICKSTART.md)
- [Development Guide](../../ATTESTATION_DEVELOPMENT.md)
- [Implementation Summary](../../IMPLEMENTATION_SUMMARY.md)

## Troubleshooting

### Local Node Not Running

If you get connection errors:

```bash
cd ../../../node
task single:start
```

### Attestation Never Signs

Check node logs:

```bash
docker logs -f kwil-node
```

Look for attestation signing activity or errors.

### Address Mismatch

Ensure your private key's address has access to the specified data provider's streams.

## Support

For issues or questions:

- [Open an issue](https://github.com/trufnetwork/sdk-go/issues)
- Contact support team
