package transport

import (
	"strings"
	"testing"
)

// TestBuildWindowsPipeName_DistinctRootsDistinctPipes is the regression test
// for issue #5264: the Windows named pipe must be scoped by daemon root so an
// isolated daemon (selftest, parallel agent, second instance) does NOT collide
// on a single process-global per-user pipe.
func TestBuildWindowsPipeName_DistinctRootsDistinctPipes(t *testing.T) {
	a := buildWindowsPipeName("alice", `C:\Users\alice\AppData\Roaming\grafel`)
	b := buildWindowsPipeName("alice", `C:\tmp\agsbx-winpipe`)
	if a == b {
		t.Fatalf("distinct roots produced identical pipe names: %q", a)
	}
	for _, n := range []string{a, b} {
		if !strings.HasPrefix(n, `\\.\pipe\grafel-daemon-`) {
			t.Errorf("pipe name %q missing required prefix", n)
		}
		if len(n) > 256 {
			t.Errorf("pipe name %q exceeds Windows length limit", n)
		}
	}
}

// TestBuildWindowsPipeName_StableForSameRoot verifies the derivation is
// deterministic: the listen side and the dial side, given the same root, must
// agree on the pipe name or they can never connect. Casing/lexical variants of
// the same root must also agree (case-insensitive NTFS).
func TestBuildWindowsPipeName_StableForSameRoot(t *testing.T) {
	root := `C:\Users\alice\AppData\Roaming\grafel`
	if got, want := buildWindowsPipeName("alice", root), buildWindowsPipeName("alice", root); got != want {
		t.Fatalf("same root not stable: %q != %q", got, want)
	}

	// Casing variants must canonicalize to the same name (case-insensitive
	// NTFS). This is the property the listen/dial sides rely on; we lower-case
	// the root before hashing precisely so two case-spellings of the same
	// directory derive one pipe. (Lexical normalization of trailing
	// separators / "." segments is left to filepath.Clean, which is
	// GOOS-specific for backslash paths and not exercised here off-Windows.)
	want := buildWindowsPipeName("alice", root)
	if got := buildWindowsPipeName("alice", `C:\Users\Alice\AppData\Roaming\Grafel`); got != want {
		t.Errorf("casing variant derived %q; want %q", got, want)
	}
}

// TestBuildWindowsPipeName_DomainStripAndLowercase checks the username
// normalization (DOMAIN\User → user, lower-cased) is preserved.
func TestBuildWindowsPipeName_DomainStripAndLowercase(t *testing.T) {
	root := `C:\grafel`
	if got, want := buildWindowsPipeName(`DESKTOP-123\Alice`, root), buildWindowsPipeName("alice", root); got != want {
		t.Fatalf("domain/case not normalized: %q != %q", got, want)
	}
}

// TestBuildWindowsPipeName_EmptyRootLegacyName confirms an empty root degrades
// to the legacy user-only name (no trailing hash), so a degenerate caller
// still gets a valid, stable pipe path.
func TestBuildWindowsPipeName_EmptyRootLegacyName(t *testing.T) {
	if got, want := buildWindowsPipeName("alice", ""), `\\.\pipe\grafel-daemon-alice`; got != want {
		t.Fatalf("empty root: got %q want %q", got, want)
	}
}

// TestRootHash_DistinctAndStable sanity-checks the hash helper directly.
func TestRootHash_DistinctAndStable(t *testing.T) {
	if rootHash("") != "" {
		t.Errorf("empty root should hash to empty string")
	}
	h := rootHash(`C:\a`)
	if len(h) != 16 {
		t.Errorf("rootHash len = %d; want 16 hex chars", len(h))
	}
	if h == rootHash(`C:\b`) {
		t.Errorf("distinct roots hashed identically")
	}
	if h != rootHash(`C:\a`) {
		t.Errorf("rootHash not deterministic")
	}
}
