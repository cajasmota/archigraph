// Tests for Issue #1275: migration metadata stamped on table entities.
package sql_test

import (
	"context"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
)

func TestMigrationMeta_DjangoStyle(t *testing.T) {
	src := `CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    total NUMERIC(10,2) NOT NULL
);
CREATE TABLE customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255)
);`
	ext, _ := extractor.Get("sql")
	entities, err := ext.Extract(context.Background(), extractor.FileInput{
		Path:     "myapp/migrations/0003_add_orders.sql",
		Content:  []byte(src),
		Language: "sql",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tables := make(map[string]map[string]string)
	for _, e := range entities {
		if e.Subtype == "table" {
			tables[e.Name] = e.Properties
		}
	}

	for _, name := range []string{"orders", "customers"} {
		props, ok := tables[name]
		if !ok {
			t.Errorf("table %q not found in entities", name)
			continue
		}
		if got := props["migration_file"]; got != "0003_add_orders.sql" {
			t.Errorf("table %q: migration_file = %q, want %q", name, got, "0003_add_orders.sql")
		}
		if got := props["migration_order"]; got != "00000003" {
			t.Errorf("table %q: migration_order = %q, want %q", name, got, "00000003")
		}
	}
}

func TestMigrationMeta_FlywayStyle(t *testing.T) {
	src := `CREATE TABLE products (id SERIAL PRIMARY KEY);`
	ext, _ := extractor.Get("sql")
	entities, err := ext.Extract(context.Background(), extractor.FileInput{
		Path:     "db/migrations/V12__add_products.sql",
		Content:  []byte(src),
		Language: "sql",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range entities {
		if e.Subtype != "table" {
			continue
		}
		if got := e.Properties["migration_order"]; got != "00000012" {
			t.Errorf("migration_order = %q, want 00000012", got)
		}
		if got := e.Properties["migration_file"]; got != "V12__add_products.sql" {
			t.Errorf("migration_file = %q, want V12__add_products.sql", got)
		}
	}
}

func TestMigrationMeta_TimestampStyle(t *testing.T) {
	src := `CREATE TABLE sessions (id SERIAL PRIMARY KEY, token TEXT);`
	ext, _ := extractor.Get("sql")
	entities, err := ext.Extract(context.Background(), extractor.FileInput{
		Path:     "app/migrations/20240501_create_sessions.sql",
		Content:  []byte(src),
		Language: "sql",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range entities {
		if e.Subtype != "table" {
			continue
		}
		if got := e.Properties["migration_order"]; got != "20240501" {
			t.Errorf("migration_order = %q, want 20240501", got)
		}
	}
}

func TestMigrationMeta_NotStampedOutsideMigrationsDir(t *testing.T) {
	src := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	ext, _ := extractor.Get("sql")
	entities, err := ext.Extract(context.Background(), extractor.FileInput{
		Path:     "schema/schema.sql",
		Content:  []byte(src),
		Language: "sql",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range entities {
		if e.Subtype != "table" {
			continue
		}
		if _, ok := e.Properties["migration_file"]; ok {
			t.Errorf("migration_file should not be set outside migrations dir; got %q", e.Properties["migration_file"])
		}
		if _, ok := e.Properties["migration_order"]; ok {
			t.Errorf("migration_order should not be set outside migrations dir")
		}
	}
}

func TestMigrationMeta_RailsDbMigrateStyle(t *testing.T) {
	src := `CREATE TABLE line_items (
    id bigint PRIMARY KEY,
    order_id bigint NOT NULL
);`
	ext, _ := extractor.Get("sql")
	entities, err := ext.Extract(context.Background(), extractor.FileInput{
		Path:     "db/migrate/20231015_create_line_items.sql",
		Content:  []byte(src),
		Language: "sql",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range entities {
		if e.Subtype == "table" && e.Name == "line_items" {
			found = true
			if got := e.Properties["migration_file"]; got != "20231015_create_line_items.sql" {
				t.Errorf("migration_file = %q", got)
			}
		}
	}
	if !found {
		t.Error("table line_items not found")
	}
}

func TestMigrationMeta_ColumnsNotStamped(t *testing.T) {
	src := `CREATE TABLE widgets (id SERIAL PRIMARY KEY, color VARCHAR(32));`
	ext, _ := extractor.Get("sql")
	entities, err := ext.Extract(context.Background(), extractor.FileInput{
		Path:     "migrations/0001_init.sql",
		Content:  []byte(src),
		Language: "sql",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range entities {
		if e.Subtype == "column" {
			if _, ok := e.Properties["migration_file"]; ok {
				t.Errorf("column entity %q should not have migration_file property", e.Name)
			}
		}
	}
}
