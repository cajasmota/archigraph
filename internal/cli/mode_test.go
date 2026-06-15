package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/daemon/mode"
)

// TestNewModeCmd_unknownMode verifies that an unrecognised mode name returns
// a descriptive error and does not write anything to disk.
func TestNewModeCmd_unknownMode(t *testing.T) {
	cmd := newModeCmd()
	cmd.SetOut(&bytes.Buffer{})
	err := cmd.RunE(cmd, []string{"turbo"})
	if err == nil {
		t.Fatal("expected error for unknown mode, got nil")
	}
	if !strings.Contains(err.Error(), "unknown mode") {
		t.Errorf("error %q does not mention 'unknown mode'", err.Error())
	}
}

// TestSaveModeConfig_roundtrip verifies that SaveConfig + LoadConfig preserve
// both the Mode and EnvOverrides fields.
func TestSaveModeConfig_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.config.json")

	cfg := mode.Config{
		Mode:         mode.Workstation,
		EnvOverrides: map[string]string{"GRAFEL_HEAP_MAX_PCT": "70"},
	}
	if err := mode.SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got, err := mode.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got.Mode != mode.Workstation {
		t.Errorf("mode = %q, want workstation", got.Mode)
	}
	if got.EnvOverrides["GRAFEL_HEAP_MAX_PCT"] != "70" {
		t.Errorf("env override = %q, want 70", got.EnvOverrides["GRAFEL_HEAP_MAX_PCT"])
	}
}

// TestModeDefaults_readonlyDisablesAll verifies that readonly mode sets all
// three DISABLE_ vars.
func TestModeDefaults_readonlyDisablesAll(t *testing.T) {
	d := mode.ModeDefaults(mode.Readonly)
	for _, k := range []string{
		"GRAFEL_DISABLE_WATCHER",
		"GRAFEL_DISABLE_REBUILD",
		"GRAFEL_DISABLE_ALGO",
	} {
		if d[k] != "true" {
			t.Errorf("readonly default %s = %q, want true", k, d[k])
		}
	}
}

// TestModeDefaults_backgroundLowFootprint verifies background mode defaults.
func TestModeDefaults_backgroundLowFootprint(t *testing.T) {
	d := mode.ModeDefaults(mode.Background)
	if d["GRAFEL_EAGER_ALGO"] != "false" {
		t.Errorf("background GRAFEL_EAGER_ALGO = %q, want false", d["GRAFEL_EAGER_ALGO"])
	}
	if d["GRAFEL_HEAP_MAX_PCT"] != "60" {
		t.Errorf("background GRAFEL_HEAP_MAX_PCT = %q, want 60", d["GRAFEL_HEAP_MAX_PCT"])
	}
}
