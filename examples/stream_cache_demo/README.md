# Stream Cache Demo

This example demonstrates comprehensive usage of the TRUF SDK's caching capabilities across all stream operations.

## Features Demonstrated

- Cache usage in `GetRecord`, `GetIndex`, `GetIndexChange`, and `GetFirstRecord`
- Cache metadata analysis and interpretation
- Performance optimization strategies
- Cache data freshness validation
- Backward compatibility with existing code
- Real-world usage patterns

## Running the Example

```bash
go run main.go
```

## Key Learning Points

1. **Cache Parameters**: All stream query methods support optional `UseCache` parameter
2. **Metadata Analysis**: Every query returns detailed cache performance metrics
3. **Performance Optimization**: Strategic caching can significantly improve query performance
4. **Data Freshness**: Cache age validation for time-sensitive applications
5. **Backward Compatibility**: Existing code continues to work without modifications

## Example Output

The example shows:
- Cache hit/miss patterns
- Performance improvements with caching
- Cache metadata interpretation
- Data freshness analysis
- Aggregated performance metrics