package mode_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/mode"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in      string
		want    mode.Mode
		wantErr bool
	}{
		{"background", mode.Background, false},
		{"workstation", mode.Workstation, false},
		{"readonly", mode.Readonly, false},
		{"BACKGROUND", "", true},
		{"", "", true},
		{"unknown", "", true},
	}
	for _, tc := range tests {
		got, err := mode.Parse(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Parse(%q): expected error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Parse(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestModeDefaultsBackground(t *testing.T) {
	d := mode.ModeDefaults(mode.Background)
	if d["GRAFEL_EAGER_ALGO"] != "false" {
		t.Errorf("background: GRAFEL_EAGER_ALGO = %q, want false", d["GRAFEL_EAGER_ALGO"])
	}
	if d["GRAFEL_HEAP_MAX_PCT"] != "60" {
		t.Errorf("background: GRAFEL_HEAP_MAX_PCT = %q, want 60", d["GRAFEL_HEAP_MAX_PCT"])
	}
	if _, ok := d["GRAFEL_EMBEDDING_URL"]; !ok {
		t.Error("background: GRAFEL_EMBEDDING_URL key should be present (empty string)")
	}
}

func TestModeDefaultsWorkstation(t *testing.T) {
	d := mode.ModeDefaults(mode.Workstation)
	if d["GRAFEL_EAGER_ALGO"] != "true" {
		t.Errorf("workstation: GRAFEL_EAGER_ALGO = %q, want true", d["GRAFEL_EAGER_ALGO"])
	}
	if d["GRAFEL_HEAP_MAX_PCT"] != "80" {
		t.Errorf("workstation: GRAFEL_HEAP_MAX_PCT = %q, want 80", d["GRAFEL_HEAP_MAX_PCT"])
	}
}

func TestModeDefaultsReadonly(t *testing.T) {
	d := mode.ModeDefaults(mode.Readonly)
	for _, k := range []string{"GRAFEL_DISABLE_WATCHER", "GRAFEL_DISABLE_REBUILD", "GRAFEL_DISABLE_ALGO"} {
		if d[k] != "true" {
			t.Errorf("readonly: %s = %q, want true", k, d[k])
		}
	}
}

func TestApplyDefaults_setsUnset(t *testing.T) {
	// Unset the key, apply background, verify it is set.
	os.Unsetenv("GRAFEL_EAGER_ALGO")
	t.Cleanup(func() { os.Unsetenv("GRAFEL_EAGER_ALGO") })

	mode.ApplyDefaults(mode.Background)
	if v := os.Getenv("GRAFEL_EAGER_ALGO"); v != "false" {
		t.Errorf("GRAFEL_EAGER_ALGO = %q after ApplyDefaults, want false", v)
	}
}

func TestApplyDefaults_doesNotOverrideExisting(t *testing.T) {
	os.Setenv("GRAFEL_EAGER_ALGO", "true")
	t.Cleanup(func() { os.Unsetenv("GRAFEL_EAGER_ALGO") })

	mode.ApplyDefaults(mode.Background)
	if v := os.Getenv("GRAFEL_EAGER_ALGO"); v != "true" {
		t.Errorf("GRAFEL_EAGER_ALGO = %q after ApplyDefaults, want true (existing override preserved)", v)
	}
}

func TestSaveLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.config.json")

	cfg := mode.Config{
		Mode:         mode.Background,
		EnvOverrides: map[string]string{"GRAFEL_HEAP_MAX_PCT": "50"},
	}
	if err := mode.SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, err := mode.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got.Mode != mode.Background {
		t.Errorf("loaded mode = %q, want background", got.Mode)
	}
	if got.EnvOverrides["GRAFEL_HEAP_MAX_PCT"] != "50" {
		t.Errorf("loaded override = %q, want 50", got.EnvOverrides["GRAFEL_HEAP_MAX_PCT"])
	}
}

func TestLoadConfig_missingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := mode.LoadConfig(filepath.Join(dir, "does-not-exist.json"))
	if err != nil {
		t.Fatalf("LoadConfig missing file: %v", err)
	}
	if cfg.Mode != "" {
		t.Errorf("expected empty mode for missing file, got %q", cfg.Mode)
	}
}
