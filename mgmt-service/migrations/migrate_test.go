package migrations

import (
	"strings"
	"testing"
)

func TestEmbeddedMigrations(t *testing.T) {
	values, err := load()
	if err != nil {
		t.Fatal(err)
	}
	for index, item := range values {
		if item.version == 0 || item.name == "" || item.script == "" || len(item.checksum) != 64 {
			t.Fatalf("migration = %#v", item)
		}
		if !strings.HasPrefix(strings.TrimSpace(item.script), "CREATE TABLE") {
			t.Fatalf("migration %s has an invalid SQL prefix", item.name)
		}
		if index > 0 && values[index-1].version >= item.version {
			t.Fatalf("migrations are not ordered: %#v", values)
		}
	}
}
