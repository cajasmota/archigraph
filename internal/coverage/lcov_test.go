package coverage

import (
	"os"
	"strings"
	"testing"
)

func loadSample(t *testing.T) *Report {
	t.Helper()
	f, err := os.Open("testdata/sample.info")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	rep, err := ParseLCOV(f)
	if err != nil {
		t.Fatalf("ParseLCOV: %v", err)
	}
	return rep
}

func TestParseLCOV_FileSummaries(t *testing.T) {
	rep := loadSample(t)
	if len(rep.Files) != 2 {
		t.Fatalf("want 2 files, got %d", len(rep.Files))
	}

	calc := rep.ByPath("/home/ci/project/src/calc.ts")
	if calc == nil {
		t.Fatal("calc.ts not parsed")
	}
	if calc.TotalLines != 10 || calc.CoveredLines != 8 {
		t.Errorf("calc totals: want 8/10, got %d/%d", calc.CoveredLines, calc.TotalLines)
	}
	if got := calc.Pct(); got != 80.0 {
		t.Errorf("calc pct: want 80.0, got %v", got)
	}
	if calc.LineHits[1] != 5 || calc.LineHits[7] != 0 {
		t.Errorf("line hits wrong: line1=%d line7=%d", calc.LineHits[1], calc.LineHits[7])
	}
}

func TestParseLCOV_Functions(t *testing.T) {
	rep := loadSample(t)
	calc := rep.ByPath("/home/ci/project/src/calc.ts")
	if len(calc.Funcs) != 2 {
		t.Fatalf("want 2 funcs, got %d", len(calc.Funcs))
	}
	byName := map[string]FuncCoverage{}
	for _, fn := range calc.Funcs {
		byName[fn.Name] = fn
	}
	if got := byName["addNumbers"]; got.Hits != 5 || got.Line != 1 {
		t.Errorf("addNumbers: want hits=5 line=1, got %+v", got)
	}
	if got := byName["unusedHelper"]; got.Hits != 0 || got.Line != 7 {
		t.Errorf("unusedHelper: want hits=0 line=7, got %+v", got)
	}
}

func TestParseLCOV_DerivedTotals(t *testing.T) {
	// No LF/LH — totals must be derived from DA lines.
	in := strings.NewReader("SF:a.go\nDA:1,1\nDA:2,0\nDA:3,4\nend_of_record\n")
	rep, err := ParseLCOV(in)
	if err != nil {
		t.Fatal(err)
	}
	f := rep.ByPath("a.go")
	if f.TotalLines != 3 || f.CoveredLines != 2 {
		t.Errorf("derived totals: want 2/3, got %d/%d", f.CoveredLines, f.TotalLines)
	}
}

func TestParseLCOV_DuplicateDAKeepsMax(t *testing.T) {
	in := strings.NewReader("SF:a.go\nDA:1,0\nDA:1,3\nend_of_record\n")
	rep, err := ParseLCOV(in)
	if err != nil {
		t.Fatal(err)
	}
	if h := rep.ByPath("a.go").LineHits[1]; h != 3 {
		t.Errorf("want max hits 3, got %d", h)
	}
}

func TestParseLCOV_Tolerant(t *testing.T) {
	// Unknown records, CRLF, stray blank lines, DA checksum field.
	in := strings.NewReader("VER:1\r\nSF:a.go\r\n\r\nWEIRD:xyz\r\nDA:1,2,abc123\r\nend_of_record\r\n")
	rep, err := ParseLCOV(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Files) != 1 || rep.ByPath("a.go").LineHits[1] != 2 {
		t.Errorf("tolerant parse failed: %+v", rep.Files)
	}
}
