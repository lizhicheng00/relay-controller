package store

import (
	"reflect"
	"testing"
)

func TestLoadMigrationsInVersionOrder(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	fileNames := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		fileNames = append(fileNames, migration.fileName)
	}
	want := []string{
		"V1__init_schema.sql",
		"V2__add_cn_north_4_bridge_cluster.sql",
		"V3__add_phase2_billing.sql",
		"V4__refine_phase2_schema.sql",
	}
	if !reflect.DeepEqual(fileNames, want) {
		t.Fatalf("migration order = %v, want %v", fileNames, want)
	}
}
