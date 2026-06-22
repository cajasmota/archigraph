package cli

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// wizardNavHint is the per-step footer shown under interactive wizard fields so
// users discover the keys — most importantly that SPACE toggles a multiselect
// item (#5338). Rendered via each field's Description.
const (
	// navHintMulti documents multiselect navigation (space toggles).
	navHintMulti = "↑/↓ move · space select · enter confirm · / filter · esc back"
	// navHintSelect documents single-select navigation (enter confirms).
	navHintSelect = "↑/↓ move · enter confirm · / filter · esc back"
)

// wizardListHeight returns a roomy height for a list of n options, clamped so a
// short list isn't cramped and a long list scrolls instead of flooding the
// terminal. Floor of 10 gives a comfortable window; ceiling of 20 keeps it from
// dwarfing small terminals (#5338).
func wizardListHeight(n int) int {
	h := n + 4 // options + title/description/help chrome
	if h < 10 {
		h = 10
	}
	if h > 20 {
		h = 20
	}
	return h
}

// wizardTheme returns the huh theme used by the interactive wizard. It is based
// on ThemeCharm but overrides the multiselect prefixes so selected/unselected
// options render as [✓]/[ ] brackets. The stock ThemeCharm uses "✓ "/"• ",
// which reads ambiguously as a bullet list rather than checkboxes — the #5337
// attempt to get brackets set the wrong field; the correct fields are
// Focused/Blurred.SelectedPrefix and UnselectedPrefix (#5338).
func wizardTheme() *huh.Theme {
	t := huh.ThemeCharm()

	// Preserve ThemeCharm's colors but swap the glyphs for explicit brackets.
	selFG := t.Focused.SelectedPrefix.GetForeground()
	unselFG := t.Focused.UnselectedPrefix.GetForeground()
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(selFG).SetString("[✓] ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(unselFG).SetString("[ ] ")

	blurSelFG := t.Blurred.SelectedPrefix.GetForeground()
	blurUnselFG := t.Blurred.UnselectedPrefix.GetForeground()
	t.Blurred.SelectedPrefix = lipgloss.NewStyle().Foreground(blurSelFG).SetString("[✓] ")
	t.Blurred.UnselectedPrefix = lipgloss.NewStyle().Foreground(blurUnselFG).SetString("[ ] ")

	return t
}
