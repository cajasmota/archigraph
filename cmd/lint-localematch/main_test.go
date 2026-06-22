package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeGo writes src to a temp .go file and returns its path.
func writeGo(t *testing.T, dir, name, src string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestAnalyzeFile(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name string
		src  string
		want int // expected findings
	}{
		{
			name: "match_on_combinedoutput",
			src: `package x
import ("os/exec"; "strings")
func f() bool {
	out, _ := exec.Command("schtasks", "/delete").CombinedOutput()
	return strings.Contains(string(out), "cannot find")
}`,
			want: 1,
		},
		{
			name: "match_via_intermediate_var",
			src: `package x
import ("os/exec"; "strings")
func f() bool {
	out, _ := exec.Command("c").Output()
	s := string(out)
	return strings.Contains(s, "No such process")
}`,
			want: 1,
		},
		{
			name: "match_on_error_string",
			src: `package x
import ("os/exec"; "strings")
func f() bool {
	err := exec.Command("c").Run()
	return strings.Contains(err.Error(), "denied")
}`,
			want: 1,
		},
		{
			name: "suppressed_by_nolint",
			src: `package x
import ("os/exec"; "strings")
func f() bool {
	out, _ := exec.Command("c").CombinedOutput()
	return strings.Contains(string(out), "cannot find") // nolint:localematch
}`,
			want: 0,
		},
		{
			name: "no_exec_in_file_is_ignored",
			src: `package x
import "strings"
func f(s string) bool { return strings.Contains(s, "cannot find") }`,
			want: 0,
		},
		{
			name: "match_on_unrelated_literal_not_flagged",
			src: `package x
import ("os/exec"; "strings")
func f(name string) bool {
	_, _ = exec.Command("ps").Output()
	return strings.Contains(name, "daemon") // name is not output-derived
}`,
			want: 0,
		},
		{
			name: "exit_code_check_not_flagged",
			src: `package x
import "os/exec"
func f() bool {
	err := exec.Command("c").Run()
	return err == nil
}`,
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeGo(t, dir, tc.name+".go", tc.src)
			got, err := analyzeFile(p)
			if err != nil {
				t.Fatalf("analyzeFile: %v", err)
			}
			if len(got) != tc.want {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tc.want, got)
			}
		})
	}
}
