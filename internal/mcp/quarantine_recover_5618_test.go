package mcp

// quarantine_recover_5618_test.go — Q3 query-side hook (#5618).
//
// Verifies that the MCP server's noteEntityAccess feeds the query/reference
// signal to a wired QuarantineRecoverer with a correctly-resolved absolute path,
// and is a safe no-op when no recoverer is wired or inputs are empty. The
// tracker's own Recover semantics (pin-respect, re-quarantine, persistence) are
// covered in internal/daemon/watch/quarantine_recover_test.go.

import (
	"path/filepath"
	"testing"
)

type fakeRecoverer struct {
	calls []struct{ repo, path string }
	ret   bool
}

func (f *fakeRecoverer) Recover(repo, path string) (string, bool) {
	f.calls = append(f.calls, struct{ repo, path string }{repo, path})
	return "", f.ret
}

func TestNoteEntityAccess_RelativeSourceResolvedToAbs(t *testing.T) {
	fr := &fakeRecoverer{}
	s := &Server{}
	s.SetQuarantineRecoverer(fr)

	lr := &LoadedRepo{Repo: "r", Path: "/proj/repo"}
	s.noteEntityAccess(lr, filepath.Join("app", "build", "x.go"))

	if len(fr.calls) != 1 {
		t.Fatalf("expected 1 Recover call, got %d", len(fr.calls))
	}
	wantPath := filepath.Join("/proj/repo", "app", "build", "x.go")
	if fr.calls[0].repo != "/proj/repo" || fr.calls[0].path != wantPath {
		t.Fatalf("Recover called with (%q,%q), want (/proj/repo,%q)",
			fr.calls[0].repo, fr.calls[0].path, wantPath)
	}
}

func TestNoteEntityAccess_AbsoluteSourcePassedThrough(t *testing.T) {
	fr := &fakeRecoverer{}
	s := &Server{}
	s.SetQuarantineRecoverer(fr)

	abs := filepath.Join("/proj/repo", "gen", "y.ts")
	s.noteEntityAccess(&LoadedRepo{Repo: "r", Path: "/proj/repo"}, abs)

	if len(fr.calls) != 1 || fr.calls[0].path != abs {
		t.Fatalf("absolute source should pass through unchanged, got %+v", fr.calls)
	}
}

func TestNoteEntityAccess_NoRecovererIsNoOp(t *testing.T) {
	s := &Server{} // no recoverer wired
	// Must not panic and must do nothing.
	s.noteEntityAccess(&LoadedRepo{Repo: "r", Path: "/proj/repo"}, "a/b.go")
}

func TestNoteEntityAccess_EmptyInputsAreNoOp(t *testing.T) {
	fr := &fakeRecoverer{}
	s := &Server{}
	s.SetQuarantineRecoverer(fr)

	s.noteEntityAccess(nil, "a/b.go")                       // nil repo
	s.noteEntityAccess(&LoadedRepo{Path: ""}, "a/b.go")     // no repo root
	s.noteEntityAccess(&LoadedRepo{Path: "/proj/repo"}, "") // no source file

	if len(fr.calls) != 0 {
		t.Fatalf("empty inputs must not call Recover, got %+v", fr.calls)
	}
}
