package tooladapter_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/install/tooladapter"
)

func TestParseToolsFlag_ValidDedupOrder(t *testing.T) {
	got, err := tooladapter.ParseToolsFlag(" claude , windsurf,cursor ,claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"claude", "windsurf", "cursor"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseToolsFlag_CaseInsensitive(t *testing.T) {
	got, err := tooladapter.ParseToolsFlag("Claude,CURSOR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"claude", "cursor"}) {
		t.Fatalf("got %v", got)
	}
}

func TestParseToolsFlag_Unknown(t *testing.T) {
	_, err := tooladapter.ParseToolsFlag("claude,bogus")
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("error should name the unknown tool: %v", err)
	}
}

func TestParseToolsFlag_Empty(t *testing.T) {
	for _, in := range []string{"", "  ", ",", " , ,"} {
		if _, err := tooladapter.ParseToolsFlag(in); err == nil {
			t.Fatalf("expected error for empty input %q", in)
		}
	}
}

func TestNormalizeSelection_DropsUnknownReordersDedups(t *testing.T) {
	// Feed out-of-order, with an unknown and a dup.
	got := tooladapter.NormalizeSelection([]string{"cursor", "bogus", "claude", "cursor"})
	// registry order is claude, codex, cursor, windsurf, ...
	want := []string{"claude", "cursor"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWizardChoices_PreCheckFromDetection(t *testing.T) {
	choices := tooladapter.WizardChoices(nil)
	if len(choices) != len(tooladapter.AllIDs()) {
		t.Fatalf("expected %d choices, got %d", len(tooladapter.AllIDs()), len(choices))
	}
	for _, c := range choices {
		if c.PreChecked != c.Detected {
			t.Fatalf("nil preselected: PreChecked should mirror Detected for %s", c.ID)
		}
	}
}

func TestWizardChoices_PreCheckFromPreselected(t *testing.T) {
	choices := tooladapter.WizardChoices([]string{"cursor"})
	for _, c := range choices {
		want := c.ID == "cursor"
		if c.PreChecked != want {
			t.Fatalf("PreChecked for %s = %v, want %v", c.ID, c.PreChecked, want)
		}
	}
}

func TestComputeDelta(t *testing.T) {
	d := tooladapter.ComputeDelta(
		[]string{"claude", "windsurf"},
		[]string{"claude", "cursor"},
	)
	if !reflect.DeepEqual(d.Enabled, []string{"cursor"}) {
		t.Fatalf("enabled = %v", d.Enabled)
	}
	if !reflect.DeepEqual(d.Disabled, []string{"windsurf"}) {
		t.Fatalf("disabled = %v", d.Disabled)
	}
}

func TestComputeDelta_EmptyNextDisablesAll(t *testing.T) {
	d := tooladapter.ComputeDelta([]string{"claude", "cursor"}, nil)
	if len(d.Enabled) != 0 {
		t.Fatalf("enabled should be empty, got %v", d.Enabled)
	}
	// Disabled in registry order: claude before cursor.
	if !reflect.DeepEqual(d.Disabled, []string{"claude", "cursor"}) {
		t.Fatalf("disabled = %v", d.Disabled)
	}
}

func TestComputeDelta_NoChange(t *testing.T) {
	d := tooladapter.ComputeDelta([]string{"claude"}, []string{"claude"})
	if len(d.Enabled) != 0 || len(d.Disabled) != 0 {
		t.Fatalf("expected empty delta, got %+v", d)
	}
}
