package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/kwilteam/kwil-db/core/crypto"
    "github.com/kwilteam/kwil-db/core/crypto/auth"
    "github.com/trufnetwork/sdk-go/core/tnclient"
)

func main() {
    ctx := context.Background()

    // 1. Configure your signer (replace with your private key!)
    pk, err := crypto.Secp256k1PrivateKeyFromHex("your-private-key")
    if err != nil {
        log.Fatalf("failed to parse private key: %v", err)
    }
    signer := &auth.EthPersonalSigner{Key: *pk}

    // 2. Connect to a TN gateway (local or mainnet)
    endpoint := "https://gateway.mainnet.truf.network" // or http://localhost:8484
    tnClient, err := tnclient.NewClient(ctx, endpoint, tnclient.WithSigner(signer))
    if err != nil {
        log.Fatalf("failed to create TN client: %v", err)
    }

    // 3. Load the generic Action API
    actions, err := tnClient.LoadActions()
    if err != nil {
        log.Fatalf("failed to load Action API: %v", err)
    }

    // ---------------------------------------------------------
    // Example: call a read-only stored procedure with arguments
    // ---------------------------------------------------------

    // This example calls the `get_divergence_index_change` procedure that
    // expects the following positional arguments:
    // 1. $from        INT  — starting unix timestamp (inclusive)
    // 2. $to          INT  — ending unix timestamp (inclusive)
    // 3. $frozen_at   INT? — optional timestamp when the stream was frozen
    // 4. $base_time   INT? — optional base time for index normalisation
    // 5. $time_interval INT — comparison interval in seconds

    from := int(time.Now().AddDate(0, 0, -7).Unix()) // one week ago
    to := int(time.Now().Unix())                      // now
    timeInterval := 31_536_000                        // one year in seconds

    // nil placeholders are used for optional parameters we want to skip
    args := []any{from, to, nil, nil, timeInterval}

    queryResult, err := actions.CallProcedure(ctx, "get_divergence_index_change", args)
    if err != nil {
        log.Fatalf("procedure call failed: %v", err)
    }

    // Print the returned rows in a simple CSV-like format
    fmt.Printf("Columns: %v\n", queryResult.ColumnNames)
    for _, row := range queryResult.Values {
        fmt.Println(row)
    }
} 