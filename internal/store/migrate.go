package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"

	"github.com/lizhicheng00/relay-controller/assets"
)

const migrationLock = "relay_controller_migration"

var migrationName = regexp.MustCompile(`^V([0-9]+)__.+\.sql$`)

type migration struct {
	version  int
	fileName string
	sql      string
}

func (s *Store) Migrate(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer conn.Close()

	var locked bool
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 60)", migrationLock).Scan(&locked); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("migration lock timed out")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", migrationLock)

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migration (
		file_name VARCHAR(255) NOT NULL,
		applied_at BIGINT UNSIGNED NOT NULL,
		PRIMARY KEY (file_name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Applied database migrations'`); err != nil {
		return fmt.Errorf("create migration history: %w", err)
	}
	applied, err := appliedMigrations(ctx, conn)
	if err != nil {
		return err
	}
	for _, item := range migrations {
		if applied[item.fileName] {
			continue
		}
		if _, err := conn.ExecContext(ctx, item.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", item.fileName, err)
		}
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO schema_migration (file_name, applied_at) VALUES (?, UNIX_TIMESTAMP())", item.fileName); err != nil {
			return fmt.Errorf("record migration %s: %w", item.fileName, err)
		}
	}
	return nil
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(assets.Files, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	result := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationName.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, fmt.Errorf("invalid migration file name %s", entry.Name())
		}
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration %s: %w", entry.Name(), err)
		}
		content, err := assets.Files.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		result = append(result, migration{version: version, fileName: entry.Name(), sql: string(content)})
	}
	sort.Slice(result, func(first, second int) bool { return result[first].version < result[second].version })
	for index := 1; index < len(result); index++ {
		if result[index-1].version == result[index].version {
			return nil, fmt.Errorf("duplicate migration version V%d", result[index].version)
		}
	}
	return result, nil
}

func appliedMigrations(ctx context.Context, conn *sql.Conn) (map[string]bool, error) {
	rows, err := conn.QueryContext(ctx, "SELECT file_name FROM schema_migration")
	if err != nil {
		return nil, fmt.Errorf("read migration history: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var fileName string
		if err := rows.Scan(&fileName); err != nil {
			return nil, fmt.Errorf("scan migration history: %w", err)
		}
		applied[fileName] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read migration history: %w", err)
	}
	return applied, nil
}
