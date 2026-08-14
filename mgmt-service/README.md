# Management Service

Management Service maps trusted cloud identities to DevBridge namespaces and API keys. Browser login and Huawei Cloud authentication are owned by the client and the upper identity layer; this service never handles login credentials or redirects.

## Business Rules

- One cloud domain maps to one shared `accountNamespace`.
- One `(domainId, userId)` maps to one permanent `namespace`.
- Callers never submit or select a namespace.
- Each namespace may have five API keys for each type: one default key and four additional keys.
- The login flow supplies a type and issues that type's default API key. Each login replaces the previous default key of the same type.
- The default API key cannot be deleted. Additional keys are intended for separate clients or usage scenarios.
- Additional key names are unique within a namespace and type. All keys currently grant the same namespace access.
- API key types are `devbridge` and `devbox`. Each type has its own default key and additional-key allowance.
- Keys use `devbridge_<payload>` or `devbox_<payload>`. The payload is 32-character unpadded Base64URL generated from 24 bytes of key material.
- MySQL stores only API key metadata and SHA-256 digests.
- Default and additional keys are generated randomly. Complete values are returned only when issued.

## Namespace Ownership

Namespaces are internal resources. The trusted identity layer provides only `domainId` and `userId`; Management Service creates and resolves the corresponding namespace. It does not expose namespace creation, selection, enumeration, or deletion APIs.

Users in the same cloud domain receive different namespaces but share one `accountNamespace`. This keeps tunnel resources isolated by user while allowing account-level quota sharing.

## API

All business APIs use the prefix `/open-api-inner/v1/mgmt-service`.

```text
POST /open-api-inner/v1/mgmt-service/api-keys/default  issue or rotate a typed default API key
POST /open-api-inner/v1/mgmt-service/api-keys/check    validate a key and resolve its identity
GET  /open-api-inner/v1/mgmt-service/api-keys  list API key metadata
POST /open-api-inner/v1/mgmt-service/api-keys  create an additional API key
DELETE /open-api-inner/v1/mgmt-service/api-keys/{keyId}  delete an additional API key
```

The default API key endpoint requires `X-DevBridge-Proxy-Token`, `X-Domain-Id`, `X-User-Id`, and a JSON body containing `type`. The remaining business endpoints require `X-API-Key`. The OpenAPI document is available at `/openapi.yaml`.

The management endpoints always operate on the namespace resolved from `X-API-Key`; a caller cannot submit another namespace. Lists contain metadata, types, masks, and last-use times only. Creating a key requires `name` and `type`; the complete value is returned once.

## Data Ownership

- `domain_account` owns the cloud-domain mapping and shared `accountNamespace`.
- `user_identity` owns the `(domainId, userId)` mapping and user namespace.
- `api_key` stores key metadata and SHA-256 digests as child records of `user_identity`.
- Within each type, API key slot `0` is the default key and slots `1` through `4` are additional keys.

Issuing a default key replaces only the previous default key of the requested type. Creating and deleting additional keys locks the user identity while assigning a slot, preserving the five-key-per-type limit across concurrent requests and service replicas. Successful API key authentication updates `lastUsedAt` at most once per minute.

For an existing DevBridge deployment, preload the known `domainId`, `userId`, `accountNamespace`, and namespace mappings into `domain_account` and `user_identity` before routing users to this service. The first request for each type creates its default API key without replacing the imported namespace. Runtime APIs do not accept a namespace supplied by the caller.

## Structure

```text
cmd                  process startup and graceful shutdown
internal/config      environment configuration
internal/core        identity models and application errors
internal/httpapi     HTTP routes and OpenAPI
internal/security    API key generation, random identifiers, and PKCS12 mTLS
internal/service     namespace and API key lifecycle
internal/store       MySQL persistence
migrations           embedded forward-only database migrations
```

## Configuration

| Variable | Required | Meaning |
| --- | --- | --- |
| `DATABASE_DSN` | yes | MySQL DSN |
| `IDENTITY_PROXY_TOKEN` | yes | Trusted identity-layer credential, at least 32 characters |
| `SERVER_ADDRESS` | no | HTTPS listen address, default `:8443` |
| `SERVER_SSL_KEY_STORE_BASE64` | yes | Base64 PKCS12 server key store |
| `SERVER_SSL_KEY_STORE_PASSWORD` | yes | Server key store password |
| `SERVER_SSL_TRUST_STORE_BASE64` | yes | Base64 PKCS12 client CA trust store |
| `SERVER_SSL_TRUST_STORE_PASSWORD` | yes | Trust store password |

The HTTPS server requires a trusted client certificate. TLS 1.2 and 1.3 are enabled.
The key store contains the server private key and certificate chain; the trust store contains
the accepted client CA. Base64 is encoding rather than encryption, so passwords and stores
must be supplied through the deployment secret mechanism.

Create an empty database and grant the service account schema-change permissions. The service applies pending embedded migrations before opening the HTTPS listener:

```bash
go run ./cmd
```

Applied migration versions are recorded in `schema_migration`. Migrations are forward-only and SQL files are maintained manually.

## Checks

```bash
go test ./...
go vet ./...
go build ./cmd
```
