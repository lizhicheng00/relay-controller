# Management Service

Management Service maps trusted cloud identities to DevBridge namespaces and API keys. Browser login and Huawei Cloud authentication are owned by the client and the upper identity layer; this service never handles login credentials or redirects.

## Business Rules

- One cloud domain maps to one shared `accountNamespace`.
- One `(domainId, userId)` maps to one permanent `namespace`.
- Callers never submit or select a namespace.
- Each namespace has an API key limit for each scope.
- CLI login and the management page create API keys through the same API. The caller supplies a descriptive name such as `CLI login`.
- API keys can be deleted by ID when they are no longer needed.
- API key names are display labels and may repeat. All keys currently grant the same namespace access.
- API key scopes are `devbridge` and `devbox`. Each scope has an independent allowance.
- Keys use `devbridge_<payload>` or `devbox_<payload>`. The payload is 32-character unpadded Base64URL generated from 24 bytes of key material.
- MySQL stores only API key metadata and SHA-256 digests.
- All API keys are generated randomly. Complete values are returned only when issued.

## Namespace Ownership

Namespaces are internal resources. The trusted identity layer provides only `domainId` and `userId`; Management Service creates and resolves the corresponding namespace. It does not expose namespace creation, selection, enumeration, or deletion APIs.

Users in the same cloud domain receive different namespaces but share one `accountNamespace`. This keeps tunnel resources isolated by user while allowing account-level quota sharing.

## API

All business APIs use the prefix `/open-api-inner/v1/mgmt-service`.

```text
POST /open-api-inner/v1/mgmt-service/api-keys/check    validate a key and resolve its identity and scope
GET  /open-api-inner/v1/mgmt-service/api-keys  list API key metadata
POST /open-api-inner/v1/mgmt-service/api-keys  create an API key
DELETE /open-api-inner/v1/mgmt-service/api-keys/{keyId}  delete an API key
```

All endpoints require mTLS. The upper identity layer confirms the user before API key management, then supplies trusted `X-Domain-Id` and `X-User-Id` headers. Management Service does not receive login credentials or use an API key to authorize key management. Only the internal check endpoint accepts `X-API-Key`; Relay Controller uses its returned namespace, account namespace, and scope to authenticate business requests. The OpenAPI document is available at `/openapi.yaml`.

The management endpoints always operate on the namespace resolved from the supplied cloud identity; a caller cannot submit a namespace. Lists contain metadata, scopes, masks, and last-use times only. Creating a key requires `name` and `scope`; the complete value is returned once.

Opening the management page does not create an identity or an API key. Listing keys for a new user returns an empty list. Creating a key creates the user's namespace when needed.

## Data Ownership

- `domain_account` owns the cloud-domain mapping and shared `accountNamespace`.
- `user_identity` owns the `(domainId, userId)` mapping and user namespace.
- `api_key` stores key metadata and SHA-256 digests as child records of `user_identity`.

Every creation produces a new key. Creating and deleting keys locks the user identity while checking the per-scope count, preserving the limit across concurrent requests and service replicas. Successful API key authentication updates `lastUsedAt` at most once per minute.

For an existing DevBridge deployment, preload the known `domainId`, `userId`, `accountNamespace`, and namespace mappings into `domain_account` and `user_identity` before routing users to this service. Creating the first API key does not replace the imported namespace. Runtime APIs do not accept a namespace supplied by the caller.

## Structure

```text
cmd                  service and configuration-tool entrypoints
internal/config      environment configuration loading
internal/core        identity models and application errors
internal/httpapi     HTTP routes and OpenAPI
internal/security    API key generation, random identifiers, and PKCS12 mTLS
internal/secret      encrypted configuration values
internal/service     namespace and API key lifecycle
internal/store       MySQL persistence
migrations           embedded forward-only database migrations
```

## Configuration

| Variable | Required | Meaning |
| --- | --- | --- |
| `SERVER_ADDRESS` | no | HTTPS listen address, default `:8443` |
| `MGMT_CONFIG_DOG_FILE` | no | Dog key-component file, default `/opt/cloud/dog/beta` |
| `MGMT_CONFIG_PIG` | when using `ENC(...)` | Base64-encoded 32-byte pig key component |
| `DATABASE_DSN` | yes | MySQL DSN |
| `SERVER_SSL_KEY_STORE_BASE64` | yes | Base64 PKCS12 server key store |
| `SERVER_SSL_KEY_STORE_PASSWORD` | yes | Server key store password |
| `SERVER_SSL_TRUST_STORE_BASE64` | yes | Base64 PKCS12 client CA trust store |
| `SERVER_SSL_TRUST_STORE_PASSWORD` | yes | Trust store password |

The HTTPS server requires a trusted client certificate. TLS 1.2 and 1.3 are enabled.
The key store contains the server private key and certificate chain; the trust store contains
the accepted client CA.

Each sensitive configuration entry accepts either a plain value or an AES-256-GCM value in
`ENC(v1.<nonce>.<ciphertext+tag>)` format. Its working key is reconstructed from three independent
32-byte components: dog from a read-only file, cat embedded in the service, and pig from runtime
configuration. Dog and pig are read only when at least one encrypted value is configured.

Build the service and its small configuration tool:

```bash
mkdir -p bin
go build -o bin/mgmt-service ./cmd
go build -o bin/mgmt-config ./cmd/config
```

For local use, create the key once in the ignored `secrets` directory:

```bash
mkdir -p secrets
bin/mgmt-config init-dog secrets/dog
export CONFIG_PIG="$(bin/mgmt-config generate-pig)"
export MGMT_CONFIG_DOG_FILE="$PWD/secrets/dog"
export MGMT_CONFIG_PIG="$CONFIG_PIG"
```

Encrypt one value and place the returned `ENC(...)` text directly in its configuration entry:

```bash
read -rsp 'Value: ' CONFIG_VALUE
printf '\n'
printf '%s' "$CONFIG_VALUE" | bin/mgmt-config encrypt secrets/dog
unset CONFIG_VALUE
```

`encrypt` reads the plaintext from standard input and the pig component from `CONFIG_PIG`, so
neither is passed as a process argument.
Use `printf` rather than `echo`, because a trailing newline changes the encrypted value. For
production, mount dog read-only at `/opt/cloud/dog/beta` and provide pig through the service
configuration. The independently encrypted configuration entries can then be changed without
changing either component. Plain values do not require dog or pig.

Create an empty database and grant the service account schema-change permissions. The service applies pending embedded migrations before opening the HTTPS listener:

```bash
go run ./cmd
```

Applied migration versions are recorded in `schema_migration`. Migrations are forward-only and SQL files are maintained manually.

## Checks

```bash
go test ./...
go vet ./...
mkdir -p bin
go build -o bin/mgmt-service ./cmd
go build -o bin/mgmt-config ./cmd/config
```
