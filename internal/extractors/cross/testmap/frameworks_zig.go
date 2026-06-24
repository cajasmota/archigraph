// Package testmap — Zig test-block detection and call resolution.
//
// #5377 (epic #5360 Group A — bootstrap). Linkage for `zig test`, Zig's
// built-in test runner. Zig has no separate test framework: top-level
// `test "name" { … }` (or `test name { … }` / bare `test { … }`) blocks are
// compiled and run by `zig test file.zig` / `zig build test`:
//
//	const std = @import("std");
//	const expect = std.testing.expect;
//
//	fn add(a: i32, b: i32) i32 { return a + b; }
//
//	test "add sums two numbers" {
//	    try expect(add(2, 2) == 4);
//	}
//
//	test add {                       // identifier form (doc-test for `add`)
//	    try expect(add(1, 1) == 2);
//	}
//
// Each `test` block is one test case. Its brace body is scanned by the shared
// resolver for direct production calls (`add(2, 2)` → the `add` operation). The
// `std.testing.*` assertion DSL is denylisted in resolver.go so it never
// surfaces as the production subject.
//
// GATING (important): Zig test blocks live in ANY `.zig` file — frequently the
// production source file itself, not a separate test file. The framework entry
// is therefore FILENAME-gated on the `.zig` extension and the detector
// SELF-CONFIRMS: a `.zig` file with no `test` block yields zero cases and is
// dropped downstream (exactly like the rust_test / fsharp-expecto entries). It
// is deliberately NOT import-token gated — a bare `test` token would over-match
// the substring import matcher against unrelated header paths (e.g. C++
// `gtest/gtest.h` contains "test"), the precedent the elm-test entry documents.
package testmap

import (
	"regexp"
	"strconv"
)

// zigTestRE matches a top-level Zig test-block header in its three forms:
//
//	test "describes the case" {      → group 1 = the quoted description
//	test add {                       → group 2 = the identifier (doc-test target)
//	test {                           → neither group (anonymous test)
//
// The `test` keyword must start a line (allowing indentation) so a `test`
// substring inside an identifier/string/comment never opens a block. The match
// ends at the opening `{`, from which extractBraceBody captures the balanced
// body. `comptime`-prefixed and nested constructs are out of scope (honest
// partial — dynamic/comptime-generated tests are not modelled).
var zigTestRE = regexp.MustCompile(
	`(?m)^[ \t]*test\b[ \t]*(?:"([^"\n\r]{1,200})"|([A-Za-z_]\w*))?[ \t]*\{`,
)

// zigTestSubject returns the production symbol an identifier-form test exercises
// (`test add { … }` → `add`). The quoted-description form carries no structural
// subject (the resolver relies on the body call scan); an empty string is
// returned so the resolver does not invent one.
func zigTestSubject(ident string) string {
	if ident == "" {
		return ""
	}
	return ident
}

// zigTestCaseName normalises a test header into a stable identifier:
//   - quoted description "add sums two numbers" → add_sums_two_numbers
//   - identifier form     add                   → add
//   - anonymous test                            → anonymous_test_<n> (caller indexes)
//
// Reuses the Nim snake-case normaliser for the quoted form (spaces/punctuation →
// underscores, no framework prefix — Zig test names are bare).
func zigTestCaseName(desc, ident string) string {
	if ident != "" {
		return ident
	}
	if desc == "" {
		return ""
	}
	return nimTestCaseName(desc)
}

// detectZigTest detects `zig test` blocks in a .zig source file. Each block is
// one test case; the body is captured for the shared resolver's production-call
// scan, and an identifier-form test (`test add { … }`) carries `add` as the
// naming-convention subject-under-test fallback.
func detectZigTest(source string) []testFunction {
	var out []testFunction
	seen := map[string]bool{}
	anon := 0
	for _, m := range zigTestRE.FindAllStringSubmatchIndex(source, -1) {
		desc := ""
		if m[2] >= 0 && m[3] >= 0 {
			desc = source[m[2]:m[3]]
		}
		ident := ""
		if m[4] >= 0 && m[5] >= 0 {
			ident = source[m[4]:m[5]]
		}
		name := zigTestCaseName(desc, ident)
		if name == "" {
			anon++
			name = "anonymous_test_" + strconv.Itoa(anon)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		// The body begins at the opening `{` (the last byte of the match).
		body := extractBraceBody(source, m[1]-1)
		out = append(out, testFunction{
			qname:           name,
			body:            body,
			describeSubject: zigTestSubject(ident),
			lang:            "zig",
		})
	}
	return out
}
