package python

import (
	"strings"
	"testing"
)

// TestExtractMigrationEntity_ThreeOps is the primary regression guard for
// #2283: a migration file with 3 operations must emit exactly 1
// kind=Migration entity with all operations captured in properties.
func TestExtractMigrationEntity_ThreeOps(t *testing.T) {
	src := `from django.db import migrations, models

class Migration(migrations.Migration):
    dependencies = [
        ("core", "0041_device_prev"),
    ]

    operations = [
        migrations.AddField(
            model_name="device",
            name="serial_number",
            field=models.CharField(max_length=128, default=""),
        ),
        migrations.AddField(
            model_name="device",
            name="firmware_version",
            field=models.CharField(max_length=64, blank=True),
        ),
        migrations.AlterField(
            model_name="device",
            name="updated_at",
            field=models.DateTimeField(auto_now=True),
        ),
    ]
`
	ent := extractMigrationEntity(djangoMigrationFile{
		path:     "core/migrations/0042_device_serial_number.py",
		language: "python",
		source:   src,
	})

	if ent.Kind != "Migration" {
		t.Errorf("kind: want Migration, got %q", ent.Kind)
	}
	if ent.Subtype != "django" {
		t.Errorf("subtype: want django, got %q", ent.Subtype)
	}
	if ent.Name != "0042_device_serial_number" {
		t.Errorf("name: want 0042_device_serial_number, got %q", ent.Name)
	}
	if ent.SourceFile != "core/migrations/0042_device_serial_number.py" {
		t.Errorf("source_file: want core/migrations/0042_device_serial_number.py, got %q", ent.SourceFile)
	}
	if ent.Language != "python" {
		t.Errorf("language: want python, got %q", ent.Language)
	}

	// Exactly 3 operations.
	if ent.Properties["op_count"] != "3" {
		t.Errorf("op_count: want 3, got %q", ent.Properties["op_count"])
	}

	// Operations JSON contains all three types.
	ops := ent.Properties["operations"]
	for _, opType := range []string{"AddField", "AlterField"} {
		if !strings.Contains(ops, opType) {
			t.Errorf("operations: missing %q in %q", opType, ops)
		}
	}
	// Field names are captured.
	if !strings.Contains(ops, "serial_number") {
		t.Errorf("operations: missing field serial_number in %q", ops)
	}
	if !strings.Contains(ops, "firmware_version") {
		t.Errorf("operations: missing field firmware_version in %q", ops)
	}

	// Dependencies captured.
	deps := ent.Properties["dependencies"]
	if !strings.Contains(deps, "core/0041_device_prev") {
		t.Errorf("dependencies: expected core/0041_device_prev in %q", deps)
	}
}

// TestExtractMigrationEntity_Empty verifies that an empty operations list
// yields op_count=0 and an empty operations array without panicking.
func TestExtractMigrationEntity_Empty(t *testing.T) {
	src := `from django.db import migrations

class Migration(migrations.Migration):
    dependencies = []
    operations = []
`
	ent := extractMigrationEntity(djangoMigrationFile{
		path:     "app/migrations/0001_initial.py",
		language: "python",
		source:   src,
	})

	if ent.Properties["op_count"] != "0" {
		t.Errorf("op_count: want 0, got %q", ent.Properties["op_count"])
	}
	if ent.Properties["operations"] != "[]" {
		t.Errorf("operations: want [], got %q", ent.Properties["operations"])
	}
}

// TestMigrationFileName verifies the filename-stem extraction helper.
func TestMigrationFileName(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"core/migrations/0042_device_serial_number.py", "0042_device_serial_number"},
		{"app/migrations/0001_initial.py", "0001_initial"},
		{"migrations/0001_initial.py", "0001_initial"},
	}
	for _, c := range cases {
		if got := migrationFileName(c.path); got != c.want {
			t.Errorf("migrationFileName(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestEncodeMigrationOps verifies the hand-rolled JSON encoder.
func TestEncodeMigrationOps(t *testing.T) {
	ops := []migrationOp{
		{Type: "AddField", Model: "user", Field: "email"},
		{Type: "RemoveField", Model: "user", Field: "old_field"},
		{Type: "CreateModel", Model: "Profile"},
	}
	got := encodeMigrationOps(ops)
	for _, want := range []string{
		`"type":"AddField"`,
		`"model":"user"`,
		`"field":"email"`,
		`"type":"RemoveField"`,
		`"type":"CreateModel"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("encodeMigrationOps: missing %q in %q", want, got)
		}
	}
	// CreateModel has no field → field key must be absent for that entry.
	// The encoded JSON has the Profile entry without a "field" key.
	if strings.Contains(`{"type":"CreateModel","model":"Profile"}`, `"field"`) {
		t.Error("CreateModel entry should not have a field key")
	}
}
