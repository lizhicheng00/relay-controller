# Management Service

Management Service maps trusted cloud identities to DevBridge namespaces and API keys. Browser login and Huawei Cloud authentication are owned by the client and the upper identity layer; this service never handles login credentials or redirects.

## Business Rules

- One cloud domain maps to one shared `accountNamespace`.
- One `(domainId, userId)` maps to one permanent `namespace`.
- Callers never submit or select a namespace.
- Each namespace has one default API key and may have up to four additional API keys.
- The login flow retrieves the default API key. Repeated requests return the same default key.
- The default API key cannot be deleted. Additional keys are intended for separate clients or usage scenarios.
- API keys are 32 lowercase Base36 characters. MySQL stores only their SHA-256 digests.
- The default key is derived with HMAC-SHA256 from `API_KEY_SECRET`; additional keys are generated randomly and returned only when created.
- All replicas must use the same `API_KEY_SECRET`. Changing it is a coordinated default-key rotation completed as each user provisions again.

## Namespace Ownership

Namespaces are internal resources. The trusted identity layer provides only `domainId` and `userId`; Management Service creates and resolves the corresponding namespace. It does not expose namespace creation, selection, enumeration, or deletion APIs.

Users in the same cloud domain receive different namespaces but share one `accountNamespace`. This keeps tunnel resources isolated by user while allowing account-level quota sharing.

## Current API

```text
POST /v1/api-key   trusted identity layer provisions or retrieves the user's default API key
GET  /v1/me        API key resolves its domain, user, and namespaces
```

`POST /v1/api-key` requires `X-DevBridge-Proxy-Token`, `X-Domain-Id`, and `X-User-Id`. `GET /v1/me` requires `X-API-Key`. The OpenAPI document is available at `/openapi.yaml`.

These two endpoints cover login provisioning and API-key authentication. Additional API-key management requires the following endpoints before that capability is exposed:

```text
GET    /v1/api-keys          list key metadata for the current namespace
POST   /v1/api-keys          create an additional API key
DELETE /v1/api-keys/{keyId}  delete an additional API key
```

The list response never contains complete key values. A newly created additional key is returned once. Creation must lock the user identity while checking the five-key limit so the limit remains valid with concurrent requests and multiple service instances.

## Data Ownership

- `domain_account` owns the cloud-domain mapping and shared `accountNamespace`.
- `user_identity` owns the `(domainId, userId)` mapping and user namespace.
- API keys will be stored as child records of `user_identity`: slot `0` is the default key and slots `1` through `4` are additional keys.

The current base schema stores only the default key on `user_identity`. The child API-key table and management endpoints are the next required implementation change; namespace CRUD remains unnecessary.

## Structure

```text
cmd                  process startup and graceful shutdown
internal/config      environment configuration
internal/core        identity models and application errors
internal/httpapi     HTTP routes and OpenAPI
internal/security    API key derivation and random namespace identifiers
internal/service     identity provisioning and authentication
internal/store       MySQL persistence
migrations           base database schema
```

## Configuration

| Variable | Required | Meaning |
| --- | --- | --- |
| `DATABASE_DSN` | yes | MySQL DSN |
| `API_KEY_SECRET` | yes | API key derivation secret, at least 32 characters |
| `IDENTITY_PROXY_TOKEN` | yes | Trusted identity-layer credential, at least 32 characters |
| `SERVER_ADDRESS` | no | HTTP listen address, default `:8080` |

Apply the SQL migration with the deployment migration tool, then start the service:

```bash
go run ./cmd
```

## Checks

```bash
go test ./...
go vet ./...
go build ./cmd
```
