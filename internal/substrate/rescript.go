// ReScript constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `let x = "literal"` / `let x: string = "literal"`
//   - `let x = Js.Dict.get(Node.Process.process["env"], "Y")->Belt.Option.getWithDefault("default")`
//     (matched as env-fallback)
//   - `open Module`
//
// ReScript is OCaml-derived but uses braces and no trailing semicolons.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("rescript", sniffReScript) }

// rescriptLiteralRe matches `let name [: string] = "value"` (no trailing `;`).
var rescriptLiteralRe = regexp.MustCompile(
	`(?m)^let\s+([a-z_][\w']*)\s*` +
		`(?::\s*string\s*)?` +
		`=\s*"([^"\n\r]{0,512})"`,
)

// rescriptEnvRe matches `Js.Dict.get(Node.Process.process["env"], "Y")->Belt.Option.getWithDefault("default")`.
// Capture groups: 1=name, 2=env-var, 3=default.
var rescriptEnvRe = regexp.MustCompile(
	`(?m)^let\s+([a-z_][\w']*)\s*=\s*` +
		`Js\.Dict\.get\s*\(\s*Node\.Process\.process\["env"\]\s*,\s*"([^"\n]{1,128})"\s*\)\s*` +
		`->\s*Belt\.Option\.getWithDefault\s*\(\s*"([^"\n\r]{0,512})"\s*\)`,
)

// rescriptOpenRe matches `open Module[.Sub]`.
var rescriptOpenRe = regexp.MustCompile(
	`(?m)^\s*open\s+([A-Z][\w.]*)`,
)

func sniffReScript(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range rescriptEnvRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 {
			continue
		}
		out = append(out, Binding{
			Ident:      content[m[2]:m[3]],
			Line:       lineOfOffset(content, m[2]),
			Value:      content[m[6]:m[7]],
			EnvVar:     content[m[4]:m[5]],
			Provenance: ProvenanceEnvFallback,
			Confidence: 0.85,
		})
		seen[m[2]] = true
	}
	for _, m := range rescriptLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 || seen[m[2]] {
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
	for _, m := range rescriptOpenRe.FindAllStringSubmatchIndex(content, -1) {
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
