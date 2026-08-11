package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/lizhicheng00/relay-controller/internal/config"
)

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Store struct {
	db   *sql.DB
	exec executor
}

const (
	maxOpenConnections = 20
	maxIdleConnections = 10
)

func Open(ctx context.Context, cfg config.Database) (*Store, error) {
	dsn, err := dataSourceName(cfg)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConnections)
	db.SetMaxIdleConns(maxIdleConnections)
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetConnMaxIdleTime(time.Minute)

	pingContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingContext); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return &Store{db: db, exec: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) InTx(ctx context.Context, fn func(*Store) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	txStore := &Store{db: s.db, exec: tx}
	if err := fn(txStore); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func IsDuplicate(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func dataSourceName(cfg config.Database) (string, error) {
	raw := strings.TrimSpace(cfg.URL)
	if !strings.Contains(raw, "://") && !strings.HasPrefix(raw, "jdbc:") {
		return raw, nil
	}
	raw = strings.TrimPrefix(raw, "jdbc:")
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse DATASOURCE_URL: %w", err)
	}
	if parsed.Scheme != "mariadb" && parsed.Scheme != "mysql" {
		return "", fmt.Errorf("DATASOURCE_URL must use mariadb or mysql")
	}
	host := parsed.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(parsed.Hostname(), "3306")
	}
	database := strings.TrimPrefix(parsed.EscapedPath(), "/")
	database, err = url.PathUnescape(database)
	if err != nil || database == "" || strings.Contains(database, "/") {
		return "", fmt.Errorf("DATASOURCE_URL must contain one database name")
	}

	driverConfig := mysql.NewConfig()
	driverConfig.User = cfg.Username
	driverConfig.Passwd = cfg.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = host
	driverConfig.DBName = database
	driverConfig.ParseTime = true
	driverConfig.MultiStatements = true
	driverConfig.Timeout = 10 * time.Second
	driverConfig.ReadTimeout = 30 * time.Second
	driverConfig.WriteTimeout = 30 * time.Second
	driverConfig.Collation = "utf8mb4_unicode_ci"
	driverConfig.Params = map[string]string{"time_zone": "'+00:00'"}
	return driverConfig.FormatDSN(), nil
}
