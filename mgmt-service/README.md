# Management Service

Management Service maps trusted cloud identities to DevBridge namespaces and API keys. Browser login and Huawei Cloud authentication are owned by the client and the upper identity layer; this service never handles login credentials or redirects.

## Business Rules

- One cloud domain maps to one shared `accountNamespace`.
- One `(domainId, userId)` maps to one `namespace` and one permanent API key.
- Callers never submit or select a namespace.
- Repeated provisioning for the same user returns the same namespace and API key.
- API keys are 32 lowercase Base36 characters. MySQL stores only their SHA-256 digests.
- API key values are derived with HMAC-SHA256 from `API_KEY_SECRET`. All replicas must use the same secret; changing it is a coordinated rotation completed as each user provisions again.

## API

```text
POST /v1/api-key   trusted identity layer provisions or retrieves a user's API key
GET  /v1/me        API key resolves its domain, user, and namespaces
```

`POST /v1/api-key` requires `X-DevBridge-Proxy-Token`, `X-Domain-Id`, and `X-User-Id`. `GET /v1/me` requires `X-API-Key`. The OpenAPI document is available at `/openapi.yaml`.

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
