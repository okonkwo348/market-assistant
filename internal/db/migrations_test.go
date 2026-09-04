package db

import (
	"testing"
)

func TestParseMigrationName(t *testing.T) {
	tests := []struct {
		name    string
		version int64
		title   string
		wantErr bool
	}{
		{"000001_create_schema_migrations.up.sql", 1, "create_schema_migrations", false},
		{"000042_add_products.up.sql", 42, "add_products", false},
		{"bad_name.sql", 0, "", true},
		{"000000_invalid.up.sql", 0, "", true},
		{"abc_test.up.sql", 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, title, err := parseMigrationName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseMigrationName() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (version != tt.version || title != tt.title) {
				t.Fatalf("got (%d, %q), want (%d, %q)", version, title, tt.version, tt.title)
			}
		})
	}
}

func TestLoadMigrations(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("got %d migrations, want 2", len(migrations))
	}
	if migrations[0].version != 1 || migrations[1].version != 2 {
		t.Fatalf("unexpected migration versions: %d, %d", migrations[0].version, migrations[1].version)
	}
	if migrations[0].upHash == "" || migrations[1].upHash == "" {
		t.Fatal("migration checksums must not be empty")
	}
	if migrations[0].downSQL == "" || migrations[1].downSQL == "" {
		t.Fatal("down migrations must not be empty")
	}
}
