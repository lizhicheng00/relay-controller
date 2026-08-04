# Relay Controller Phase Two

## Scope

Relay Controller owns:

- namespace account and trial plan;
- monthly quota state;
- Tunnel and Port metadata quotas;
- one-minute usage settlement;
- `GET /limits`;
- Tunnel detail runtime status;
- quota checks during token issuance.

Relay Gateway owns:

- traffic byte counting at the Host side;
- one-Host Redis lock;
- bandwidth, HTTP request, and connection token buckets;
- incremental metering writes and shutdown flush;
- latest runtime-status writes;
- active Tunnel expiration refresh;
- immediate admission and disconnect execution.

CLI echo, ping, random port allocation, verbose logging, and local HTTP request printing are not Relay Controller code.

## Trial Plan

The `trial` row in `billing_plan` is the single source of truth:

| Limit | Value |
| --- | ---: |
| Monthly traffic | 5 GiB |
| Active tunnels | 10 |
| Ports per tunnel | 10 |
| Hosts per tunnel | 1 |
| Tunnel bandwidth | 5 MiB/s |
| HTTP requests per port | 500/min |
| Concurrent connections per port | 100 |

Monthly periods use UTC and are immutable rows. A new `billing_period` is created for each month; no scheduled reset mutates the previous month.

## Gateway Usage Contract

Gateway appends one incremental usage row every 30 seconds and once more when the Host session closes.

```text
unique key = tunnelId + sessionId + reportedAt
usageBytes = bytes since the previous successful report
```

An exact retry must keep the same `reportedAt` and `usageBytes`. Gateway allows at most one report per session in the same second and coalesces a session-end flush with a periodic report when necessary.

```sql
INSERT IGNORE INTO tunnel_metering (
    account_id,
    cluster_id,
    tunnel_id,
    session_id,
    usage_bytes,
    reported_at,
    created_at,
    settled
)
SELECT
    t.account_id,
    t.cluster_id,
    t.tunnel_id,
    ?,
    ?,
    ?,
    UNIX_TIMESTAMP(),
    0
FROM tunnel t
WHERE t.tunnel_id = ?
  AND t.cluster_id = ?
  AND t.deleted = 0;
```

The placeholders after `tunnel_id` are `session_id`, `usage_bytes`, and `reported_at`. Selecting account and cluster ownership from `tunnel` prevents caller-supplied ownership mismatches. Gateway retains the local byte count until this insert succeeds; a duplicate-key result is successful only for an exact retry.

Every minute Controller selects bounded local-Region batches where `settled = 0` using `FOR UPDATE SKIP LOCKED` until the current backlog is drained. Each batch produces account-period totals, Tunnel-minute usage, and Tunnel totals, then updates `billing_period`, `billing_usage_1m`, and Tunnel usage before marking the selected records settled. Each batch shares one transaction: either every aggregate and marker commits, or all records remain available for retry. `SKIP LOCKED` lets multiple Controller replicas work without charging the same row twice.

## Quota Decisions

Internally, the current balance is calculated as:

```text
usedBytes      = billing_period.billed_bytes
remainingBytes = max(0, quotaBytes - usedBytes)
```

Balance and Gateway enforcement use settled monthly state. This intentionally accepts up to one settlement interval of delay in exchange for one consistent quota source.

When a period reaches its quota, Controller advances `billing_account.quota_blocked_until` to that period's end in the same settlement transaction. The timestamp never moves backward, so late historical metering cannot clear a current block; it expires naturally at the next UTC month. Gateway only needs the Tunnel's account status and this timestamp when accepting a connection.

Existing JWTs remain valid cryptographically until expiration. Gateway must disconnect active traffic when the account becomes blocked.

Controller maintains UTC hourly `reported_at` partitions and keeps two future hours ready. A database lock ensures that only one Controller replica performs each maintenance run. Partitions older than seven days are dropped directly. Gateway must not replay metering older than seven days, and the Controller database account needs `ALTER` permission on `tunnel_metering`. The raw table remains a short operational audit buffer; `billing_usage_1m` and `billing_period` are the durable billing results.

## Tunnel Runtime Status

Gateway writes the latest runtime state directly to `tunnel_runtime_status`, using `tunnel_id` as the key. The write must verify that the Tunnel belongs to the reporting cluster and update values only when the incoming `reported_at` is not older than the stored value. `client_connection_count` is the active SSH channel count; a separate channel field is intentionally not stored. Controller exposes a compact Tunnel `status` object containing Host and client connection counts, current rates, and `reportedAt`.

Status telemetry is operational state, not billing truth. Controller refreshes `tunnel.expiration` while settling positive metering, using the latest `reported_at` and the configured `expiration_hours`. An open Host session without traffic and a zero-usage report are not activity. Gateway owns quota, one-Host, bandwidth, HTTP, and connection enforcement; there is no Controller status callback or control-decision response.

## Tunnel Token

The JWT audience is `relay-gateway`. Gateway must validate `aud`, signature, expiration, Tunnel, cluster, and scope.
