package store

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"relay-controller/internal/config"
)

func TestIsTunnelNameConflict(t *testing.T) {
	nameConflict := &mysql.MySQLError{
		Number:  1062,
		Message: "Duplicate entry 'ns-user-1-demo' for key 'uk_tunnel_namespace_name'",
	}
	if !IsTunnelNameConflict(nameConflict) {
		t.Fatal("expected tunnel name conflict")
	}
	if IsTunnelNameConflict(&mysql.MySQLError{Number: 1062, Message: "duplicate uk_tunnel_id"}) ||
		IsTunnelNameConflict(errors.New("database unavailable")) {
		t.Fatal("unrelated errors must not be treated as tunnel name conflicts")
	}
}

func TestDataSourceNameIgnoresJDBCParameters(t *testing.T) {
	dsn := dataSourceName(config.Database{
		URL:      "jdbc:mariadb://database.example.com:3306/relay_controller?useUnicode=true&characterEncoding=utf8&serverTimezone=UTC",
		Username: "relay",
		Password: "secret",
	})
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Addr != "database.example.com:3306" || parsed.DBName != "relay_controller" {
		t.Fatalf("unexpected datasource: address=%q database=%q", parsed.Addr, parsed.DBName)
	}
}
