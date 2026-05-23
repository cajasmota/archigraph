package dashboard

// handlers_flows_source_test.go — unit tests for readSourceLines (#1898).
//
// Verifies the zero-startLine guard that prevents the step-click panel from
// displaying the file head when an entity has no recorded source position.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempSource creates a temp file with numbered lines for use in tests.
// Each line is "line <n>: <content>" so assertions can be precise.
func writeTempSource(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "source.js")
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(p, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("writeTempSource: %v", err)
	}
	return p
}

// TestReadSourceLines_ZeroStartLine_ReturnsEmpty is the regression test for
// #1898: when startLine == 0, readSourceLines must return ("", nil) rather
// than silently reading the first contextLines lines of the file.
func TestReadSourceLines_ZeroStartLine_ReturnsEmpty(t *testing.T) {
	lines := []string{
		"import OrdersService from './service';",
		"class OrdersController {",
		"  byNumber() { /* unrelated */ }",
		"  process()  { /* unrelated */ }",
		"  summary()  { /* this is the target method */ }",
		"}",
	}
	p := writeTempSource(t, lines)

	got, err := readSourceLines(p, "", 0, 0, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("want empty string for startLine==0; got:\n%s", got)
	}
}

// TestReadSourceLines_ZeroEndLine_AnchorOnStart verifies that when startLine
// is set but endLine is 0, the window is anchored on startLine (not on line 0)
// so the correct function body is returned.
func TestReadSourceLines_ZeroEndLine_AnchorOnStart(t *testing.T) {
	// 10 lines; target function lives on line 8.
	lines := []string{
		"// header comment",          // 1
		"import foo from 'foo';",     // 2
		"class OrdersService {",      // 3
		"  byNumber() {}",            // 4
		"  process() {}",             // 5
		"",                           // 6
		"  // GET /orders/summary",   // 7
		"  summary() { return []; }", // 8  ← target
		"}",                          // 9
		"module.exports = ...;",      // 10
	}
	p := writeTempSource(t, lines)

	// startLine=8, endLine=0 (not recorded), context=1
	got, err := readSourceLines(p, "", 8, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Lines 7..9 expected (startLine-1 .. startLine+1).
	if !strings.Contains(got, "summary()") {
		t.Errorf("want snippet to contain 'summary()'; got:\n%s", got)
	}
	// Must NOT contain file-head content (lines 1-5).
	if strings.Contains(got, "header comment") {
		t.Errorf("snippet must not include file header; got:\n%s", got)
	}
	if strings.Contains(got, "byNumber") {
		t.Errorf("snippet must not include unrelated method byNumber; got:\n%s", got)
	}
}

// TestReadSourceLines_Normal verifies the happy path: startLine and endLine
// both set, snippet covers the expected range with context.
func TestReadSourceLines_Normal(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = strings.Repeat("x", i+1) // distinct content per line
	}
	p := writeTempSource(t, lines)

	// target: lines 10-12, context 1 → expect lines 9-13
	got, err := readSourceLines(p, "", 10, 12, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Line 9 should appear (context before).
	if !strings.Contains(got, "    9  ") {
		t.Errorf("want line 9 in snippet; got:\n%s", got)
	}
	// Line 13 should appear (context after).
	if !strings.Contains(got, "   13  ") {
		t.Errorf("want line 13 in snippet; got:\n%s", got)
	}
	// Line 8 should NOT appear.
	if strings.Contains(got, "    8  ") {
		t.Errorf("line 8 must not appear; got:\n%s", got)
	}
}

// TestReadSourceLines_MissingFile returns an error for a non-existent file
// (startLine > 0 so the file-open path is reached).
func TestReadSourceLines_MissingFile(t *testing.T) {
	_, err := readSourceLines("/nonexistent/path/file.go", "", 5, 10, 1)
	if err == nil {
		t.Error("want error for missing file, got nil")
	}
}
