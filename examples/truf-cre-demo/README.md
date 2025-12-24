# TRUF + CRE Integration Example

This example demonstrates how to integrate TRUF.NETWORK SDK with Chainlink Runtime Environment (CRE) workflows, showcasing the **complete stream CRUD lifecycle** across three separate workflows.

## 🎯 Overview

The demo is split into **3 separate workflows** to stay within CRE simulation's 5 HTTP request limit per workflow:

1. **Write Workflow** (`truf-write-workflow/`) - Deploy stream + Insert records
2. **Read Workflow** (`truf-read-workflow/`) - Retrieve records
3. **Cleanup Workflow** (`truf-cleanup-workflow/`) - Delete stream

An orchestration script (`run-full-demo.sh`) runs all three workflows sequentially to demonstrate the complete CRUD lifecycle.

## ⚙️ Setup

Before running the demo, configure your private key:

1. **Edit `config.json`** in the `truf-cre-demo/` directory
2. **Replace `YOUR_PRIVATE_KEY_HERE`** with your Ethereum private key (without `0x` prefix)
3. **Ensure your account has write permissions** on the TRUF node

```json
{
  "schedule": "0 */1 * * * *",
  "trufEndpoint": "https://gateway.mainnet.truf.network",
  "privateKey": "your_actual_private_key_here"
}
```

**Note**: All 3 workflows share the same `config.json` file via their `workflow.yaml` settings.

## 🚀 Quick Start

### Run the Complete Demo

```bash
cd examples/truf-cre-demo
./run-full-demo.sh
```

This will execute all 3 workflows in sequence with 5-second delays for transaction confirmation.

### Run Individual Workflows

```bash
# Deploy + Insert
cre workflow simulate truf-write-workflow

# Get Records
cre workflow simulate truf-read-workflow

# Delete Stream
cre workflow simulate truf-cleanup-workflow
```

## 📋 Workflow Details

### Workflow 1: Write (Deploy + Insert)

**Purpose**: Create stream and insert data
**HTTP Requests**: 5/5 (at limit)
- `chain_info` - Get chain metadata
- `authn_param` + `authn` - Authentication
- `user.account` - Fetch nonce
- `user.broadcast` - Deploy stream
- `user.broadcast` - Insert record

**Operations**:
1. Deploy primitive stream (`stcreteststream00000000000000000`)
2. Insert 1 sample record with timestamp and value
3. Leave stream active for read workflow

**Result**: Stream deployed, 1 record inserted

---

### Workflow 2: Read (Get Records)

**Purpose**: Query stream data
**HTTP Requests**: 4/5
- `chain_info`, `authn_param`, `authn` - Setup
- `user.call` - Get records (read operation)

**Operations**:
1. Retrieve all records from last 24 hours
2. Sort by eventTime (newest first)
3. Display top 5 records with timestamps

**Result**: Records retrieved and displayed

---

### Workflow 3: Cleanup (Delete Stream)

**Purpose**: Clean up resources
**HTTP Requests**: 5/5 (at limit)
- `chain_info`, `authn_param`, `authn` - Setup
- `user.account` - Fetch nonce
- `user.broadcast` - Delete stream

**Operations**:
1. Destroy the stream using `client.DestroyStream()`
2. Free blockchain resources

**Result**: Stream deleted

## 🔧 Configuration

Each workflow uses `config.json`:

```json
{
  "schedule": "0 */1 * * * *",
  "trufEndpoint": "https://gateway.mainnet.truf.network",
  "privateKey": "0000000000000000000000000000000000000000000000000000000000000001"
}
```

**⚠️ Important**: The example uses a well-known test private key. For production, use a securely generated private key and **never commit it to version control**.

## 🏗️ Architecture

### Why 3 Separate Workflows?

CRE simulation enforces a **5 HTTP request limit per workflow**. Our operations require:

- **Write operations** (deploy, insert): Each needs setup + broadcast (2-3 requests each)
- **Read operations** (get records): Needs separate auth for `user.call`
- **Delete operations**: Needs setup + broadcast

Splitting into 3 workflows ensures each stays within the limit while demonstrating the full CRUD lifecycle.

### Key SDK Features Demonstrated

1. **CRE Transport Integration**
   ```go
   client, err := tnclient.NewClient(ctx, endpoint,
       tnclient.WithCRETransportAndSigner(nodeRuntime, endpoint, signer),
   )
   ```

2. **Stream Deployment**
   ```go
   txHash, err := client.DeployStream(ctx, streamId, types.StreamTypePrimitive)
   ```

3. **Record Insertion**
   ```go
   primitiveActions, _ := client.LoadPrimitiveActions()
   txHash, err := primitiveActions.InsertRecords(ctx, records)
   ```

4. **Record Retrieval**
   ```go
   actions, _ := client.LoadActions()
   result, err := actions.GetRecord(ctx, input)
   ```

5. **Stream Deletion**
   ```go
   txHash, err := client.DestroyStream(ctx, streamId)
   ```

## 📊 Expected Output

### Write Workflow
```
=== TRUF CRE Write Workflow: Deploy & Insert Demo ===
Signer created address=0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf
✅ Stream deployment transaction submitted txHash=...
✅ Records inserted successfully count=1
=== Write Workflow Completed Successfully ===
```

### Read Workflow
```
=== TRUF CRE Read Workflow: Get Records Demo ===
Top 5 records (descending by eventTime) total=1
Record rank=1 eventTime=... value=102.700000000000000000
✅ Records retrieved successfully count=1
=== Read Workflow Completed Successfully ===
```

### Cleanup Workflow
```
=== TRUF CRE Cleanup Workflow: Delete Stream Demo ===
✅ Stream deletion transaction submitted txHash=...
=== Cleanup Workflow Completed Successfully ===
```

## 🔑 Key Implementation Details

### Handling Duplicate Streams

The write workflow gracefully handles existing streams:

```go
deployTxHash, err := client.DeployStream(ctx, streamId, types.StreamTypePrimitive)
if err != nil && strings.Contains(err.Error(), "duplicate key") {
    logger.Info("Stream already exists, continuing with existing stream")
    result.Deployed = true
} else if err != nil {
    return result, nil
}
```

### Transaction Signing

Both HTTP and CRE transports use the same signing mechanism:

```go
privateKey, _ := kwilCrypto.Secp256k1PrivateKeyFromHex(privateKeyHex)
signer := &auth.EthPersonalSigner{Key: *privateKey}
```

**Critical Fix**: Fee must be `big.NewInt(0)`, not `nil`:
- When Fee is `nil`, serialization produces `"Fee: <nil>"`
- After JSON marshaling, it becomes `"Fee: 0"`
- This mismatch causes signature verification to fail

### Nonce Management

The SDK automatically manages nonces:
- Fetches current nonce from `user.account`
- Caches and increments for subsequent transactions
- Account nonce is **last used**, so next transaction uses `nonce + 1`

## 🛠️ Development

### Prerequisites
- Go 1.25.3 or later
- CRE CLI installed
- Access to TRUF.NETWORK gateway

### Project Structure
```
truf-cre-demo/
├── run-full-demo.sh           # Orchestration script
├── truf-write-workflow/       # Workflow 1: Deploy + Insert
│   ├── main.go
│   ├── config.json
│   └── workflow.yaml
├── truf-read-workflow/        # Workflow 2: Get Records
│   ├── main.go
│   ├── config.json
│   └── workflow.yaml
└── truf-cleanup-workflow/     # Workflow 3: Delete Stream
    ├── main.go
    ├── config.json
    └── workflow.yaml
```

### Adding More Records

To insert more records, modify `truf-write-workflow/main.go`:

```go
recordsToInsert := []struct {
    timestamp int64
    value     float64
}{
    {currentTime - 120, 100.5},  // 2 minutes ago
    {currentTime - 60, 101.2},   // 1 minute ago
    {currentTime, 102.7},        // now
}
```

**Note**: Adding more inserts will exceed the 5 HTTP request limit. Consider creating an additional workflow for bulk inserts.

## 🐛 Troubleshooting

### "cannot use 6, limit is 5" Error
- **Cause**: Workflow exceeded 5 HTTP request limit
- **Solution**: Split operations into separate workflows

### Signature Mismatch Error
- **Cause**: Fee is `nil` instead of `big.NewInt(0)`
- **Solution**: Ensure all transactions set Fee explicitly

### Nonce Error
- **Cause**: Using wrong nonce value
- **Solution**: SDK handles this automatically; if issues persist, check transport implementation

## 📚 Additional Resources

- [TRUF.NETWORK Documentation](https://docs.truf.network)
- [Chainlink CRE Documentation](https://docs.chain.link/chainlink-cre)
- [TRUF SDK API Reference](../docs/api-reference.md)

## 🤝 Contributing

When contributing workflow examples:
1. Ensure each workflow stays within the 5 HTTP request limit
2. Add comprehensive logging for debugging
3. Handle errors gracefully
4. Document any CRE-specific considerations

---

**Note**: This example demonstrates CRE integration patterns. For production deployments, follow security best practices including proper key management, error handling, and monitoring.
