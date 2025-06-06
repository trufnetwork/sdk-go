# Primitive Stream Interface

## Overview

Primitive streams are the foundational data sources in the TRUF.NETWORK ecosystem. They represent raw, unprocessed data points that can be used directly or as components in more complex composed streams.

## Key Characteristics

- Direct data input mechanism
- Immutable record storage

## Record Insertion

### `InsertRecords`

```go
InsertRecords(ctx context.Context, inputs []types.InsertRecordInput) (transactions.TxHash, error)
```

Allows insertion of one or multiple records into a primitive stream.

#### Record Input Structure

```go
type InsertRecordInput struct {
    DataProvider string    // Address of the data provider
    StreamId     string    // Unique stream identifier
    EventTime    int       // Unix timestamp of the record
    Value        float64   // Numeric value of the record
}
```

#### Example Usage

```go
// Insert a single record
records := []types.InsertRecordInput{
    {
        DataProvider: myAddress,
        StreamId:     "my-economic-stream",
        EventTime:    int(time.Now().Unix()),
        Value:        105.75,  // Economic indicator value
    },
}

txHash, err := primitiveStream.InsertRecords(ctx, records)
```

## Best Practices

1. **Consistent Timestamps**
   - Use UTC timestamps
   - Handle potential time zone complexities

2. **Data Validation**
   - Validate input values before insertion

3. **Error Handling**
   - Implement retry mechanisms
   - Log insertion failures

## Performance Considerations

- Batch record insertions when possible