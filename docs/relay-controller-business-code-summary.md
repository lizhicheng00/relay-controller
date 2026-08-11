# Relay Controller Business And Code Summary

## 1. Architecture

The Go service keeps four business boundaries:

- `httpapi` owns the OpenAPI-facing HTTP contract, validation entry points, error responses, recovery, and the bounded local rate limiter.
- `service` coordinates tunnel, port, token, account, billing, and lifecycle transactions.
- `core` contains business models and pure rules such as identifiers, Base32, expiration, and Beijing billing periods.
- `store` owns SQL, transactions, Flyway-compatible migration history, row locking, and metering partitions.

`security` loads the PKCS8 RSA signing key and PKCS12 mTLS stores. `assets` embeds the OpenAPI document and unchanged migrations into the binary.

## 2. Tunnel Flow

Creation validates both namespace headers, confirms that `clusterId` belongs to the configured region, creates the parent billing account when needed, and locks that account before checking the shared tunnel quota. It then generates one random 40-bit code and its eight-character Base32 ID, persists the tunnel, and returns metadata.

List filtering happens in SQL by resource namespace, local region, active expiration, and deletion state. `portCount` is calculated in the same query. Detail additionally reads the latest Gateway-written runtime status.

Update and port mutations refresh the configured inactivity window. Positive metering refreshes it from the latest report timestamp during settlement. Reads, token issuance, open idle connections, and zero-byte reports do not count as activity.

Explicit deletion and scheduled aging remove the tunnel, its ports, and runtime status in one transaction. Metering remains until its audit partitions expire so already reported traffic is not lost.

## 3. Port And Token Flow

A tunnel port is unique by `(tunnel_code, port)`. Create requires `protocol` (`http`, `https`, or `auto`) and `allowAnonymous`; update accepts either field independently. The account row is locked while checking the per-tunnel port quota, so replicas cannot exceed it through concurrent requests.

Token issuance requires an active owned tunnel and available monthly quota. Every request signs a fresh RS256 token containing `iss`, `aud`, `exp`, `nbf`, `jti`, `tunnelId`, `clusterId`, and `scp`. Its fixed lifetime is independent of tunnel expiration and the response is marked `Cache-Control: no-store`.

## 4. Billing Flow

Gateway appends upload and download deltas to `tunnel_metering`. The unique report key makes an exact retry idempotent. Each minute Controller replicas claim different unsettled rows with database row locks, aggregate both directions into the Beijing calendar-month period and tunnel total, refresh activity, update quota blocking, and mark those rows settled in one transaction.

`billing_period` is the durable monthly result. Raw metering is a seven-day audit buffer split into UTC hourly partitions. One database scheduler lock serializes partition DDL across replicas; tunnel cleanup itself uses bounded `SKIP LOCKED` batches and may run concurrently.

## 5. Reliability And Security

- Database transactions enforce account tunnel quotas, port quotas, and exactly-once settlement across replicas.
- Region and namespace checks happen before business data is returned or changed.
- Invalid input returns 4xx; unexpected failures return a stable 5xx body while the server logs the cause and stack.
- mTLS rejects callers without a certificate signed by the configured client CA.
- JWT startup fails when the configured key is not valid PKCS8 RSA or is weaker than 2048 bits.
- Graceful shutdown stops HTTP intake and waits for scheduler goroutines.
- Redis is intentionally outside this service; Gateway owns distributed data-plane locks and limits.

The API contract is [assets/openapi.yaml](../assets/openapi.yaml). Runtime configuration and commands are in [README.md](../README.md).
