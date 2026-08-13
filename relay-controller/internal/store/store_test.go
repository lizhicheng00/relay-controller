package store

import (
	"testing"

	"github.com/go-sql-driver/mysql"
	"relay-controller/internal/config"
)

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
