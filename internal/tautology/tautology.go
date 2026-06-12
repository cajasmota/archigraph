// Package tautology implements the pure detection core for the
// contract_test_effectiveness MCP tool (#4893, epic #4419).
//
// # The problem it solves
//
// A greenfield rewrite (group `v3`) is audited READ-ONLY against a behavioral
// oracle. The parity gate trusts the v3's own test suite as a witness — a green
// spec is taken as evidence the endpoint behaves. But some specs are
// ORACLE-BLIND: they assert a value against ITSELF, assert a constant true, or
// codify the WRONG expected shape as a literal. Such a spec is green no matter
// what the handler does, so it FALSE-GREENS the parity gate. A real miss: a
// `status_counts` dict→array shape drift PASSED because the spec asserted the
// (wrong) array shape against the handler's array output — the assertion could
// never fail.
//
// # The signal model
//
// This package takes a test's SOURCE TEXT (read by the caller from disk over
// the entity's file+line span) and the assertion vocabulary for the test's
// language/framework, and flags INEFFECTIVE assertions:
//
//   - self_compare    — both sides of an equality assertion are the SAME
//     expression: expect(x).toBe(x), assertEquals(x, x),
//     assertBodyContract(body, body). The assertion is a tautology.
//   - constant_true   — a constant-true assertion: expect(true).toBe(true),
//     assert True, assertTrue(true), expect(1).toBe(1). Always passes.
//   - same_literal    — expected and actual are the SAME literal:
//     expect("ok").toBe("ok"), assertEquals(200, 200). Always passes.
//
// And, best-effort and LOW confidence, a whole-spec signal:
//
//   - no_golden_linkage — the spec body never references the entity / endpoint
//     it claims to test (no symbol the test is named for, no route literal).
//     The caller supplies the linkage terms; this package only checks textual
//     presence. Reported as a low-confidence advisory, never as a hard flag.
//
// # Conservatism
//
// The detector is regex/token based (no full parser) so it favours precision:
// it only flags assertion forms it can match UNAMBIGUOUSLY, and it skips lines
// inside comments. A spec with zero matched ineffective assertions is reported
// effective. The whole-spec no_golden_linkage advisory never on its own makes a
// spec "ineffective" — it is surfaced separately so a human can judge.
package tautology

import (
	"regexp"
	"sort"
	"strings"
)

// Verdict is the per-spec classification.
type Verdict string

const (
	// VerdictIneffective — the spec contains at least one tautological /
	// oracle-blind assertion that cannot fail. The parity gate must NOT trust
	// this spec as a witness.
	VerdictIneffective Verdict = "ineffective"
	// VerdictEffective — no tautological assertion was found; the spec's
	// assertions can distinguish pass from fail.
	VerdictEffective Verdict = "effective"
	// VerdictUnknown — the test's language has no registered assertion
	// vocabulary, or the source window was unreadable; no judgement made.
	VerdictUnknown Verdict = "unknown"
)

// Reason categorises why an individual assertion is ineffective.
type Reason string

const (
	ReasonSelfCompare  Reason = "self_compare"
	ReasonConstantTrue Reason = "constant_true"
	ReasonSameLiteral  Reason = "same_literal"
)

// Finding is one ineffective assertion located in a spec.
type Finding struct {
	Reason Reason `json:"reason"`
	// Line is the 1-based source line (absolute, file-relative) of the
	// assertion, computed by the caller's startLine offset.
	Line int `json:"line"`
	// Snippet is the trimmed source line that triggered the finding.
	Snippet string `json:"snippet"`
	// Detail is a short human note (e.g. the repeated expression / literal).
	Detail string `json:"detail,omitempty"`
}

// Input is the per-spec analysis request.
type Input struct {
	// Language is the lowercased graph language (javascript, typescript,
	// python, go, java, ruby, …). Selects the assertion vocabulary.
	Language string
	// Source is the verbatim test source over the entity's file+line span.
	Source string
	// StartLine is the 1-based file line of Source's first line, so findings can
	// carry absolute line numbers. 1 when the whole file was passed.
	StartLine int
	// LinkageTerms are case-insensitive substrings the spec SHOULD reference if
	// it genuinely tests its target (e.g. the tested function name, the route
	// path "/api/orders"). Empty disables the no_golden_linkage advisory.
	LinkageTerms []string
}

// Result is the per-spec verdict.
type Result struct {
	Verdict  Verdict   `json:"verdict"`
	Findings []Finding `json:"findings"`
	// NoGoldenLinkage is a low-confidence advisory: true when LinkageTerms were
	// supplied AND none of them appears in the source. Never on its own sets
	// VerdictIneffective.
	NoGoldenLinkage bool `json:"no_golden_linkage"`
	// Supported is false when the language has no assertion vocabulary; verdict
	// is then Unknown and findings empty (honest-partial).
	Supported bool `json:"supported"`
}

// Analyze scans one spec's source and returns its effectiveness verdict.
func Analyze(in Input) Result {
	vocab, ok := vocabFor(in.Language)
	if !ok {
		return Result{Verdict: VerdictUnknown, Supported: false}
	}

	start := in.StartLine
	if start < 1 {
		start = 1
	}

	findings := make([]Finding, 0)
	lines := strings.Split(in.Source, "\n")
	for i, raw := range lines {
		line := stripLineComment(raw, vocab.lineComment)
		if strings.TrimSpace(line) == "" {
			continue
		}
		abs := start + i
		if f, ok := vocab.match(line); ok {
			f.Line = abs
			f.Snippet = strings.TrimSpace(raw)
			findings = append(findings, f)
		}
	}

	res := Result{Supported: true, Findings: findings}
	if len(findings) > 0 {
		res.Verdict = VerdictIneffective
	} else {
		res.Verdict = VerdictEffective
	}
	res.NoGoldenLinkage = computeNoGoldenLinkage(in.Source, in.LinkageTerms)
	return res
}

// computeNoGoldenLinkage reports whether none of the linkage terms textually
// appears in the source. Empty terms ⇒ false (advisory disabled).
func computeNoGoldenLinkage(source string, terms []string) bool {
	if len(terms) == 0 {
		return false
	}
	low := strings.ToLower(source)
	for _, t := range terms {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if strings.Contains(low, t) {
			return false
		}
	}
	return true
}

// stripLineComment removes a trailing line comment so an assertion commented out
// or annotated does not cause false matches. Best-effort: it does not track
// string literals, so a comment marker inside a string can over-trim — acceptable
// because that only makes the matcher MORE conservative (fewer flags).
func stripLineComment(line, marker string) string {
	if marker == "" {
		return line
	}
	if i := strings.Index(line, marker); i >= 0 {
		return line[:i]
	}
	return line
}

// ---------------------------------------------------------------------------
// Assertion vocabularies (per language family)
// ---------------------------------------------------------------------------

// vocab is the set of compiled matchers for one language family.
type vocab struct {
	lineComment string
	// constTrue matches a whole-line constant-true assertion.
	constTrue []*regexp.Regexp
	// pairForms extract the two compared operands of an equality assertion so
	// the engine can test self-compare / same-literal. Each regexp must expose
	// two capture groups (left, right).
	pairForms []*regexp.Regexp
}

// match runs the vocabulary against one (comment-stripped) line.
func (v vocab) match(line string) (Finding, bool) {
	for _, re := range v.constTrue {
		if re.MatchString(line) {
			return Finding{Reason: ReasonConstantTrue, Detail: "constant-true assertion always passes"}, true
		}
	}
	for _, re := range v.pairForms {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		left := normalizeOperand(m[1])
		right := normalizeOperand(m[2])
		if left == "" || right == "" {
			continue
		}
		if left == right {
			if isLiteral(left) {
				return Finding{Reason: ReasonSameLiteral, Detail: "expected == actual literal " + left}, true
			}
			return Finding{Reason: ReasonSelfCompare, Detail: "both sides are the same expression: " + left}, true
		}
	}
	return Finding{}, false
}

// normalizeOperand trims surrounding whitespace and collapses internal spaces so
// `foo .bar` and `foo.bar` compare equal, while preserving literal content.
func normalizeOperand(s string) string {
	s = strings.TrimSpace(s)
	// Collapse runs of internal whitespace to a single space.
	s = whitespaceRun.ReplaceAllString(s, " ")
	// Remove spaces around member-access dots and call parens to canonicalise
	// `obj . prop` vs `obj.prop`.
	s = strings.ReplaceAll(s, " .", ".")
	s = strings.ReplaceAll(s, ". ", ".")
	return s
}

var whitespaceRun = regexp.MustCompile(`\s+`)

// isLiteral reports whether an operand is a primitive literal (string, number,
// true/false/null/None) rather than an identifier/expression. Used to label a
// same-value match as same_literal vs self_compare.
func isLiteral(s string) bool {
	if s == "" {
		return false
	}
	switch s {
	case "true", "false", "null", "nil", "None", "True", "False", "undefined":
		return true
	}
	if numberLit.MatchString(s) {
		return true
	}
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`")) {
		return true
	}
	return false
}

var numberLit = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// operand is the regex fragment for one assertion operand inside a `(a, b)`
// two-argument call: a chunk that does NOT contain a comma or paren, so the two
// args split cleanly. Nested calls with commas are out of scope (they would not
// be a self-compare we can prove anyway); a no-arg method call like `body.json`
// without parens still matches.
const operand = `([^,()]+?)`

// exprOperand is the fragment for an operand of an infix `==` comparison, where
// the two sides are NOT separated by a comma. It permits a single level of
// balanced parens (so `resp.json()` is one operand) but no commas, keeping the
// split unambiguous for the common `assert a.b() == a.b()` shape.
const exprOperand = `([\w$.\[\]"'` + "`" + `]+(?:\([^()]*\))?(?:[\w$.\[\]]*)?)`

// callArgs is the fragment for `(left, right)` two-argument assertions.
const callArgs = `\(\s*` + operand + `\s*,\s*` + operand + `\s*\)`

// vocabFor returns the assertion vocabulary for a graph language. The bool is
// false for languages without a registered vocabulary (honest-partial).
func vocabFor(lang string) (vocab, bool) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "javascript", "typescript", "js", "ts", "jsx", "tsx":
		return jstsVocab, true
	case "python", "py":
		return pythonVocab, true
	case "go", "golang":
		return goVocab, true
	case "java", "kotlin", "kt":
		return javaVocab, true
	case "ruby", "rb":
		return rubyVocab, true
	default:
		return vocab{}, false
	}
}

// --- JS/TS (Jest / vitest — the live NestJS spec style) ---
//
// expect(LEFT).toBe(RIGHT) / toEqual / toStrictEqual / toMatchObject /
// toContainEqual. assertBodyContract(LEFT, RIGHT) is the custom contract helper
// the live suite uses; we treat any `xxx(LEFT, RIGHT)` Contract/Equal helper as
// a pair form too.
var jstsVocab = vocab{
	lineComment: "//",
	constTrue: []*regexp.Regexp{
		// expect(true|1|"...").toBe(true|1|"...") where both are the SAME truthy
		// constant — the canonical no-op. Kept narrow to avoid flagging real
		// boolean assertions like expect(x.ok).toBe(true).
		regexp.MustCompile(`expect\(\s*(true|1)\s*\)\s*\.\s*(?:toBe|toEqual|toStrictEqual)\(\s*(true|1)\s*\)`),
	},
	pairForms: []*regexp.Regexp{
		// expect(LEFT).toBe/toEqual/...(RIGHT)
		regexp.MustCompile(`expect\(\s*` + operand + `\s*\)\s*\.\s*(?:toBe|toEqual|toStrictEqual|toMatchObject|toContainEqual)\(\s*` + operand + `\s*\)`),
		// assertBodyContract(LEFT, RIGHT) and any *Contract / assertEqual* helper
		// taking two args.
		regexp.MustCompile(`(?:assertBodyContract|assertEqual|assertEquals|assertContract|expectEqual)` + callArgs),
	},
}

// --- Python (pytest assert + unittest) ---
var pythonVocab = vocab{
	lineComment: "#",
	constTrue: []*regexp.Regexp{
		regexp.MustCompile(`^\s*assert\s+True\s*(?:#.*)?$`),
		regexp.MustCompile(`^\s*assert\s+1\s*(?:==\s*1\s*)?$`),
		regexp.MustCompile(`\bassertTrue\(\s*True\s*\)`),
		regexp.MustCompile(`\bassertTrue\(\s*1\s*\)`),
	},
	pairForms: []*regexp.Regexp{
		// assert LEFT == RIGHT
		regexp.MustCompile(`^\s*assert\s+` + exprOperand + `\s*==\s*` + exprOperand + `\s*$`),
		// self.assertEqual(LEFT, RIGHT)
		regexp.MustCompile(`assertEqual` + callArgs),
	},
}

// --- Go (testing — if got != want style + assert.Equal(t, a, b)) ---
var goVocab = vocab{
	lineComment: "//",
	constTrue: []*regexp.Regexp{
		regexp.MustCompile(`if\s+true\s*(?:!=\s*true\s*)?\{`),
	},
	pairForms: []*regexp.Regexp{
		// assert.Equal(t, LEFT, RIGHT) / require.Equal(t, LEFT, RIGHT)
		regexp.MustCompile(`(?:assert|require)\.Equal\(\s*[^,]+,\s*` + operand + `\s*,\s*` + operand + `\s*\)`),
	},
}

// --- Java / Kotlin (JUnit / AssertJ) ---
var javaVocab = vocab{
	lineComment: "//",
	constTrue: []*regexp.Regexp{
		regexp.MustCompile(`assertTrue\(\s*true\s*\)`),
		regexp.MustCompile(`assertThat\(\s*true\s*\)\s*\.\s*isTrue\(\)`),
	},
	pairForms: []*regexp.Regexp{
		// assertEquals(LEFT, RIGHT)
		regexp.MustCompile(`assertEquals` + callArgs),
		// assertThat(LEFT).isEqualTo(RIGHT)
		regexp.MustCompile(`assertThat\(\s*` + operand + `\s*\)\s*\.\s*isEqualTo\(\s*` + operand + `\s*\)`),
	},
}

// --- Ruby (RSpec / minitest) ---
var rubyVocab = vocab{
	lineComment: "#",
	constTrue: []*regexp.Regexp{
		regexp.MustCompile(`expect\(\s*true\s*\)\s*\.\s*to\s+eq\(\s*true\s*\)`),
		regexp.MustCompile(`expect\(\s*true\s*\)\s*\.\s*to\s+be_truthy`),
		regexp.MustCompile(`assert\s+true\s*$`),
	},
	pairForms: []*regexp.Regexp{
		// expect(LEFT).to eq(RIGHT) / to eql(RIGHT)
		regexp.MustCompile(`expect\(\s*` + operand + `\s*\)\s*\.\s*to\s+eq(?:l)?\(\s*` + operand + `\s*\)`),
		// assert_equal LEFT, RIGHT
		regexp.MustCompile(`assert_equal\s+` + operand + `\s*,\s*` + operand + `\s*$`),
	},
}

// SortFindings orders findings by line then reason for deterministic output.
func SortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Line != fs[j].Line {
			return fs[i].Line < fs[j].Line
		}
		return fs[i].Reason < fs[j].Reason
	})
}
