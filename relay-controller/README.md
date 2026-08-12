# Relay Controller

Relay Controller is the regional DevBridge control plane. It manages tunnels, port policies, namespace accounts, limits, JWT issuance, billing settlement, runtime-status queries, and expired-resource cleanup. Relay Gateway owns traffic forwarding and writes metering and runtime status directly to the shared database.

## Business Rules

- One process serves one `RELAY_REGION`; only clusters registered to that region are accepted.
- `X-Namespace` isolates resources. `X-Account-Namespace` identifies the parent account that shares quota across its namespaces.
- A trial account has 5 GiB per Beijing calendar month, 10 active tunnels, and 10 ports per tunnel.
- `tunnelCode` is a random positive 40-bit integer. `tunnelId` is its fixed eight-character lowercase Base32 encoding.
- Tunnel URLs use `{tunnelId}.{clusterId}.{RELAY_DOMAIN}`.
- Tunnel inactivity defaults to 72 hours and is capped at 720 hours. Configuration changes and positive settled metering refresh the deadline.
- Every token request creates a new RS256 JWT with `aud=relay-gateway`; tokens are never cached.
- Gateway appends metering rows. Controller settles each row exactly once into monthly and tunnel totals.
- Explicit deletion and aged cleanup physically remove tunnel metadata, port policies, and runtime status. Raw metering is retained by hourly partition policy for billing audit.

## API

All business APIs use the prefix `/open-api-inner/v1/relay-controller` and require both namespace headers.

```text
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

## Structure

```text
cmd                    process startup and graceful shutdown
internal/httpapi       HTTP routing, JSON errors, recovery, rate limiting
internal/service       tunnel, port, token, billing, cleanup workflows
internal/core          business models and deterministic rules
internal/store         MySQL queries and transactions
internal/security      RS256 signing and PKCS12 mTLS
migrations             reserved for deployment-managed schema changes
```

The runtime uses the Go standard library where practical. The only direct dependencies are the MySQL driver and PKCS12 decoder.

## Configuration

Required environment variables:

| Variable | Meaning |
| --- | --- |
| `DATASOURCE_URL` | `jdbc:mariadb://host:3306/database` or `jdbc:mysql://...` |
| `DATASOURCE_USERNAME` | Database user |
| `DATASOURCE_PASSWORD` | Database password |
| `RELAY_REGION` | Region owned by this instance |
| `RELAY_DOMAIN` | Tunnel DNS suffix |
| `RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE` | API requests allowed per namespace and process each minute |
| `RELAY_JWT_PRIVATE_KEY` | PKCS8 RSA private key, PEM or Base64 DER, at least 2048 bits |
| `SERVER_SSL_KEY_STORE_BASE64` | Base64 PKCS12 server key store |
| `SERVER_SSL_KEY_STORE_PASSWORD` | Server key store password |
| `SERVER_SSL_TRUST_STORE_BASE64` | Base64 PKCS12 client CA trust store |
| `SERVER_SSL_TRUST_STORE_PASSWORD` | Trust store password |
| `RELAY_CONFIG_DECRYPTION_KEY` | Base64 32-byte key; required when an encrypted value is used |

The HTTPS server listens on port `8443`.

The key store must be PKCS12 and contain the server key and certificate chain. The trust store must be PKCS12 and contain the accepted client CA. TLS 1.2 and 1.3 are enabled, and a trusted client certificate is mandatory.

`DATASOURCE_PASSWORD`, `RELAY_JWT_PRIVATE_KEY`, and both TLS passwords accept either plaintext or an authenticated value in `ENC(v1.<nonce>.<ciphertext>)` format. Production should inject encrypted values and supply `RELAY_CONFIG_DECRYPTION_KEY` separately. Generate and retain the key in a secret manager:

```bash
export RELAY_CONFIG_DECRYPTION_KEY="$(openssl rand -base64 32)"
printf %s '<secret>' | go run ./cmd encrypt-secret DATASOURCE_PASSWORD
```

The command prints the value to assign to the named configuration. Repeat it with the corresponding configuration name for each secret. AES-256-GCM binds ciphertext to that name, so encrypted values cannot be exchanged between fields. Do not store the decryption key beside the encrypted configuration; Base64 alone is not encryption.

## Build And Run

Go 1.24 or newer is required.

```bash
go test ./...
go vet ./...
go build -trimpath -ldflags '-s -w' -o bin/relay-controller ./cmd
./bin/relay-controller
```

For local development, provide the same mTLS configuration used in deployment:

```bash
export DATASOURCE_URL='jdbc:mariadb://127.0.0.1:3306/relay_controller'
export DATASOURCE_USERNAME='relay_controller'
export DATASOURCE_PASSWORD='<secret>'
export RELAY_REGION='cn-north-4'
export RELAY_DOMAIN='myhuaweicloud.com'
export RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE='120'
export RELAY_JWT_PRIVATE_KEY='<PKCS8 PEM or Base64 DER>'
export SERVER_SSL_KEY_STORE_BASE64='<Base64 PKCS12>'
export SERVER_SSL_KEY_STORE_PASSWORD='<secret>'
export SERVER_SSL_TRUST_STORE_BASE64='<Base64 PKCS12>'
export SERVER_SSL_TRUST_STORE_PASSWORD='<secret>'
go run ./cmd
```

The service does not create or migrate the shared database. Provision the Relay Controller schema before startup. The empty `migrations` directory remains reserved for deployment-managed schema changes.

## Security Boundary

mTLS authenticates the calling service, not the end user. Relay Service must derive and overwrite `X-Namespace` and `X-Account-Namespace` from its authenticated context; Controller validates syntax and trusts that internal identity assertion. Gateway uses its own database identity and must validate JWT signature, `aud`, expiration, tunnel, cluster, and scope.

Gateway enforces data-plane limits such as one Host per tunnel, bandwidth, HTTP request rate, and concurrent connections. The Controller's in-memory request limiter is only a bounded per-instance safety limit; strict cross-replica API limiting belongs at ingress.
