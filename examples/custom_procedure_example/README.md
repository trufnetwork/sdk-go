# Custom Procedure Example (Go)

This example demonstrates how to invoke an **arbitrary stored procedure** on the TRUF.NETWORK database from the Go SDK using the `CallProcedure` helper.  

## What You Will Learn

* How to initialise a `tnclient.Client`
* How to load the generic `Action` API
* How to call a read-only stored procedure that expects custom arguments
* How to inspect the raw `QueryResult` that is returned

## Procedure Used

For demonstration purposes we call the procedure `get_divergence_index_change`, which accepts the following **positional** arguments:

| Position | Name        | Type | Description                                    |
|----------|-------------|------|------------------------------------------------|
| 1        | `from`      | INT  | Unix timestamp (inclusive) marking the start   |
| 2        | `to`        | INT  | Unix timestamp (inclusive) marking the end     |
| 3        | `frozen_at` | INT? | *Optional* freeze timestamp                    |
| 4        | `base_time` | INT? | *Optional* base time for normalisation         |
| 5        | `time_interval` | INT | Comparison interval in seconds              |

Feel free to replace the procedure name and arguments with your own.

## Prerequisites

* Go 1.20+ installed
* A running TN gateway (local **or** `https://gateway.mainnet.truf.network`)
* A funded Ethereum-style private key to sign transactions

## How to Run

1. **Replace** `"your-private-key"` in `main.go` with your actual private key (never commit real keys!).
2. **Optionally** switch the `endpoint` value to your local node (`http://localhost:8484`).
3. In this directory run:

   ```bash
   go mod tidy
   go run .
   ```

4. You should see something similar to:

   ```text
   Columns: [event_time value]
   [1717503323 1.2345]
   [1717589723 1.5678]
   ...
   ```