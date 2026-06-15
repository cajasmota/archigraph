// Package bazel implements the BUILD/BUCK/WORKSPACE file extractor for
// grafel. It parses Bazel build files as first-class dependency signals,
// emitting BAZEL_DEPENDS_ON edges between declared targets.
//
// # Issue #2183 — Monorepo M6: Bazel BUILD-graph fusion
//
// Build files declare ground-truth module dependencies that call-graph
// inference alone cannot see (generated code, external deps, cross-package
// binaries). Combining BUILD edges with CALLS gives uniquely accurate graphs
// for Bazel monorepos — the enterprise differentiator vs. Sourcegraph (no
// BUILD fusion) and Bazel-native tools (no language-semantic CALLS).
//
// Supported rule kinds (v1 / Bazel):
//
//	py_library, py_binary, py_test
//	java_library, java_binary, java_test
//	go_library, go_binary, go_test   (rules_go)
//	cc_library, cc_binary, cc_test
//	<name>_library / <name>_binary / <name>_test  (catch-all for custom rules)
//
// Pants and Buck are follow-ups (M6a / M6b).
package bazel

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// Rule represents a single Bazel build rule parsed from a BUILD file.
type Rule struct {
	// Kind is the rule function name, e.g. "py_library", "java_binary".
	Kind string
	// Name is the value of the name= attribute, e.g. "server_lib".
	Name string
	// Package is the Bazel package path (directory relative to WORKSPACE),
	// e.g. "services/auth". Together with Name it forms the canonical label
	// "//services/auth:server_lib".
	Package string
	// Deps holds the raw label strings from the deps= attribute.
	// Labels may be absolute ("//other/pkg:target"), short-form (":target"),
	// or external ("@maven//:guava"). External labels are recorded as-is;
	// short-form labels are resolved relative to the rule's own Package.
	Deps []string
	// SourceFile is the BUILD/BUILD.bazel file path relative to repo root
	// where this rule was found.
	SourceFile string
	// StartLine is the 1-based line where the rule definition opens.
	StartLine int
}

// Label returns the canonical Bazel target label for this rule.
// Format: "//<Package>:<Name>"
func (r *Rule) Label() string {
	return fmt.Sprintf("//%s:%s", r.Package, r.Name)
}

// ParseBUILD parses a BUILD or BUILD.bazel file and returns all rules found.
// pkg is the Bazel package path (directory containing the BUILD file,
// relative to the WORKSPACE root). sourceFile is the relative path to the
// BUILD file itself, used as SourceFile on every emitted Rule.
//
// The parser is intentionally permissive: it uses a line-by-line state
// machine rather than a full Starlark AST so it never panics on malformed
// input. Unknown rule kinds are skipped.
func ParseBUILD(content []byte, pkg, sourceFile string) ([]Rule, error) {
	var rules []Rule

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNo := 0

	// State for the current rule block we're accumulating.
	var (
		inRule    bool
		ruleKind  string
		ruleName  string
		ruleDeps  []string
		ruleStart int
		depth     int // parenthesis depth inside the rule block
		depsMode  bool
		depsBuf   strings.Builder
	)

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip blank lines and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if !inRule {
			// Detect the start of a recognised rule invocation.
			// Pattern: <rule_kind>( at start of a non-comment line.
			kind, ok := detectRuleKind(trimmed)
			if !ok {
				continue
			}
			inRule = true
			ruleKind = kind
			ruleName = ""
			ruleDeps = nil
			ruleStart = lineNo
			depth = strings.Count(trimmed, "(") - strings.Count(trimmed, ")")
			depsMode = false
			depsBuf.Reset()
			// The opening line may also contain name= or deps= inline.
			parseLine(trimmed, &ruleName, &ruleDeps, &depsMode, &depsBuf, pkg)

			// Single-line rule: e.g. py_library(name="x", deps=[])
			// The opening line closed the rule block — emit immediately.
			if depth <= 0 && !depsMode {
				if ruleName != "" {
					rules = append(rules, Rule{
						Kind:       ruleKind,
						Name:       ruleName,
						Package:    pkg,
						Deps:       ruleDeps,
						SourceFile: sourceFile,
						StartLine:  ruleStart,
					})
				}
				inRule = false
				depth = 0
			}
			continue
		}

		// We're inside a rule block. Track paren depth.
		depth += strings.Count(trimmed, "(") - strings.Count(trimmed, ")")

		parseLine(trimmed, &ruleName, &ruleDeps, &depsMode, &depsBuf, pkg)

		if depth <= 0 {
			// End of rule block.
			if ruleName != "" {
				rules = append(rules, Rule{
					Kind:       ruleKind,
					Name:       ruleName,
					Package:    pkg,
					Deps:       ruleDeps,
					SourceFile: sourceFile,
					StartLine:  ruleStart,
				})
			}
			inRule = false
			depth = 0
		}
	}
	if err := scanner.Err(); err != nil {
		return rules, fmt.Errorf("bazel parser: scan %s: %w", sourceFile, err)
	}
	return rules, nil
}

// recognisedRuleKinds contains the suffixes that grafel treats as build
// targets. A rule is recognised if its function name ends with one of these
// suffixes (case-sensitive). The leading "<lang>_" prefix is intentionally
// not constrained so that custom rules (e.g. "my_go_library") are included.
var recognisedRuleSuffixes = []string{
	"_library", "_binary", "_test", "_proto_library",
}

// alwaysRecognised is the set of well-known rule kinds without a typed suffix.
var alwaysRecognised = map[string]bool{
	"filegroup":          true,
	"alias":              true,
	"proto_library":      true,
	"grpc_proto_library": true,
}

// detectRuleKind returns (kind, true) if line starts with a recognised Bazel
// rule invocation. It matches "<kind>(" at the beginning of the trimmed line.
func detectRuleKind(line string) (string, bool) {
	idx := strings.IndexByte(line, '(')
	if idx < 1 {
		return "", false
	}
	kind := strings.TrimSpace(line[:idx])
	// Must be a valid identifier (no spaces, no '=').
	if strings.ContainsAny(kind, " \t=") {
		return "", false
	}
	if alwaysRecognised[kind] {
		return kind, true
	}
	for _, suf := range recognisedRuleSuffixes {
		if strings.HasSuffix(kind, suf) {
			return kind, true
		}
	}
	return "", false
}

// parseLine extracts name= and deps= values from a single line within a rule
// block. It updates *name and *deps in place.
//
// deps accumulation is a multi-line affair: once we see "deps = [" we enter
// depsMode and keep reading until the closing "]".
func parseLine(line string, name *string, deps *[]string, depsMode *bool, depsBuf *strings.Builder, pkg string) {
	if *depsMode {
		// Accumulate until we see the closing bracket.
		// If the closing "]" is on this line, only take the content before it.
		if ci := strings.Index(line, "]"); ci >= 0 {
			// Include only the content before "]".
			depsBuf.WriteString(line[:ci])
			depsBuf.WriteByte('\n')
			*deps = parseDepsBuffer(depsBuf.String(), pkg)
			*depsMode = false
			depsBuf.Reset()
		} else {
			depsBuf.WriteString(line)
			depsBuf.WriteByte('\n')
		}
		return
	}

	// name = "..."
	if *name == "" {
		if v, ok := extractStringAttr(line, "name"); ok {
			*name = v
		}
	}

	// deps = [...]  — may start and end on the same line.
	if idx := strings.Index(line, "deps"); idx >= 0 {
		after := line[idx+len("deps"):]
		after = strings.TrimSpace(after)
		if !strings.HasPrefix(after, "=") {
			return
		}
		after = strings.TrimSpace(after[1:])
		// Check if the full list is on one line.
		open := strings.Index(after, "[")
		if open < 0 {
			return
		}
		after = after[open+1:]
		close := strings.Index(after, "]")
		if close >= 0 {
			// Single-line deps list.
			*deps = parseDepsBuffer(after[:close], pkg)
		} else {
			// Multi-line: enter depsMode.
			*depsMode = true
			depsBuf.Reset()
			depsBuf.WriteString(after)
			depsBuf.WriteByte('\n')
		}
	}
}

// extractStringAttr extracts the string value of a Starlark keyword argument
// from a single line, e.g. `name = "my_lib"` → "my_lib".
func extractStringAttr(line, attr string) (string, bool) {
	idx := strings.Index(line, attr)
	if idx < 0 {
		return "", false
	}
	rest := strings.TrimSpace(line[idx+len(attr):])
	if !strings.HasPrefix(rest, "=") {
		return "", false
	}
	rest = strings.TrimSpace(rest[1:])
	// Strip surrounding quotes (double or single).
	if len(rest) >= 2 && (rest[0] == '"' || rest[0] == '\'') {
		q := rest[0]
		end := strings.IndexByte(rest[1:], q)
		if end >= 0 {
			return rest[1 : end+1], true
		}
	}
	return "", false
}

// parseDepsBuffer parses the content between "[" and "]" in a deps list and
// returns the label strings. Short-form labels (":foo") are resolved to
// absolute form relative to pkg.
func parseDepsBuffer(buf, pkg string) []string {
	var out []string
	// Split on commas and newlines; strip quotes, comments, whitespace.
	for _, tok := range strings.FieldsFunc(buf, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	}) {
		tok = strings.TrimSpace(tok)
		// Strip inline comments.
		if ci := strings.Index(tok, "#"); ci >= 0 {
			tok = strings.TrimSpace(tok[:ci])
		}
		// Strip surrounding quotes.
		if len(tok) >= 2 && (tok[0] == '"' || tok[0] == '\'') {
			tok = tok[1 : len(tok)-1]
		}
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		// Resolve short-form labels ":foo" → "//pkg:foo".
		if strings.HasPrefix(tok, ":") {
			tok = fmt.Sprintf("//%s%s", pkg, tok)
		}
		out = append(out, tok)
	}
	return out
}
