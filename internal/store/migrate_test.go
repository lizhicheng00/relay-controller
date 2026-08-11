package store

import "testing"

func TestEmbeddedMigrationChecksumsMatchReleasedFlywayHistory(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	expected := map[int]int32{
		1: -1809567610,
		2: -1206028741,
		3: -69587134,
		4: 1107761935,
	}
	if len(migrations) != len(expected) {
		t.Fatalf("found %d migrations, want %d", len(migrations), len(expected))
	}
	for _, migration := range migrations {
		if migration.checksum != expected[migration.version] {
			t.Fatalf("V%d checksum = %d, want %d", migration.version, migration.checksum, expected[migration.version])
		}
	}
}
