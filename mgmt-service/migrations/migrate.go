package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

const migrationLock = "mgmt_service_migration"

//go:embed *.sql
var files embed.FS

type migration struct {
	version  uint64
	name     string
	script   string
	checksum string
}

func Run(ctx context.Context, dsn string) error {
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("parse migration DSN: %w", err)
	}
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()

	connection, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("connect migration database: %w", err)
	}
	defer connection.Close()

	locked, err := acquireLock(ctx, connection)
	if err != nil {
		return err
	}
	if !locked {
		return errors.New("migration lock timed out")
	}
	defer releaseLock(connection)

	if _, err := connection.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migration (
			version BIGINT UNSIGNED NOT NULL,
			name VARCHAR(128) NOT NULL,
			checksum CHAR(64) NOT NULL,
			applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			PRIMARY KEY (version)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`); err != nil {
		return fmt.Errorf("create migration history: %w", err)
	}

	migrations, err := load()
	if err != nil {
		return err
	}
	for _, item := range migrations {
		if err := apply(ctx, connection, item); err != nil {
			return err
		}
	}
	return nil
}

func releaseLock(connection *sql.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = connection.ExecContext(ctx, `SELECT RELEASE_LOCK(?)`, migrationLock)
}

func acquireLock(ctx context.Context, connection *sql.Conn) (bool, error) {
	var result sql.NullInt64
	if err := connection.QueryRowContext(ctx, `SELECT GET_LOCK(?, 30)`, migrationLock).Scan(&result); err != nil {
		return false, fmt.Errorf("acquire migration lock: %w", err)
	}
	return result.Valid && result.Int64 == 1, nil
}

func apply(ctx context.Context, connection *sql.Conn, item migration) error {
	var checksum string
	err := connection.QueryRowContext(ctx,
		`SELECT checksum FROM schema_migration WHERE version = ?`, item.version).Scan(&checksum)
	switch {
	case err == nil:
		if checksum != item.checksum {
			return fmt.Errorf("migration %s checksum mismatch", item.name)
		}
		return nil
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("read migration %s: %w", item.name, err)
	}

	if _, err := connection.ExecContext(ctx, item.script); err != nil {
		return fmt.Errorf("apply migration %s: %w", item.name, err)
	}
	if _, err := connection.ExecContext(ctx, `
		INSERT INTO schema_migration (version, name, checksum) VALUES (?, ?, ?)`,
		item.version, item.name, item.checksum); err != nil {
		return fmt.Errorf("record migration %s: %w", item.name, err)
	}
	return nil
}

func load() ([]migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	values := make([]migration, 0, len(entries))
	seen := make(map[uint64]bool)
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
			continue
		}
		prefix, _, found := strings.Cut(entry.Name(), "_")
		version, err := strconv.ParseUint(prefix, 10, 64)
		if !found || err != nil || version == 0 || seen[version] {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		script, err := files.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		digest := sha256.Sum256(script)
		values = append(values, migration{
			version: version, name: entry.Name(), script: string(script),
			checksum: hex.EncodeToString(digest[:]),
		})
		seen[version] = true
	}
	sort.Slice(values, func(left, right int) bool { return values[left].version < values[right].version })
	if len(values) == 0 {
		return nil, errors.New("no database migrations found")
	}
	return values, nil
}
