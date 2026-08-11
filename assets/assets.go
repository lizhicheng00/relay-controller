package assets

import "embed"

// Files contains the versioned database migrations.
//
//go:embed migrations/*.sql
var Files embed.FS
