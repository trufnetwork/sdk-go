# Index-Change Market Example

Creates a set of prediction markets that settle on **how far an index moved**, rather than on the
level it published.

## Why this action exists

A stream that publishes an index *level* — CPI at ~335, PCE at ~131 — cannot back an inflation-rate
market through the value actions in migration 040. Those compare a level against a rate, and the two
are not the same number. `get_index_change` computes the rate but returns a series, and multi-row
actions cannot be attested.

`index_change_in_range` computes the same percentage and returns a single boolean, so a market can
be struck on the *rate* while reading a stream that only publishes *levels*.

## What the example does

1. Reads the stream's current year-over-year change, so you can see the number such a market settles
   against. This is a plain read and costs nothing.
2. Builds three buckets and decodes them locally, without touching the network. This is where the
   encoding contract is easiest to see.
3. Creates the same three markets on testnet.

## Buckets, bounds and open tails

The three buckets share an observation time and an interval and differ only in where they are
struck, so exactly one of them can settle YES:

| Question | `min_change` | `max_change` |
|---|---|---|
| below 2% | `nil` | `"2"` |
| between 2% and 3% | `"2"` | `"3"` |
| 3% or more | `"3"` | `nil` |

Two things follow from that table.

**Bounds are half-open, `[min, max)`.** A change landing exactly on a boundary belongs to the bucket
*above* it. That is what lets a set tile the number line without two buckets settling YES at once.

**`nil` strikes an open tail**, which is how the outer two buckets are always struck. Passing `nil`
for both is rejected — it would be every outcome at once.

An open tail decodes back as an **empty string holding its slot**, not as a shorter list:

```text
below 2%             thresholds=["" "2.000000000000000000"]
3% or more           thresholds=["3.000000000000000000" ""]
```

Dropping the empty entry would slide the surviving bound into the other position and turn
"below 2%" into "2% and up". `BucketBoundsFromMarketData` reads both slots and returns `nil` for the
open side.

## Prerequisites

- Go 1.24 or later
- Nothing else. The example points at `https://gateway.testnet.truf.network` and ships with a
  throwaway testnet wallet.

## Running it

```bash
go run ./examples/index_change_market_example
```

To create the markets from your own wallet instead:

```bash
PRIVATE_KEY=your_private_key_here go run ./examples/index_change_market_example
```

## Cost

Each market costs a **2 TRUF creation fee**, so a run of this example costs 6 TRUF. The fee is
always taken from the wallet's `hoodi_tt` balance, whatever bridge the market uses for collateral —
this example collateralises in `hoodi_tt2`.

The bundled wallet is `0x32a46917DF74808b9aDD7DC6eF0c34520412FDF3`, the same OBMarketCreator wallet
the sdk-py order-book examples use. It is a **throwaway testnet key**: do not use it in production
or hold anything of value in it. If it runs out of TRUF, pass your own `PRIVATE_KEY`.

## Expected output

```text
=== Index-change prediction markets ===
Endpoint: https://gateway.testnet.truf.network
Wallet:   0x32a46917df74808b9add7dc6ef0c34520412fdf3

--- What such a market measures ---
Stream st9f212b7c208afd83705cc0dbdadfe8 moved -33.659022158930734676% over the year ending 2026-07-06.

--- The bucket set, built and decoded locally ---
  below 2%             type=change_between thresholds=["" "2.000000000000000000"] bounds=[open, 2)
  between 2% and 3%    type=change_between thresholds=["2.000000000000000000" "3.000000000000000000"] bounds=[2, 3)
  3% or more           type=change_between thresholds=["3.000000000000000000" ""] bounds=[3, open)

--- Creating the set on testnet (2 TRUF each) ---
  below 2%             tx 27af027eed365c8a91b2f918782a7a90900822dd31fa39c0af823b1a241c16a0
  between 2% and 3%    tx 3cec60478755049f14c1c0a61230d1de2fa5c3aae7c9d6a75a6ee6abcce1ff81
  3% or more           tx 87530270f36c646e9e68333f3cea6867e5d6f04317952ff74bcca3ba6e6bcb99

Settles at 2026-08-26 13:11:57 UTC.
```

## Notes

- Each run uses a fresh settlement time. A market's identity is a hash of its query components, so
  re-creating an identical market is refused rather than duplicated.
- The market observes the stream at its settlement time, so it cannot be resolved before then.
- `index_change_in_range` cannot be called read-only: it goes through
  `validate_not_before_timestamp`, which needs a writer connection. Reading a truth value means
  requesting an attestation. Every binary attestation action behaves this way, not just this one.
- To read markets back off the network, see [`../decode_market_example`](../decode_market_example).
