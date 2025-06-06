# Types of Streams in TN

The TRUF.NETWORK (TN) primarily supports two types of streams: Primitive and Composed. Additionally, both types can become System streams when accepted by the TN governance.

## Primitive Streams

Primitive streams are the foundation of the TN data ecosystem. They represent direct data sources from providers.

### Key Characteristics:

- Raw data input from trusted sources
- Can include off-chain and on-chain data
- Serve as building blocks for more complex data structures

### Examples:

- Economic indexes from reputable sources
- Aggregation outputs (e.g., sentiment analysis results)
- Raw market data (e.g., asset prices, trading volumes)

## Composed Streams

Composed streams aggregate and process data from multiple streams, allowing for complex data transformations and analyses.

### Key Characteristics:

- Derive data from multiple primitive or other composed streams
- Can implement custom logic for data processing
- Enable the creation of sophisticated financial products and indicators

### Default Behavior:

The composed stream contract implements a weighting mechanism for child streams by default. This allows for a straightforward data composition based on weighted averages of the child streams' values.

### Examples:

- Custom economic indices combining multiple data points
- Risk assessment scores based on various market indicators
- Yield farming strategies using multiple DeFi protocol data

## System Streams

System streams are streams (either Primitive or Composed) that have been audited and accepted by TN governance to ensure quality and reliability.

### Key Characteristics:

- Undergo a rigorous auditing process
- Managed by system contracts for enhanced security and trust
- Serve as reliable data sources for critical applications

### Data Access:

Users can fetch official (system) and unofficial streams through the system contract. However, they also have the option to fetch data directly from the stream contracts. For more detailed information on data retrieval, please refer to the `reading-data.md` document.

### Governance and Ownership:

When a stream is accepted as a system stream:

- The original data provider can still push primitives to the stream.
- The data provider cannot drop the stream or modify its taxonomy.
- Ownership of the stream is transferred to the TN governance.
- Any future changes to the stream are decided by the community through governance processes.

### Examples:

- Official inflation rate streams
- Benchmark interest rates
- Regulatory compliance data

## Stream Customization and Composability

While TN provides suggested contract templates for each stream type, users can alter contract logic as long as the procedure interfaces are maintained. This approach ensures:

1. **Flexibility**: Users can implement custom logic tailored to their specific use cases.
2. **Composability**: Streams remain interoperable within the TN ecosystem by maintaining consistent interfaces.
3. **Innovation**: Developers can create novel data products while leveraging the TN infrastructure.

### Important Considerations:

- Maintain the defined procedure interfaces to ensure compatibility with the TN ecosystem.
- Custom logic should adhere to best practices for security and efficiency.
- Consider the impact of customizations on stream composability and usability by other network participants.

## System Stream Governance

System streams undergo a rigorous process to ensure their reliability and trustworthiness:

1. **Proposal**: Data providers submit their streams for consideration as system streams.
2. **Audit**: TN governance thoroughly audits the stream's data sources, methodology, and contract logic.
3. **Community Review**: The TN community reviews and provides feedback on proposed system streams.
4. **Acceptance**: Upon passing audits and community review, streams are accepted as system streams.
5. **Ongoing Monitoring**: System streams are subject to continuous monitoring and periodic reviews to maintain their status.

By leveraging these different stream types and understanding their characteristics, developers and data providers can create powerful, composable data solutions within the TN ecosystem.
