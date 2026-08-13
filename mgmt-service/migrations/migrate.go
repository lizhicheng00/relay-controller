package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"

	mysqldriver "github.com/go-sql-driver/mysql"
)

//go:embed *.sql
var files embed.FS

type migration struct {
	version uint64
	name    string
	script  string
}

func Run(ctx context.Context, dsn string) error {
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return err
	}
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migration (
			version BIGINT UNSIGNED NOT NULL PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`); err != nil {
		return err
	}

	values, err := load()
	if err != nil {
		return err
	}
	for _, item := range values {
		var applied bool
		if err := db.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM schema_migration WHERE version = ?)`,
			item.version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		if _, err := db.ExecContext(ctx, item.script); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO schema_migration (version, name) VALUES (?, ?)`,
			item.version, item.name); err != nil {
			return err
		}
	}
	return nil
}

func load() ([]migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, err
	}
	values := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version, err := strconv.ParseUint(entry.Name()[:6], 10, 64)
		if err != nil {
			return nil, err
		}
		script, err := files.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		values = append(values, migration{version: version, name: entry.Name(), script: string(script)})
	}
	sort.Slice(values, func(left, right int) bool { return values[left].version < values[right].version })
	return values, nil
}
