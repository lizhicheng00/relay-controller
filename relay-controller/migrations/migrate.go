package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

//go:embed *.sql
var files embed.FS

const migrationLock = "relay_controller_migrations"

func Run(ctx context.Context, dsn string) error {
	driverConfig, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return err
	}
	driverConfig.MultiStatements = true
	db, err := sql.Open("mysql", driverConfig.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	var locked sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, 300)`, migrationLock).Scan(&locked); err != nil {
		return err
	}
	if !locked.Valid || locked.Int64 != 1 {
		return fmt.Errorf("acquire migration lock %q: timed out", migrationLock)
	}
	defer func() {
		releaseContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var released sql.NullInt64
		_ = conn.QueryRowContext(releaseContext, `SELECT RELEASE_LOCK(?)`, migrationLock).Scan(&released)
	}()

	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migration (
			version BIGINT UNSIGNED NOT NULL PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		version, err := strconv.ParseUint(name[:6], 10, 64)
		if err != nil {
			return err
		}
		var applied bool
		if err := conn.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM schema_migration WHERE version = ?)`,
			version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		script, err := files.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, string(script)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := conn.ExecContext(ctx, `
			INSERT INTO schema_migration (version, name) VALUES (?, ?)`,
			version, name); err != nil {
			return err
		}
	}
	return nil
}
