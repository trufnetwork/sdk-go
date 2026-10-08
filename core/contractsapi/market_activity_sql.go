package contractsapi

// marketActivitySQL defines a market's filled volume and unique traders over a
// time window. GetMarketActivity runs it as one ad hoc read-only query, binding
// $query_id, $from_ts and $to_ts.
//
// The definition's home is the parent Goal,
// https://github.com/trufnetwork/node/issues/1429. Volume counts each trade
// once, in cents of the market's own collateral, over an allowlist of fill
// events, so an event type added later stays out until someone decides it is
// volume. sdk-js carries the same text, and sdk-py runs this copy through its Go
// bindings: a change here belongs in sdk-js too. If a node action replaces the
// statement, GetMarketActivity is the one place that changes.
//
// The ::NUMERIC(78,0) casts are load-bearing. Without them Kwil's planner rejects
// COALESCE(SUM(...), 0) as a numeric/int8 mismatch.
const marketActivitySQL = `
SELECT
  q.bridge,
  COALESCE(SUM(CASE WHEN e.event_type = 'direct_buy_fill' THEN e.price * e.amount
                    WHEN e.event_type IN ('mint_fill', 'burn_fill') AND e.outcome = TRUE THEN 100 * e.amount
                    ELSE 0 END)::NUMERIC(78,0), 0::NUMERIC(78,0))                                   AS volume_cents,
  COALESCE(SUM(CASE WHEN e.event_type = 'direct_buy_fill' THEN e.price * e.amount ELSE 0 END)::NUMERIC(78,0), 0::NUMERIC(78,0)) AS direct_cents,
  COALESCE(SUM(CASE WHEN e.event_type IN ('mint_fill', 'burn_fill') AND e.outcome = TRUE THEN 100 * e.amount ELSE 0 END)::NUMERIC(78,0), 0::NUMERIC(78,0)) AS mint_burn_cents,
  COUNT(DISTINCT CASE WHEN e.event_type IN ('direct_buy_fill', 'direct_sell_fill', 'mint_fill', 'burn_fill') THEN e.participant_id END)::INT AS unique_traders,
  COUNT(CASE WHEN e.event_type = 'direct_buy_fill' OR (e.event_type IN ('mint_fill', 'burn_fill') AND e.outcome = TRUE) THEN 1 END)::INT AS fill_count,
  COUNT(CASE WHEN e.event_type = 'direct_buy_fill' THEN 1 END)::INT AS direct_fill_count,
  COALESCE(SUM(CASE WHEN e.event_type = 'direct_buy_fill' OR (e.event_type IN ('mint_fill', 'burn_fill') AND e.outcome = TRUE) THEN e.amount ELSE 0 END)::INT8, 0::INT8) AS shares_traded,
  MIN(CASE WHEN e.event_type IN ('direct_buy_fill', 'direct_sell_fill', 'mint_fill', 'burn_fill') THEN e.block_timestamp END) AS first_event_ts,
  MAX(CASE WHEN e.event_type IN ('direct_buy_fill', 'direct_sell_fill', 'mint_fill', 'burn_fill') THEN e.block_timestamp END) AS last_event_ts,
  (SELECT MIN(block_height) FROM main.ob_order_events) AS coverage_from_block,
  ((SELECT MIN(block_height) FROM main.ob_order_events) <= q.created_at) AS coverage_complete
FROM main.ob_queries q
LEFT JOIN main.ob_order_events e
  ON e.query_id = q.id AND e.block_timestamp >= $from_ts AND e.block_timestamp <= $to_ts
WHERE q.id = $query_id
GROUP BY q.id, q.bridge, q.created_at`
