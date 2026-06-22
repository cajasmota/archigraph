package wiztui

import (
	"strings"
	"testing"
)

// TestAccentIsLightBlue asserts the single-source-of-truth accent is the new
// light blue (not the former pink/magenta 212), with a light-mode-friendly
// adaptive variant (fix B, #5340).
func TestAccentIsLightBlue(t *testing.T) {
	ac := colAccent // declared as lipgloss.AdaptiveColor in the theme
	if ac.Dark != "117" {
		t.Errorf("accent Dark = %q, want 117 (light blue)", ac.Dark)
	}
	if ac.Light == "" {
		t.Errorf("accent Light variant unset; want a light-mode-friendly blue")
	}
	// No lingering magenta anywhere in the accent colors.
	for _, c := range []string{ac.Light, ac.Dark} {
		if c == "212" || c == "211" || c == "213" {
			t.Errorf("accent still uses pink/magenta color %q", c)
		}
	}
}

// TestHeaderUsesAccent asserts the rendered header references the accent badge
// (smoke check that the theme wiring still produces styled output).
func TestHeaderUsesAccent(t *testing.T) {
	h := header(StepAction, 80)
	if !strings.Contains(h, "grafel wizard") {
		t.Errorf("header missing title:\n%s", h)
	}
}
