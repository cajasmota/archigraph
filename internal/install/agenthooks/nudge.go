package agenthooks

import (
	"regexp"
	"strings"
)

// NudgeScript is the POSIX-sh advisory hook body. It is written to
// .claude/grafel-grep-nudge.sh by Install and invoked by the managed
// PreToolUse entry.
//
// Contract with Claude Code's PreToolUse mechanism:
//
//   - The tool call arrives as JSON on stdin. We read it, pull the command
//     string out of it (best-effort, no jq dependency), and decide whether
//     it looks like a STRUCTURAL grep.
//   - We ALWAYS exit 0. This is advisory; we never deny a tool call. The
//     nudge goes to stderr so it is visible without polluting any stdout the
//     harness might parse.
//   - We nudge at most ONCE PER SESSION: the first structural grep touches a
//     per-session marker file under the session tmp dir; subsequent calls in
//     the same session see the marker and stay silent, so we don't train the
//     user to ignore a repeated nag.
//
// The structural heuristic intentionally mirrors classifyStructural below so
// the Go table-test is an accurate proxy for the shell behaviour.
const NudgeScript = `#!/bin/sh
# grafel grep-interceptor (` + Marker + `)
# ADVISORY ONLY — nudges toward grafel MCP on STRUCTURAL greps.
# Never blocks: always exits 0. Claude Code ONLY (no other host has PreToolUse).

# Read the PreToolUse payload (JSON) from stdin.
payload="$(cat 2>/dev/null)"

# Best-effort extraction of the command/pattern text without a jq dependency:
# just scan the raw payload. The heuristic below is matched against it.
cmd="$payload"

# ---- structural-query heuristic (mirror of classifyStructural) ----
# Only fire on grep/rg invocations. Bail on anything else.
case "$cmd" in
  *grep*|*\"rg\"*|*\ rg\ *|*ripgrep*) : ;;
  *) exit 0 ;;
esac

# NOT structural: TODO/FIXME/string sweeps — grep is the right tool there.
case "$cmd" in
  *TODO*|*FIXME*|*XXX*|*HACK*) exit 0 ;;
esac

structural=0
# Definition hunts: 'def X', 'class X', 'func X', 'function X', 'type X',
# 'interface X', 'struct X' — language-agnostic symbol-definition patterns.
case "$cmd" in
  *def\ *|*class\ *|*func\ *|*function\ *|*interface\ *|*struct\ *|*type\ *) structural=1 ;;
esac
# Who-calls / usage hunts and recursive symbol sweeps: 'grep -r' / 'grep -rn'
# for a bare identifier are almost always "where is X used / defined".
case "$cmd" in
  *grep\ -r*|*grep\ -rn*|*grep\ -nr*|*rg\ *) structural=1 ;;
esac

[ "$structural" = "1" ] || exit 0

# ---- once-per-session dedup ----
sess="${CLAUDE_SESSION_ID:-${TMPDIR:-/tmp}}"
# Reduce the session id to a filesystem-safe token.
token="$(printf '%s' "$sess" | tr -c 'A-Za-z0-9_.-' '_')"
marker="${TMPDIR:-/tmp}/grafel-grep-nudge.${token}"
if [ -e "$marker" ]; then
  exit 0
fi
: > "$marker" 2>/dev/null || true

echo "grafel: structural query — grafel_find/inspect/neighbors is faster + more accurate than grep here (advisory; this grep still runs)." 1>&2
exit 0
`

// ── Go mirror of the heuristic (for hermetic table-testing) ──────────────

var (
	reGrepTool   = regexp.MustCompile(`\b(grep|rg|ripgrep)\b`)
	reNotStruct  = regexp.MustCompile(`\b(TODO|FIXME|XXX|HACK)\b`)
	reDefHunt    = regexp.MustCompile(`\b(def|class|func|function|interface|struct|type)\s`)
	reRecursive  = regexp.MustCompile(`grep\s+-[a-zA-Z]*r|grep\s+-[a-zA-Z]*\s|\brg\b`)
	reSymbolLike = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{2,}`)
)

// classifyStructural is the Go mirror of the shell heuristic in NudgeScript.
// It returns true when a grep/rg command line looks like a STRUCTURAL query
// (definition hunt, who-calls, or recursive symbol sweep) and false for
// non-grep commands and TODO/string sweeps. It is the canonical, testable
// statement of the heuristic; the shell version is kept behaviourally
// equivalent.
func classifyStructural(cmd string) bool {
	if !reGrepTool.MatchString(cmd) {
		return false
	}
	if reNotStruct.MatchString(cmd) {
		return false
	}
	// Definition hunt — strongest structural signal.
	if reDefHunt.MatchString(cmd) {
		return true
	}
	// Recursive / ripgrep sweep for a symbol-like identifier is "where is X
	// used/defined" — structural. Require a symbol-like token so a recursive
	// grep for a punctuation/string literal doesn't trip it.
	if reRecursive.MatchString(cmd) && reSymbolLike.MatchString(grepOperands(cmd)) {
		return true
	}
	return false
}

// grepOperands returns the command's pattern/operand tokens: it drops
// dash-flag tokens (e.g. "-rn") and the grep/rg tool words themselves, so
// the symbol-like check inspects the SEARCH PATTERN rather than the literal
// word "grep" or flag letters. Surrounding quotes are also trimmed so a
// quoted pattern like "===" is judged on its real content.
func grepOperands(cmd string) string {
	toolWord := map[string]bool{"grep": true, "rg": true, "ripgrep": true, "egrep": true, "fgrep": true}
	var b strings.Builder
	for _, tok := range strings.Fields(cmd) {
		if strings.HasPrefix(tok, "-") {
			continue
		}
		if toolWord[tok] {
			continue
		}
		tok = strings.Trim(tok, `"'`)
		b.WriteString(tok)
		b.WriteByte(' ')
	}
	return b.String()
}
