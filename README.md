# Relay Controller

Relay Controller is the DevBridge / Relay Tunnel control plane service. It manages tunnel metadata, namespace accounts, limits, JWT signing, billing settlement, runtime decisions, and port policies.

This service does not implement WebSocket, WebTransport, TCP, or HTTP body forwarding. Real traffic bridging belongs to Relay Gateway.

Detailed business and code summary: [docs/relay-controller-business-code-summary.md](docs/relay-controller-business-code-summary.md)

User story and implementation design: [docs/relay-controller-user-story.md](docs/relay-controller-user-story.md)

## Stack

- Java 17
- Spring Boot 3
- Jetty embedded server
- Maven
- MySQL
- MyBatis Plus
- Nimbus JOSE JWT

## Implemented APIs

```text
POST   /open-api-inner/v1/relay-controller/tunnels
GET    /open-api-inner/v1/relay-controller/tunnels?clusterId=
DELETE /open-api-inner/v1/relay-controller/tunnels
GET    /open-api-inner/v1/relay-controller/tunnels/{tunnelId}
PUT    /open-api-inner/v1/relay-controller/tunnels/{tunnelId}
DELETE /open-api-inner/v1/relay-controller/tunnels/{tunnelId}
POST   /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/token?scope=host|connect
GET    /open-api-inner/v1/relay-controller/limits

POST   /open-api-inner/v1/relay-controller/clusters/{clusterId}/metering
POST   /open-api-inner/v1/relay-controller/clusters/{clusterId}/tunnels/status

POST   /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/ports
GET    /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/ports
GET    /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/ports/{port}
PUT    /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/ports/{port}
DELETE /open-api-inner/v1/relay-controller/tunnels/{tunnelId}/ports/{port}
GET    /open-api-inner/v1/relay-controller/clusters/{clusterId}/tunnels/{tunnelId}/ports/{port}
```

Namespace-scoped APIs read `X-Namespace` directly and store it as the tunnel namespace.
Each Relay Controller instance owns one configured region. Set `RELAY_REGION`; tunnel, port, and metering operations only accept clusters found under that local region. Set `RELAY_DOMAIN` to the tunnel URL suffix.
Tunnel `type` is restricted to `bridge` or `env`; blank create requests default to `bridge`.
Tunnel `expiration` in create and update requests is the allowed inactivity duration in hours. Blank create requests default to 72 hours. Tunnel responses expose that window as `expirationHours` and the current Unix expiration time as `tunnelExpiration`; clients can derive a live countdown from `tunnelExpiration`.
Successful tunnel or port changes refresh `tunnelExpiration`. Positive legacy metering reports also refresh it. An active Gateway status heartbeat refreshes it at most once every five minutes, so a connected tunnel is not aged out without rewriting its row every ten seconds. Reads and token issuance do not refresh expiration.
Tunnel `tunnelCode` is a 40-bit `long`; `tunnelId` is the fixed 8-character lowercase base32 encoding of that 40-bit value.
Tunnel URL format is `{tunnelId}.{clusterId}.{relay.domain}`.
Delete operations physically remove tunnels and their port policies. List APIs return only active, non-expired tunnels. Detail, update, port, and metering operations reject expired tunnels; expired records are physically removed after the configured retention period.
The seeded `trial` plan gives each namespace 5 GiB per UTC calendar month, at most 10 active tunnels, and at most 10 ports per tunnel. Tunnel and port creation serialize on database rows, so replicas sharing the database cannot exceed those metadata quotas through concurrent requests. Deleted and expired tunnels do not count against the tunnel quota.
`GET /limits` returns the monthly quota, used and remaining bytes, reset time, active tunnel count, and all plan limits. `POST /clusters/{clusterId}/tunnels/status` stores the latest Gateway telemetry and returns a per-tunnel `keep` or `disconnect` decision plus data-plane limits.

Tunnel tokens are issued explicitly with `POST /tunnels/{tunnelId}/token?scope=host|connect`. Every call creates a new token; tokens are not cached. Issuance is rejected after the current monthly quota is exhausted. Token lifetime is fixed by `relay.jwt.token.ttl-seconds` and does not follow the tunnel expiration. JWT claims are `iss`, `aud`, `exp`, `nbf`, `jti`, `tunnelId`, `clusterId`, `scp`, and `delivery`.
The optional `forCookies=true` query marks `delivery=cookie`; it does not encrypt the JWT or set a cross-domain cookie. The redirect or Gateway layer must set the returned signed token as a `Secure`, `HttpOnly`, appropriately scoped cookie.

Tunnel port APIs manage the explicit per-port allow list for a tunnel. Each port declares `protocol` as `http`, `https`, or `auto`. Unconfigured ports are denied by default. `allowAnonymous` only controls sending-side access to that port; listening-side gateway connection still requires token authentication.
The gateway port policy API keeps `clusterId` in the path intentionally. Gateway callers use it as their cluster scope, and Relay Controller verifies the tunnel belongs to that cluster before returning the port policy.

Namespace-scoped Tunnel and limits APIs have an in-memory fixed-window safety limit. The key is `X-Namespace`; set the per-instance limit with `RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE`. The bounded local limiter is not a cross-replica business quota, so a strict deployment-wide API limit belongs at the ingress. Cluster-scoped Gateway status, metering compatibility, and policy calls are not put behind this low user limit.

OpenAPI is maintained as YAML at `src/main/resources/static/openapi.yaml`. Maven uses this YAML during `generate-sources` to generate Spring API interfaces under `target/generated-sources/openapi`; controllers implement those generated interfaces and do not declare request mappings by hand.
The same YAML is served directly as a static resource:

```text
GET /openapi.yaml
```

## Database

Create the MySQL database before starting the service:

```sql
CREATE DATABASE IF NOT EXISTS relay_controller
DEFAULT CHARACTER SET utf8mb4
COLLATE utf8mb4_0900_ai_ci;
```

Flyway runs on application startup and applies migrations from `src/main/resources/db/migration`. `V3` adds trial plans, namespace accounts, UTC monthly periods, cumulative usage windows, 10-minute bills, runtime status, and the Tunnel account binding. `V4` removes unused audit fields and replaces surrogate IDs with business keys where no external identity is needed.

Gateway phase-two metering writes cumulative values directly to `tunnel_usage_window` every 30 seconds and at session end. The unique key is `(tunnel_id, session_id, window_start)`, where `window_start` is the UTC timestamp floored to ten minutes. Retries must use `GREATEST(existing, incoming)` rather than add the same report again. Relay Controller settles only the unbilled difference every ten minutes, so retries and multiple Controller replicas do not double charge.

The existing `/metering` endpoint remains compatible with phase one but is not part of monthly billing. A Gateway must use either that legacy flow or the phase-two usage-window flow, never both. See [docs/relay-controller-phase2.md](docs/relay-controller-phase2.md) for the database contract and ownership boundaries.

Database columns use snake_case for compound words, for example `tunnel_id`, `tunnel_code`, `cluster_id`, `bandwidth_used`, and `allow_anonymous`. Java fields remain camelCase and rely on MyBatis Plus underscore-to-camel mapping. The list-only `portCount` projection is explicitly marked as non-persistent.

## Run

Configure MySQL in `src/main/resources/application.yml`, then run:

```bash
mvn spring-boot:run
```

For another local machine, prefer overriding only the datasource URL, username, and password:

```bash
export DATASOURCE_URL='jdbc:mariadb://127.0.0.1:3306/relay_controller'
export DATASOURCE_USERNAME='root'
export DATASOURCE_PASSWORD='<secret>'
mvn spring-boot:run
```

For IntelliJ IDEA, paste this semicolon-separated template into **Run/Debug Configuration > Environment variables** and fill in each value:

```text
SPRING_PROFILES_ACTIVE=;SERVER_PORT=;DATASOURCE_URL=;DATASOURCE_USERNAME=;DATASOURCE_PASSWORD=;RELAY_JWT_PRIVATE_KEY=;SERVER_SSL_KEY_STORE_BASE64=;SERVER_SSL_KEY_STORE_PASSWORD=;SERVER_SSL_TRUST_STORE_BASE64=;SERVER_SSL_TRUST_STORE_PASSWORD=
```

Keep these values out of committed YAML:

```text
DATASOURCE_PASSWORD
RELAY_JWT_PRIVATE_KEY
SERVER_SSL_KEY_STORE_BASE64
SERVER_SSL_KEY_STORE_PASSWORD
SERVER_SSL_TRUST_STORE_BASE64
SERVER_SSL_TRUST_STORE_PASSWORD
```

For TLS, treat these as secrets:

```text
SERVER_SSL_KEY_STORE_BASE64
SERVER_SSL_KEY_STORE_PASSWORD
SERVER_SSL_TRUST_STORE_BASE64
SERVER_SSL_TRUST_STORE_PASSWORD
```

The Base64 keystore content is sensitive because it contains the server private key. The truststore usually contains only trusted client CA certificates, but it must still be protected from unauthorized replacement. Base64 is transport encoding, not encryption; keep both values in deployment secret storage.

The project uses MariaDB Connector/J. Use a `jdbc:mariadb://` datasource URL; Spring Boot infers the driver, so no driver class is configured explicitly.

## Mutual TLS

Relay Controller requires client certificates at the embedded Jetty layer. The `dev` and `prod` profile groups both activate `mtls`; provide a Base64-encoded PKCS12 server keystore plus a truststore containing the client CA:

```bash
export SPRING_PROFILES_ACTIVE=dev
export SERVER_PORT=8443
export SERVER_SSL_KEY_STORE_BASE64="$(base64 < /path/to/server.p12 | tr -d '\n')"
export SERVER_SSL_KEY_STORE_PASSWORD='<secret>'
export SERVER_SSL_TRUST_STORE_BASE64="$(base64 < /path/to/server-truststore.p12 | tr -d '\n')"
export SERVER_SSL_TRUST_STORE_PASSWORD='<secret>'
export RELAY_DOMAIN='myhuaweicloud.com'
export RELAY_REGION='cn-north-4'
export RELAY_RATE_LIMIT_REQUESTS_PER_MINUTE='120'
export RELAY_JWT_PRIVATE_KEY='<PKCS#8 PEM or Base64>'
mvn spring-boot:run
```

The profile registers `spring.ssl.bundle.jks.mtls` and assigns it through `server.ssl.bundle`. With `server.ssl.client-auth=need`, requests without a trusted client certificate fail during the TLS handshake and never reach the API controllers. Only TLS 1.2 and 1.3 are enabled. Callers must use `https://` and pass a client certificate signed by the CA in the configured truststore.

Use JDK 17 for normal development and deployment. The project uses Jetty instead of Tomcat. If Maven itself is run on JDK 26, Maven's own dependencies may still print JVM warnings before the application starts; those are not emitted by the Relay Controller runtime.

The optional `local-company-library-stubs` profile supports local IDE and `spring-boot:run` use only. It disables Spring Boot executable-JAR repackaging so a build containing fake SCC, random, or exception utilities is not mistaken for a deployable artifact.

All environments reject startup without `RELAY_JWT_PRIVATE_KEY`. The decrypted value must be a PKCS#8 RSA private key of at least 2048 bits; rotate it through `relay.jwt.key-id` and the verifier's public-key set. Gateway must validate both the signature and `aud=relay-gateway`.

## Security boundary

mTLS authenticates the calling regional service, while `X-Namespace` remains a caller-supplied account value. The deployment identity layer must authorize that service for the requested namespace; Relay Controller does not derive a user from the regional certificate. Cluster-scoped Gateway calls must also remain internal.

Gateway, not Relay Controller, enforces the returned data-plane limits: one Host per tunnel through a distributed Redis lock, 5 MiB/s per tunnel, 500 HTTP requests per minute per port, and 100 concurrent connections per port. Status heartbeats let Controller request disconnection after quota or policy changes, but they do not replace admission checks or token buckets in Gateway.
