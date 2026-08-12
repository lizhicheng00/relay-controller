# Management Service

Management Service maps authenticated Huawei IAM identities to DevBridge namespaces and manages permanent API keys. Relay Controller and Relay Gateway remain responsible for tunnels, ports, tokens, metering, quotas, and data-plane behavior.

## Business Rules

- One Huawei IAM domain maps to one DevBridge account namespace.
- One IAM user maps to one principal and one non-deletable default namespace.
- A principal may create additional namespaces and rename their display names.
- API keys belong to a namespace. Each namespace supports at most five keys.
- API keys are permanent, have `read` or `write` permission, and are returned only when created.
- The server stores only HMAC-SHA256 API key digests. A five-minute Redis login token separates IAM login from API key creation and can be consumed once.

## API

```text
POST   /v1/auth/iam/login
POST   /v1/auth/api-key

GET    /v1/me
GET    /v1/namespaces
POST   /v1/namespaces
GET    /v1/namespaces/{namespaceId}
PATCH  /v1/namespaces/{namespaceId}
DELETE /v1/namespaces/{namespaceId}

GET    /v1/namespaces/{namespaceId}/api-keys
POST   /v1/namespaces/{namespaceId}/api-keys
DELETE /v1/namespaces/{namespaceId}/api-keys/{keyId}
```

The OpenAPI document is available at `/openapi.yaml`. Business APIs use `X-API-Key`. IAM login is restricted to the trusted gateway by `X-DevBridge-Proxy-Token`; the gateway supplies the authenticated IAM identity headers.

## Structure

```text
cmd/mgmt-service     process startup and graceful shutdown
cmd/migrate          database migration command
cmd/mgmtctl          existing namespace binding command
internal/httpapi     HTTP routes, authentication, errors, and OpenAPI
internal/service     namespace, login, and API key workflows
internal/store       MySQL persistence
internal/session     one-time Redis login sessions
migrations           base database schema
```

## Configuration

| Variable | Required | Meaning |
| --- | --- | --- |
| `DATABASE_DSN` | yes | MySQL DSN |
| `API_KEY_PEPPER` | yes | API key HMAC secret, at least 32 characters |
| `IAM_TRUSTED_PROXY_TOKEN` | yes | Trusted IAM gateway secret, at least 32 characters |
| `REDIS_ADDRESS` | no | Redis address, default `localhost:6379` |
| `REDIS_PASSWORD` | no | Redis password |
| `SERVER_ADDRESS` | no | HTTP listen address, default `:8080` |
| `MIGRATIONS_PATH` | no | Migration directory, default `migrations` |

Run the base migration before starting the service:

```bash
go run ./cmd/migrate
go run ./cmd/mgmt-service
```

Existing namespace mappings can be bound before their first IAM login:

```bash
go run ./cmd/mgmtctl bind-namespace \
  --iam-domain-id domain-id \
  --iam-user-id user-id \
  --iam-user-name user-name \
  --account-namespace ns-existing-account \
  --namespace ns-existing-user
```

## Checks

```bash
go test ./...
go vet ./...
go build ./cmd/...
```
