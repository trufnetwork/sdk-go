# MAA Lifecycle Smoke Test (Go)

A runnable, end-to-end proof of **Modular Agent Addresses** ("agent wallets") through the Go SDK,
against a live TRUF.NETWORK node where `maa_exec` is activated (testnet from height `6523123`).

An MAA lets a token holder (the **owner**) hand a constrained **agent** key a wallet it can *operate*
but provably cannot *drain*. This example drives the whole lifecycle and asserts the properties that
make that safe, mirroring the node's canonical oracle `tests/streams/maa/data_agent_test.go` and the
Python SDK's `examples/maa_lifecycle_example`.

## What it proves

1. **`@caller` rewrite** — the agent runs `create_streams` / `insert_records` *as the MAA*, so the
   stream it creates is owned by the **MAA address**, not the agent's own key. The example reads the
   record back under the MAA address to confirm.
2. **Fees debit the MAA's own escrow** — each action's fee comes out of the wallet's bridge balance
   (printed before/after every step).
3. **The agent cannot exfiltrate** — when the restricted agent attempts `maa_withdraw`, the node
   rejects it ("reserved for the unrestricted owner") and the escrow is unchanged.
4. **The owner withdraws with commission** — the owner drains the remaining escrow; the agent earns
   the rule's `fee_bps` commission.
5. **Local derivation matches the chain** — `rule_id` and the MAA address are derived offline
   (`util.ComputeRulesHash` / `util.DeriveRuleID` / `util.DeriveMAAAddress`) and asserted equal to what
   the SDK returns.
6. **Portfolio reads by address** — `GetPositionsByWallet` / `GetCollateralByWallet` (migration 051)
   read the agent wallet's order-book inventory *by address* — the signer is not the wallet being read
   — so an owner or a delegated market-maker bot can monitor an agent wallet it does not sign for. This
   MAA does data provision (not order-book trading), so the reads return empty/zero; a clean return is
   the proof that 051 is live.

## Two identities (required)

The agent and the owner are **different keys**:

| Role | Env var | Signs | Becomes |
|------|---------|-------|---------|
| **Restricted agent** | `AGENT_PRIVATE_KEY` | `CreateAgentRule`, runs allow-listed actions as the MAA | the `restricted` address baked into `rule_id` |
| **Unrestricted owner** | `OWNER_PRIVATE_KEY` | `JoinAgentAddress`, funds, withdraws | the `unrestricted` address; controls the funds |

## Run

Configuration is read from a `.env` file next to the program (real environment variables still take
precedence, so you can override any value with a shell `export`). `.env` is gitignored.

```bash
# from the repo root
cd examples/maa_lifecycle_example
cp .env.example .env        # then edit .env: fill in AGENT_PRIVATE_KEY and OWNER_PRIVATE_KEY

go run .
```

Each run uses a fresh salt by default, so it registers a new rule/MAA and can be re-run without
colliding with a previous run. Set `MAA_SALT` (64 hex chars) to pin a reproducible rule_id.

### Environment variables (see `.env.example`)

| Variable | Default | Notes |
|----------|---------|-------|
| `PROVIDER_URL` | `https://gateway.testnet.truf.network` | The testnet RPC/gateway. **Confirm the real URL for your network.** |
| `AGENT_PRIVATE_KEY` | — (required) | Restricted agent key (with or without `0x`). |
| `OWNER_PRIVATE_KEY` | — (required) | Unrestricted owner key (with or without `0x`); must hold ≥ `MAA_FUND_AMOUNT` + fees of bridged token. |
| `MAA_BRIDGE` | `hoodi_tt` | Funding/fee bridge namespace. dev = `hoodi_tt`/`hoodi_tt2`, mainnet = `eth_truf`/`eth_usdc`. |
| `MAA_FUND_AMOUNT` | `250000000000000000000` (250 TRUF) | Must cover the action fees (`create_streams` may cost 100 TRUF where the fee is active). |
| `MAA_FEE_BPS` | `250` (2.5%) | Owner-withdraw commission paid to the agent. |
| `MAA_COLLATERAL_BRIDGE` | `hoodi_tt2` | Order-book bridge for `GetCollateralByWallet` (migration 051) — the bridge the markets settle in, **not** the `hoodi_tt` fee bridge. |
| `MAA_SALT` | _(fresh per run)_ | Optional 64-hex salt to pin a reproducible rule_id across runs. |

## Open items to confirm before the first run

These can't be derived from code — set them for your testnet:

1. **Provider URL** — the public testnet RPC/gateway endpoint.
2. **Two funded keys** — the owner key in particular needs enough bridged token to fund the MAA.
3. **Bridge namespace** — which of `hoodi_tt` / `eth_truf` / … is registered on this network.
4. **Fee schedule** — whether the 100-TRUF `create_streams` fee is active (drives `MAA_FUND_AMOUNT`).
   The program reads balances around each step, so it works whether or not fees are active.

## NUMERIC arguments (no marker wrapper — unlike Python)

`ExecuteAgentAction` encodes the inner action's arguments with the same encoder used for ordinary
action calls (kwil `EncodeValue`), which carries a decimal's precision/scale natively. So a `NUMERIC`
argument is just a `*kwilTypes.Decimal` parsed with the action's **exact** precision/scale via
`ParseDecimalExplicit` — there is no JSON marker to wrap. (The Python SDK needs `MAANumericArg` only
because JSON has no decimal type.) The node does **not** coerce text to `NUMERIC`, so the
precision/scale must match the declared parameter (`maa_withdraw`'s `$amount NUMERIC(78,0)`,
`insert_records`' `$value NUMERIC(36,18)[]`).

```go
amount, _ := kwilTypes.ParseDecimalExplicit("110000000000000000000", 78, 0)
client.ExecuteAgentAction(ctx, types.MAAExecuteInput{
    MAAAddress: maa, Action: "maa_withdraw", Args: []any{"hoodi_tt", amount},
})

value, _ := kwilTypes.ParseDecimalExplicit("42.5", 36, 18)
client.ExecuteAgentAction(ctx, types.MAAExecuteInput{
    MAAAddress: maa, Action: "insert_records",
    Args: []any{[]string{provider}, []string{streamID}, []int64{eventTime}, []*kwilTypes.Decimal{value}},
})
```

## Transactions are asynchronous

SDK write methods return once the transaction enters the mempool, not when it commits. Each step that
depends on the previous one (join reads the rule on-chain; exec needs funded escrow) waits for
inclusion with `client.WaitForTx` and checks the result code before continuing.

## What success looks like

Each step prints a `✅`, balances move as expected, the agent's withdrawal is blocked, the owner's
succeeds, and the run ends with `✅ MAA lifecycle smoke test PASSED`. A non-zero exit or a missing `✅`
means a step failed — the raw node error is printed inline.

## Oracle

The Go equivalents exercised directly against the engine live in the node repo:
`tests/streams/maa/data_agent_test.go`, `withdraw_test.go`, `lp_vault_test.go`, and
`docs/modular-agent-addresses.md`.
