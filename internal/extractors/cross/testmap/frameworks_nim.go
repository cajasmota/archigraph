// Package testmap — Nim test-framework detection and call resolution.
//
// #4749 (the Nim slice of the coverage-linkage tail epic #4749/#4615). Deep
// linkage for Nim's standard test runner:
//
//	std/unittest (xUnit/BDD hybrid): suite "Subject": ... test "does y": ...
//	  Each `test "..."` leaf is a test case; its body (the colon-introduced,
//	  indentation-delimited statement list that follows) is scanned for direct
//	  production calls. The enclosing `suite "..."` subject is carried as a
//	  naming-convention fallback when the leaf body has no resolvable production
//	  call — mirroring the RSpec/busted describe-subject path.
//
// Nim blocks are INDENTATION-delimited (no braces, no `end` keyword), so this
// file carries a Nim-specific body extractor (extractNimBlockBody) that returns
// the run of lines indented MORE than the `test` line — quote-aware so a `test`
// keyword inside a string never opens a block. This is the Nim analog of the
// Lua keyword-balanced extractor in frameworks_lua.go.
package testmap

import (
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// std/unittest — suite "..." / test "..."  (xUnit/BDD)
// ---------------------------------------------------------------------------

// nimUnittestTestRE matches a unittest leaf case: `test "description":`. Group 1
// is the description. The trailing `:` introduces the block body.
var nimUnittestTestRE = regexp.MustCompile(
	`(?m)^([ \t]*)test\s+"([^"\n\r]{1,200})"\s*:`,
)

// nimUnittestSuiteRE matches a unittest container: `suite "Subject":`. The first
// suite whose subject looks like a code identifier (CamelCase / dotted /
// module-ish) is used as the subject-under-test fallback. Group 1 = subject.
var nimUnittestSuiteRE = regexp.MustCompile(
	`(?m)^[ \t]*suite\s+"([^"\n\r]{1,200})"\s*:`,
)

// nimSubjectIdentRE recognises a suite subject that names a code symbol (e.g.
// "UserService", "users.handler", "Account.create") so it can seed a
// naming-convention TESTS edge. Plain prose subjects ("returns 200 on GET") are
// rejected — they have spaces and are not identifier-shaped.
var nimSubjectIdentRE = regexp.MustCompile(`^[A-Za-z_][\w]*(?:[.][A-Za-z_][\w]*)*$`)

// nimSuiteSubject returns the first suite subject that is identifier-shaped
// (no spaces), trimming a trailing "()" some authors append. Returns "" when no
// suite block names a code symbol.
func nimSuiteSubject(source string) string {
	for _, m := range nimUnittestSuiteRE.FindAllStringSubmatch(source, -1) {
		subj := strings.TrimSpace(m[1])
		subj = strings.TrimSuffix(subj, "()")
		if nimSubjectIdentRE.MatchString(subj) {
			// Use the tail of a dotted subject ("users.handler" → "handler").
			if idx := strings.LastIndexByte(subj, '.'); idx >= 0 {
				if tail := subj[idx+1:]; tail != "" {
					return tail
				}
			}
			return subj
		}
	}
	return ""
}

// indentWidth returns the number of leading space/tab bytes in line (tabs count
// as one column, sufficient for relative-indent comparison within one file).
func indentWidth(line string) int {
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	return n
}

// extractNimBlockBody returns the source slice of the indentation-delimited
// block whose header line begins at byte offset headerStart. The body is the
// run of subsequent lines indented STRICTLY MORE than the header line; the
// block ends at the first non-blank line indented at or below the header
// indent. Blank lines inside the block are kept. This mirrors the Nim compiler's
// off-side rule closely enough for call-scanning purposes.
func extractNimBlockBody(source string, headerStart int) string {
	// Locate the end of the header line.
	nl := strings.IndexByte(source[headerStart:], '\n')
	if nl < 0 {
		return ""
	}
	headerLine := source[headerStart : headerStart+nl]
	headerIndent := indentWidth(headerLine)
	bodyStart := headerStart + nl + 1
	i := bodyStart
	n := len(source)
	for i < n {
		lineEnd := strings.IndexByte(source[i:], '\n')
		var line string
		if lineEnd < 0 {
			line = source[i:]
		} else {
			line = source[i : i+lineEnd]
		}
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && indentWidth(line) <= headerIndent {
			// Dedented to header level or less → block ended before this line.
			return source[bodyStart:i]
		}
		if lineEnd < 0 {
			return source[bodyStart:]
		}
		i += lineEnd + 1
	}
	return source[bodyStart:]
}

// nimTestCaseName normalises a `test "does a thing"` description into a snake
// case-ish identifier used as the test-case qname (spaces → underscores, only
// identifier-safe chars kept). Returns "" when nothing usable remains.
func nimTestCaseName(desc string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range strings.TrimSpace(desc) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func detectNimUnittest(source string) []testFunction {
	subject := nimSuiteSubject(source)

	var out []testFunction
	seen := map[string]bool{}
	for _, m := range nimUnittestTestRE.FindAllStringSubmatchIndex(source, -1) {
		desc := source[m[4]:m[5]]
		name := nimTestCaseName(desc)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		body := extractNimBlockBody(source, m[0])
		out = append(out, testFunction{qname: name, body: body, describeSubject: subject})
	}
	return out
}
