# Relay Controller

Relay Controller is the regional DevBridge control plane. It manages tunnels, port policies, namespace accounts, limits, JWT issuance, billing settlement, runtime-status queries, and expired-resource cleanup. Relay Gateway owns traffic forwarding and writes metering and runtime status directly to the shared database.

## Business Rules

- One process serves one `RELAY_REGION`; only clusters registered to that region are accepted.
- A `devbridge` API key resolves to one namespace and its quota-sharing account namespace through Management Service.
- A trial account has 5 GiB per Beijing calendar month, 10 active tunnels, and 10 ports per tunnel.
- `tunnelCode` is a random positive 40-bit integer. `tunnelId` is its fixed eight-character lowercase Base32 encoding.
- Tunnel URLs use `{tunnelId}.{clusterId}.{RELAY_DOMAIN}`.
- Tunnel names are unique within a namespace.
- Tunnel inactivity defaults to 72 hours and is capped at 720 hours. Configuration changes and positive settled metering refresh the deadline.
- Every token request creates a new RS256 JWT with `aud=relay-gateway`; tokens are never cached.
- Gateway appends metering rows. Controller settles each row exactly once into monthly and tunnel totals.
- Explicit deletion and aged cleanup physically remove tunnel metadata, port policies, and runtime status. Raw metering is retained by hourly partition policy for billing audit.

## API

All business APIs use the prefix `/open-api-inner/v1/relay-controller` and require `X-API-Key`.

```text
GET    /auth/check
POST   /tunnels
GET    /tunnels?clusterId=
DELETE /tunnels
GET    /tunnels/{tunnelId}
PUT    /tunnels/{tunnelId}
DELETE /tunnels/{tunnelId}
POST   /tunnels/{tunnelId}/token?scope=host|connect

POST   /tunnels/{tunnelId}/ports
GET    /tunnels/{tunnelId}/ports
GET    /tunnels/{tunnelId}/ports/{port}
PUT    /tunnels/{tunnelId}/ports/{port}
DELETE /tunnels/{tunnelId}/ports/{port}

GET    /limits
```

`GET /auth/check` performs the same API key and scope validation as every business request and returns `204` without querying tunnel or billing data. CLI clients use it once when accepting a manually supplied API key; normal commands call their business endpoint directly.

## Structure

```text
cmd                    process startup and graceful shutdown
internal/httpapi       HTTP routing, JSON errors, recovery, rate limiting
internal/auth          API key resolution through Management Service
internal/service       tunnel, port, token, billing, cleanup workflows
internal/core          business models and deterministic rules
internal/store         MySQL queries and transactions
internal/security      RS256 signing
migrations             embedded database migrations
```

The runtime uses the Go standard library where practical.

## Configuration

Environment variables:

| Variable | Meaning |
| --- | --- |
| `SERVER_ADDRESS` | HTTP listen address, default `:8443` |
| `MGMT_SERVICE_URL` | Management Service internal HTTPS address, default `https://127.0.0.1:8444` |
| `MGMT_SERVER_NAME` | TLS SNI and certificate name, default `mgmt.developer.myhuaweicloud.com` |
| `MGMT_CLIENT_CERT_BASE64` | Base64-encoded client certificate PEM used for Management Service mTLS |
| `MGMT_CLIENT_KEY_BASE64` | Base64-encoded client private-key PEM used for Management Service mTLS |
| `MGMT_CLIENT_KEY_PASSWORD` | Client-key password when the key is encrypted PKCS#8; omit for an unencrypted key |
| `MGMT_CA_CERT_BASE64` | Optional Base64-encoded PEM CA for the Management Service certificate; otherwise the issuer bundled in `MGMT_CLIENT_CERT_BASE64` and system roots are used |
| `DATABASE_DSN` | MySQL DSN, for example `user:password@tcp(host:3306)/database` |
| `RELAY_CONFIG_DOG_FILE` | Dog key-component file, default `/opt/cloud/dog/beta` |
| `RELAY_CONFIG_PIG` | Base64-encoded 32-byte pig key component, required for `ENC(...)` values |
| `RELAY_REGION` | Region owned by this instance, default `cn-north-4` |
| `RELAY_DOMAIN` | Tunnel DNS suffix, default `myhuaweicloud.com` |
| `RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE` | API requests allowed per namespace and process each minute, default `120` |
| `RELAY_JWT_PRIVATE_KEY` | PKCS8 RSA private key, PEM or Base64 DER, at least 2048 bits |

Relay Controller serves HTTP only. A gateway or private ingress owns public HTTPS and forwards `X-API-Key`. Bind the service to loopback or a private interface; never expose its HTTP listener directly to the public network.

`DATABASE_DSN`, `RELAY_JWT_PRIVATE_KEY`, `MGMT_CLIENT_KEY_BASE64`, and `MGMT_CLIENT_KEY_PASSWORD` accept either plaintext or an `ENC(v1.<nonce>.<ciphertext+tag>)` value. The AES-256-GCM working key is reconstructed from dog in a read-only file, cat embedded in both services, and pig in runtime configuration. Each component is independently generated Base64-encoded 32-byte data. Dog and pig are only required when an encrypted value is present. Base64 alone is encoding, not encryption.

## Build And Run

Go 1.24 or newer is required.

```bash
go test ./...
go vet ./...
go build -trimpath -ldflags '-s -w' -o bin/relay-controller ./cmd
./bin/relay-controller
```

For local development:

```bash
export DATABASE_DSN='relay_controller:<secret>@tcp(127.0.0.1:3306)/relay_controller?timeout=10s&readTimeout=30s&writeTimeout=30s'
export RELAY_REGION='cn-north-4'
export RELAY_DOMAIN='myhuaweicloud.com'
export RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE='120'
export RELAY_JWT_PRIVATE_KEY='<PKCS8 PEM or Base64 DER>'
export SERVER_ADDRESS='127.0.0.1:8443'
export MGMT_SERVICE_URL='https://127.0.0.1:8444'
export MGMT_SERVER_NAME='mgmt.developer.myhuaweicloud.com'
export MGMT_CLIENT_CERT_BASE64="$(base64 -w 0 client.crt)"
export MGMT_CLIENT_KEY_BASE64="$(base64 -w 0 client.key)"
export MGMT_CLIENT_KEY_PASSWORD='<secret>'
# Only required when the Management Service certificate uses a private CA.
# export MGMT_CA_CERT_BASE64="$(base64 -w 0 ca.crt)"
go run ./cmd
```

The service applies embedded migrations before opening the application store. Applied versions are recorded in `schema_migration`.

## Security Boundary

The caller supplies only `X-API-Key`. Relay Controller resolves it through Management Service over mTLS, requires `scope=devbridge`, and places the returned namespace identity in request context. Caller-supplied namespace headers are ignored. Missing or invalid credentials return `401`, an incompatible scope returns `403`, and a Management Service failure returns `503`.

The API key Check endpoint remains owned by Management Service because it owns key hashes and identity mappings. Relay Controller owns when that check is required. Public HTTPS terminates at the gateway; the Relay HTTP listener remains private. Relay authenticates itself to Management Service with its client certificate and verifies the Management Service certificate against an explicitly configured CA, the issuer bundled in the client certificate chain, or the system trust store.

Gateway enforces data-plane limits such as one Host per tunnel, bandwidth, HTTP request rate, and concurrent connections. The Controller's in-memory request limiter is only a bounded per-instance safety limit; strict cross-replica API limiting belongs at ingress.
