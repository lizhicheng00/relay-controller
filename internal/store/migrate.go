package store

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"hash/crc32"
	"io/fs"
	"os/user"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lizhicheng00/relay-controller/assets"
)

var migrationName = regexp.MustCompile(`^V([0-9]+)__(.+)\.sql$`)

type migration struct {
	version     int
	description string
	script      string
	checksum    int32
	sql         string
}

func (s *Store) Migrate(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer conn.Close()

	var locked int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK('relay_controller_migration', 60)").Scan(&locked); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if locked != 1 {
		return fmt.Errorf("migration lock timed out")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('relay_controller_migration')")

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	historyExists, err := tableExists(ctx, conn, "flyway_schema_history")
	if err != nil {
		return err
	}
	var existingTables int
	if !historyExists {
		if err := conn.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM information_schema.tables
			WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'`).Scan(&existingTables); err != nil {
			return fmt.Errorf("inspect database schema: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flyway_schema_history (
			installed_rank INT NOT NULL,
			version VARCHAR(50) NULL,
			description VARCHAR(200) NOT NULL,
			type VARCHAR(20) NOT NULL,
			script VARCHAR(1000) NOT NULL,
			checksum INT NULL,
			installed_by VARCHAR(100) NOT NULL,
			installed_on TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			execution_time INT NOT NULL,
			success TINYINT(1) NOT NULL,
			PRIMARY KEY (installed_rank),
			KEY flyway_schema_history_s_idx (success)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return fmt.Errorf("create migration history: %w", err)
	}
	if !historyExists && existingTables > 0 {
		if err := insertHistory(ctx, conn, 1, "1", "<< Flyway Baseline >>", "BASELINE", "<< Flyway Baseline >>", nil, 0, true); err != nil {
			return err
		}
	}

	applied, rank, err := migrationHistory(ctx, conn)
	if err != nil {
		return err
	}
	for _, item := range migrations {
		if checksum, ok := applied[item.version]; ok {
			if checksum != nil && *checksum != item.checksum {
				return fmt.Errorf("migration V%d checksum mismatch", item.version)
			}
			continue
		}
		rank++
		started := time.Now()
		_, executionErr := conn.ExecContext(ctx, item.sql)
		executionMillis := int(time.Since(started).Milliseconds())
		checksum := item.checksum
		if historyErr := insertHistory(ctx, conn, rank, strconv.Itoa(item.version), item.description, "SQL", item.script, &checksum, executionMillis, executionErr == nil); historyErr != nil {
			return historyErr
		}
		if executionErr != nil {
			return fmt.Errorf("apply migration %s: %w", item.script, executionErr)
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
		matches := migrationName.FindStringSubmatch(entry.Name())
		if entry.IsDir() || matches == nil {
			continue
		}
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration %s: %w", entry.Name(), err)
		}
		content, err := assets.Files.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		result = append(result, migration{
			version: version, description: strings.ReplaceAll(matches[2], "_", " "), script: entry.Name(),
			checksum: flywayChecksum(content), sql: string(content),
		})
	}
	sort.Slice(result, func(first, second int) bool { return result[first].version < result[second].version })
	for index := 1; index < len(result); index++ {
		if result[index-1].version == result[index].version {
			return nil, fmt.Errorf("duplicate migration version V%d", result[index].version)
		}
	}
	return result, nil
}

func migrationHistory(ctx context.Context, conn *sql.Conn) (map[int]*int32, int, error) {
	rows, err := conn.QueryContext(ctx, `SELECT installed_rank, version, checksum, success
		FROM flyway_schema_history ORDER BY installed_rank`)
	if err != nil {
		return nil, 0, fmt.Errorf("read migration history: %w", err)
	}
	defer rows.Close()
	applied := make(map[int]*int32)
	maxRank := 0
	for rows.Next() {
		var rank int
		var version sql.NullString
		var checksum sql.NullInt64
		var success bool
		if err := rows.Scan(&rank, &version, &checksum, &success); err != nil {
			return nil, 0, fmt.Errorf("scan migration history: %w", err)
		}
		maxRank = max(maxRank, rank)
		if !success {
			return nil, 0, fmt.Errorf("failed migration history entry at rank %d must be repaired", rank)
		}
		parsed, err := strconv.Atoi(version.String)
		if !version.Valid || err != nil {
			continue
		}
		if checksum.Valid {
			value := int32(checksum.Int64)
			applied[parsed] = &value
		} else {
			applied[parsed] = nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read migration history: %w", err)
	}
	return applied, maxRank, nil
}

func tableExists(ctx context.Context, conn *sql.Conn, name string) (bool, error) {
	var count int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?`, name).Scan(&count); err != nil {
		return false, fmt.Errorf("inspect table %s: %w", name, err)
	}
	return count == 1, nil
}

func insertHistory(ctx context.Context, conn *sql.Conn, rank int, version, description, migrationType, script string, checksum *int32, executionMillis int, success bool) error {
	installedBy := "relay-controller"
	if current, err := user.Current(); err == nil && current.Username != "" {
		installedBy = current.Username
	}
	_, err := conn.ExecContext(ctx, `INSERT INTO flyway_schema_history
		(installed_rank, version, description, type, script, checksum, installed_by, execution_time, success)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rank, version, description, migrationType, script, checksum, installedBy, executionMillis, success)
	if err != nil {
		return fmt.Errorf("write migration history: %w", err)
	}
	return nil
}

func flywayChecksum(content []byte) int32 {
	checksum := crc32.NewIEEE()
	for _, line := range bytes.Split(content, []byte{'\n'}) {
		checksum.Write(bytes.TrimSuffix(line, []byte{'\r'}))
	}
	return int32(checksum.Sum32())
}
