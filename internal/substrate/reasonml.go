// ReasonML constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `let name = "literal";` / `let name: string = "literal";`
//   - `open Module;`
//
// ReasonML's env-var idiom mirrors OCaml's (Sys.getenv_opt), but the
// ReasonML community largely deprecated the syntax in favour of ReScript.
// Env-fallback is reported as `partial` in the registry because we only
// handle direct literals here.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("reasonml", sniffReasonML) }

// reasonLiteralRe matches `let name [: string] = "value";`.
var reasonLiteralRe = regexp.MustCompile(
	`(?m)^let\s+([a-z_][\w']*)\s*` +
		`(?::\s*string\s*)?` +
		`=\s*"([^"\n\r]{0,512})"\s*;`,
)

// reasonOpenRe matches `open Module.Path;`.
var reasonOpenRe = regexp.MustCompile(
	`(?m)^\s*open\s+([A-Z][\w.]*)\s*;`,
)

func sniffReasonML(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	for _, m := range reasonLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		out = append(out, Binding{
			Ident:      content[m[2]:m[3]],
			Line:       lineOfOffset(content, m[2]),
			Value:      content[m[4]:m[5]],
			Provenance: ProvenanceLiteral,
			Confidence: 1.0,
		})
	}
	for _, m := range reasonOpenRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		qualified := content[m[2]:m[3]]
		local := qualified
		if dot := strings.LastIndex(qualified, "."); dot >= 0 {
			local = qualified[dot+1:]
		}
		out = append(out, Binding{
			Ident:        local,
			Line:         lineOfOffset(content, m[2]),
			Provenance:   ProvenanceCrossFile,
			Confidence:   0.6,
			ImportSource: qualified,
		})
	}
	return out
}
