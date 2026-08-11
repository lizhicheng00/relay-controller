# Relay Controller User Story

## 1. Service Boundary

Relay Controller is the control plane for one configured region. It owns tunnel metadata, tunnel port policies, JWT issuance, and billing settlement. Relay Gateway owns traffic forwarding, metering writes, and runtime connection state.

All user-facing APIs receive `X-Namespace` as the resource boundary and `X-Account-Namespace` as the shared quota owner. A tunnel is usable only when its resource namespace matches, its cluster belongs to the controller's configured region, and it has not expired.

## 2. Tunnel

As a DevBridge user, I can:

- create a tunnel in a local cluster;
- list my active tunnels, optionally filtered by `clusterId`;
- read or update one tunnel;
- delete one tunnel or all tunnels in my namespace;
- request a host or connect token for an active tunnel.

Business rules:

- `tunnelCode` is a random 40-bit positive `long`.
- `tunnelId` is the fixed eight-character lowercase Base32 encoding of `tunnelCode`.
- tunnel URL is `{tunnelId}.{clusterId}.{relay.domain}`.
- default expiration is 72 hours and the maximum is 720 hours.
- create and update requests use `expiration`; tunnel responses return the inactivity window as `expirationHours` and its current Unix deadline as `tunnelExpiration`.
- tunnel expiration is extended by successful tunnel or port changes and positive metering, using the latest reported activity time.
- an account namespace owns at most 10 active tunnels across its resource namespaces by default.
- list returns only non-deleted, non-expired tunnels and includes `portCount`.
- explicit delete physically removes tunnel metadata and its port policies.
- expired or deleted tunnels cannot issue tokens or accept port operations.

## 3. Token

The client calls:

```text
POST /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/token?scope=host|connect
```

The response contains:

```json
{
  "tunnelId": "aaaadysa",
  "scope": "host",
  "lifetime": 3600,
  "expiration": 1720086400,
  "token": "eyJ..."
}
```

Every call signs a new JWT. Tokens are not cached. `lifetime` is the fixed configured token TTL and does not follow the tunnel expiration. JWT claims include `iss`, `aud`, `exp`, `nbf`, `jti`, `tunnelId`, `clusterId`, and `scp`. Token issuance checks the current monthly balance.

## 4. Tunnel Port

As a user, I can create, list, read, update, and delete one tunnel port policy.

Each policy contains:

- `port`: 1 through 65535;
- `protocol`: `http`, `https`, or `auto`;
- `allowAnonymous`: whether sending-side anonymous access is allowed.

There is no public delete-all port endpoint. Deleting a tunnel and the aging job still delete all related port rows internally.

Gateway reads tunnel and port policy from the shared database. Public port operations still verify namespace ownership, local region, and tunnel activity before returning or changing a policy.

Gateway writes the latest Host activity directly to the shared runtime-status table. Tunnel detail returns it as a `status` object containing Host connection count, client connection count measured as active SSH channels, current upload/download rates, cumulative upload/download bytes, and report time. Read operations and token issuance do not extend the tunnel lifetime.

## 5. Billing And Aging

Gateway appends idempotent incremental usage directly to the shared database. Relay Controller settles each report once into monthly and Tunnel totals; it does not expose a metering HTTP endpoint.

Expired tunnels remain stored for the configured retention period but cannot be used. The hourly cleanup job hard-deletes aged tunnel metadata, port policies, and runtime status in bounded batches.

## 6. Acceptance Criteria

- Namespace and region boundaries are enforced before returning or mutating business data.
- Concurrent tunnel creation cannot exceed the account-namespace quota across Controller replicas sharing the database.
- Invalid request values return 4xx responses; unexpected failures return 5xx responses.
- OpenAPI YAML is the external contract; HTTP contract tests keep the Go routes and response shapes aligned with it.
- Relay Controller itself does not depend on Redis. Gateway uses Redis for the distributed single-Host lock.
