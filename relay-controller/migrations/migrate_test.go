package migrations

import "testing"

func TestEmbeddedMigrations(t *testing.T) {
	values, err := load()
	if err != nil {
		t.Fatal(err)
	}
	for index, item := range values {
		if item.version == 0 || item.name == "" || item.script == "" {
			t.Fatalf("migration = %#v", item)
		}
		if index > 0 && values[index-1].version >= item.version {
			t.Fatalf("migrations are not ordered: %#v", values)
		}
	}
}
