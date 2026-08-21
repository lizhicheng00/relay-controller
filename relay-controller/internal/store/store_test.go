package store

import (
	"errors"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
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

func TestDataSourceNameAppliesRequiredParameters(t *testing.T) {
	dsn, err := dataSourceName("relay:secret@tcp(database.example.com:3306)/relay_controller?timeout=5s")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Addr != "database.example.com:3306" || parsed.DBName != "relay_controller" ||
		!parsed.ParseTime || !parsed.ClientFoundRows || parsed.Loc != time.UTC {
		t.Fatalf("unexpected datasource: %#v", parsed)
	}
}
