# Relay Controller Phase Two

## Scope

Relay Controller owns:

- namespace account and trial plan;
- monthly quota state;
- Tunnel and Port metadata quotas;
- ten-minute usage settlement;
- `GET /limits`;
- Gateway tunnel status ingestion and control decisions;
- quota checks during token issuance.

Relay Gateway owns:

- traffic byte counting at the Host side;
- one-Host Redis lock;
- bandwidth, HTTP request, and connection token buckets;
- cumulative usage-window writes and shutdown flush;
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

Gateway writes one cumulative row for each Host session and ten-minute window. It writes every 30 seconds and once more when the session closes.

```text
windowStart = floor(reportTimestamp / 600) * 600
unique key  = tunnelId + sessionId + windowStart
usageBytes  = cumulative bytes within that window
```

The write must be idempotent:

```sql
INSERT INTO tunnel_usage_window (
    account_id,
    cluster_id,
    tunnel_id,
    session_id,
    window_start,
    usage_bytes,
    billed_bytes,
    reported_at
)
SELECT
    t.account_id,
    t.cluster_id,
    t.tunnel_id,
    ?,
    ?,
    ?,
    0,
    ?
FROM tunnel t
WHERE t.tunnel_id = ?
  AND t.cluster_id = ?
  AND t.deleted = 0
ON DUPLICATE KEY UPDATE
    usage_bytes = GREATEST(usage_bytes, VALUES(usage_bytes)),
    reported_at = GREATEST(reported_at, VALUES(reported_at));
```

Selecting `account_id` from `tunnel` prevents a caller-supplied account mismatch. Gateway must verify that one row was inserted or updated and must not increment `billed_bytes`.

The scheduled Controller job uses a conditional update from the previously billed value to the observed cumulative value. Only the replica that wins that update adds the delta to `billing_period` and `billing_usage_10m`. A later larger cumulative report produces only another delta.

## Quota Decisions

Internally, the current balance is calculated as:

```text
usedBytes = billing_period.billed_bytes
          + SUM(tunnel_usage_window.usage_bytes - tunnel_usage_window.billed_bytes)

remainingBytes = max(0, quotaBytes - usedBytes)
```

Including pending bytes prevents a ten-minute settlement delay from allowing new tokens after the known quota is exhausted.

Existing JWTs remain valid cryptographically until expiration. Gateway must check quota when accepting a connection and obey `disconnect` from the next status response. A ten-second heartbeat still permits a small overrun, so strict byte-level cutoff belongs in Gateway.

## Tunnel Status

Gateway calls:

```text
POST /open-api-inner/v1/relay-controller/clusters/{clusterId}/tunnels/status
```

The batch reports Host/client connections, channel count, current rates, session ID, and report time. Controller stores only the latest row per Tunnel and returns:

- `keep` for an active, funded Tunnel;
- `disconnect` for missing, mismatched, expired, disabled, over-quota, or over-Host-limit state;
- current remaining bytes and data-plane limits when the account is available;
- `nextReportInSeconds=10`.

Status telemetry is operational state, not billing truth. Active status extends Tunnel inactivity expiration with a five-minute write granularity.

## Cookie Token

`forCookies=true` adds `delivery=cookie` to the same signed JWT. It does not use private-key encryption. The component serving the user-facing domain sets the token cookie with `Secure`, `HttpOnly`, and an appropriate `SameSite` and domain policy.

The JWT audience is `relay-gateway`. Gateway must validate `aud`, signature, expiration, Tunnel, cluster, scope, and delivery policy.

## Compatibility

Phase-one APIs and response bodies remain available. The old `/clusters/{clusterId}/metering` endpoint is not included in monthly settlement. Gateway must not report the same traffic through both mechanisms.
