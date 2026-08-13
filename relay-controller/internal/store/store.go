package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"relay-controller/internal/config"
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

type NamedLock struct {
	conn *sql.Conn
	name string
}

const (
	maxOpenConnections = 20
	maxIdleConnections = 10
	lockReleaseTimeout = 5 * time.Second
)

func Open(ctx context.Context, cfg config.Database) (*Store, error) {
	db, err := sql.Open("mysql", dataSourceName(cfg))
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
		_ = db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return &Store{db: db, exec: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) TryNamedLock(ctx context.Context, name string) (*NamedLock, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", name).Scan(&acquired); err != nil {
		discardConn(conn)
		return nil, err
	}
	if !acquired {
		_ = conn.Close()
		return nil, nil
	}
	return &NamedLock{conn: conn, name: name}, nil
}

func (l *NamedLock) Release() error {
	ctx, cancel := context.WithTimeout(context.Background(), lockReleaseTimeout)
	defer cancel()
	var released bool
	err := l.conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", l.name).Scan(&released)
	if err != nil {
		discardConn(l.conn)
		return err
	}
	if !released {
		discardConn(l.conn)
		return fmt.Errorf("named lock %q was not held", l.name)
	}
	return l.conn.Close()
}

func discardConn(conn *sql.Conn) {
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	_ = conn.Close()
}

func (s *Store) InTx(ctx context.Context, fn func(*Store) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	txStore := &Store{db: s.db, exec: tx}
	if err := fn(txStore); err != nil {
		_ = tx.Rollback()
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

func dataSourceName(cfg config.Database) string {
	address := strings.TrimPrefix(cfg.URL, "jdbc:mariadb://")
	address = strings.TrimPrefix(address, "jdbc:mysql://")
	address, _, _ = strings.Cut(address, "?")
	address, database, _ := strings.Cut(address, "/")

	driverConfig := mysql.NewConfig()
	driverConfig.User = cfg.Username
	driverConfig.Passwd = cfg.Password
	driverConfig.Net = "tcp"
	driverConfig.Addr = address
	driverConfig.DBName = database
	driverConfig.ClientFoundRows = true
	driverConfig.ParseTime = true
	driverConfig.Timeout = 10 * time.Second
	driverConfig.ReadTimeout = 30 * time.Second
	driverConfig.WriteTimeout = 30 * time.Second
	driverConfig.Collation = "utf8mb4_unicode_ci"
	driverConfig.Params = map[string]string{"time_zone": "'+00:00'"}
	return driverConfig.FormatDSN()
}
