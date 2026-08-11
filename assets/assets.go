package assets

import "embed"

// Files contains the API contract and the versioned database migrations.
//
//go:embed openapi.yaml migrations/*.sql
var Files embed.FS
