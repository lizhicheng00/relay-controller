# Relay Controller Business And Code Summary

## 1. Architecture

The project uses a small layered structure:

- `interfaces`: generated OpenAPI contracts, controllers, request/response models, rate limiting;
- `application`: tunnel, port, account limits, settlement, cluster, and cleanup workflows;
- `domain`: business models, enums, repositories, and validation services;
- `infrastructure`: MySQL persistence, JWT signing, and configuration.

OpenAPI source is `src/main/resources/static/openapi.yaml`. Maven generates Spring interfaces under `target/generated-sources/openapi`; controllers only implement those interfaces.

## 2. Core Data

### Tunnel

Important fields are `tunnelId`, `tunnelCode`, `clusterId`, `namespace`, `expiration`, `expirationHours`, `type`, `url`, and `deleted`. `expirationHours` is the inactivity window; `expiration` is its current Unix deadline. `portCount` is a list-query projection and is not stored in the `tunnel` table.

Active tunnel list SQL filters namespace, region, soft-delete state, and expiration in the database. It calculates `portCount` with one indexed correlated count, avoiding an N+1 query.

### Tunnel Port

`tunnel_port` uses `(tunnel_code, port)` as its unique business key. `protocol` is persisted as `http`, `https`, or `auto`; `allow_anonymous` controls sending-side access.

The public collection supports create and list only. Repository-level `deleteByTunnelCode` remains internal for tunnel deletion and aging cleanup.

## 3. Main Flows

### Create Tunnel

1. Require `X-Namespace`.
2. Verify the requested cluster belongs to the local region.
3. Create or lock the namespace billing account and enforce the active tunnel quota in the database transaction.
4. Resolve the expiration duration and cap it at 720 hours.
5. Allocate a unique 40-bit code and Base32 tunnel ID.
6. Persist and return metadata without issuing tokens.

### List Tunnels

The repository returns only active local-region rows and computes `portCount`. Expired or soft-deleted tunnels do not appear.

### Issue Token

1. Verify namespace ownership, local region, and expiration.
2. Validate `scope` as `host` or `connect`.
3. Set expiration from the fixed configured token TTL.
4. Add `aud=relay-gateway`, a delivery claim, and a random `jti`, then sign a new RS256 JWT.
5. Return `tunnelId`, `scope`, `lifetime`, `expiration`, and `token`.

No cache is read or written. This makes each call independent and removes Redis from the runtime architecture.

### Port Policy

Create and update use the `TunnelProtocol` enum from request through persistence. This prevents unsupported protocol strings from entering the domain or database. Gateway policy lookup additionally verifies the caller's cluster against the tunnel. Successful port mutations refresh the tunnel inactivity window.

### Tunnel Activity

Successful tunnel and port changes refresh `tunnelExpiration`. Gateway directly refreshes active Tunnel expiration with a five-minute write granularity. Reads and token issuance do not refresh it.

## 4. API Summary

```text
POST   /open-api-inner/v1/relay-controller/tunnels
GET    /open-api-inner/v1/relay-controller/tunnels
DELETE /open-api-inner/v1/relay-controller/tunnels
GET    /open-api-inner/v1/relay-controller/tunnels/{tunnelId}
PUT    /open-api-inner/v1/relay-controller/tunnels/{tunnelId}
DELETE /open-api-inner/v1/relay-controller/tunnels/{tunnelId}
POST   /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/token?scope=host|connect

POST   /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/ports
GET    /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/ports
GET    /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/ports/{port}
PUT    /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/ports/{port}
DELETE /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/ports/{port}

GET    /open-api-inner/v1/relay-controller/clusters/{clusterId}/tunnels/{tunnelId}/ports/{port}
GET    /open-api-inner/v1/relay-controller/limits
```

## 5. Persistence And Lifecycle

Flyway `V1` creates the phase-one tables. The consolidated `V3` adds the final phase-two plans, accounts, UTC monthly periods, one-minute usage and billing windows, runtime status, and the Tunnel account binding. Compound database names use snake_case while Java fields use camelCase.

Tunnel expiration is refreshed by meaningful configuration changes and direct Gateway activity updates. Explicit deletion removes the Tunnel, its Port rows, and runtime status in one transaction; scheduled cleanup does the same for aged Tunnels.

## 6. Runtime Configuration

Required configuration:

- `DATASOURCE_URL`
- `DATASOURCE_USERNAME`
- `DATASOURCE_PASSWORD`
- `RELAY_REGION`
- `RELAY_DOMAIN`
- `RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE`
- `RELAY_JWT_PRIVATE_KEY` for stable production signing

mTLS additionally requires the server keystore and client-CA truststore variables documented in `README.md`. Redis configuration is intentionally absent.

## 7. Verification

Tests cover API mappings and errors, database-scoped metadata quotas, monthly balance, idempotent settlement claims, runtime detail, expiration handling, token claims and uniqueness, port protocol behavior, rate limiting, and aged tunnel cleanup.
