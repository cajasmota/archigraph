package patterns

import (
	"regexp"

	"github.com/cajasmota/grafel/internal/types"
)

// assertionLibDetector detects F# assertion-library usage (#5114 — the non-db
// tail of #4941). It emits one `SCOPE.Pattern` / `assertion_lib` record per
// distinct assertion library a file uses, mirroring the property-test-record
// precedent (property_test_detector.go) — a structural marker that downstream
// test-quality / coverage consumers can key the test's assertion style off.
//
// Two dominant F# assertion DSLs are recognised:
//
//	Unquote — quoted-expression assertions, where the asserted expression is
//	wrapped in F# code quotations `<@ … @>`:
//	  test <@ add 2 2 = 4 @>
//	  raises<ArgumentException> <@ parse "x" @>
//
//	FsUnit — the fluent `should` operator on a piped actual value:
//	  result |> should equal 4
//	  result |> should be (greaterThan 0)
//	  (fun () -> parse "x") |> should throw typeof<exn>
//
// Both are F#-only gated (language == "fsharp") so the F#-shaped tokens never
// misfire on another language whose source happens to contain a `should equal`
// or `<@` substring.
type assertionLibDetector struct{}

var (
	// aldUnquoteRE matches an Unquote assertion: the `test`/`testRaises`/`raises`
	// driver immediately followed by an F# code-quotation opener `<@`. The `<@`
	// opener is the unambiguous Unquote signal (it is the F# quotation syntax
	// Unquote is built on); requiring the driver in front keeps it off arbitrary
	// quotations.
	aldUnquoteRE = regexp.MustCompile(`\b(?:test|testRaises|raises(?:When)?|reduceFully)\b[^\n\r]{0,40}<@`)

	// aldFsUnitRE matches an FsUnit fluent assertion: a `|> should <matcher>`
	// pipe. The `|> should` pipe-into-`should` shape is the FsUnit signature
	// (`should equal` / `should be` / `should throw` / `should contain` / …).
	aldFsUnitRE = regexp.MustCompile(`\|>\s*should\s+(?:equal|be|throw|contain|haveLength|startWith|endWith|not'|not)\b`)
)

func (a *assertionLibDetector) Category() string { return "assertion_lib" }

func (a *assertionLibDetector) AppliesTo(src string) bool {
	return aldUnquoteRE.MatchString(src) || aldFsUnitRE.MatchString(src)
}

func (a *assertionLibDetector) Detect(filePath, language, src string) []types.EntityRecord {
	// F#-only gate: the assertion-DSL tokens are F#-specific shapes, but a
	// `<@`/`should` substring could appear in other languages — restrict to F#.
	if language != "fsharp" {
		return nil
	}
	var results []types.EntityRecord
	seen := map[string]bool{}

	emit := func(key, library string, line int) {
		if seen[key] {
			return
		}
		seen[key] = true
		results = append(results, makeEntity(filePath,
			"assertion_lib_"+library, "SCOPE.Pattern", "assertion_lib", language, line,
			map[string]string{"kind": "assertion_lib", "library": library}))
	}

	if m := aldUnquoteRE.FindStringIndex(src); m != nil {
		emit("fsharp:unquote", "unquote", lineOf(src, m[0]))
	}
	if m := aldFsUnitRE.FindStringIndex(src); m != nil {
		emit("fsharp:fsunit", "fsunit", lineOf(src, m[0]))
	}

	return results
}

func init() {
	Register(&assertionLibDetector{})
}
