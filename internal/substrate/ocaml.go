// OCaml constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `let name = "literal"` / `let name : string = "literal"`
//   - `let name = Option.value (Sys.getenv_opt "Y") ~default:"default"`
//   - `let name = try Sys.getenv "Y" with Not_found -> "default"`
//   - `open Module` / `open Module.Sub`
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("ocaml", sniffOCaml) }

// ocamlLiteralRe matches a top-level `let name [: string] = "value"`.
// Capture groups: 1=name, 2=value.
var ocamlLiteralRe = regexp.MustCompile(
	`(?m)^let\s+([a-z_][\w']*)\s*` +
		`(?::\s*string\s*)?` +
		`=\s*"([^"\n\r]{0,512})"`,
)

// ocamlEnvOptionValueRe matches `Option.value (Sys.getenv_opt "Y") ~default:"default"`.
// Capture groups: 1=name, 2=env-var, 3=default.
var ocamlEnvOptionValueRe = regexp.MustCompile(
	`(?m)^let\s+([a-z_][\w']*)\s*=\s*Option\.value\s*\(\s*Sys\.getenv_opt\s+` +
		`"([^"\n]{1,128})"\s*\)\s*~default:\s*"([^"\n\r]{0,512})"`,
)

// ocamlEnvTryRe matches `try Sys.getenv "Y" with Not_found -> "default"`.
// Capture groups: 1=name, 2=env-var, 3=default.
var ocamlEnvTryRe = regexp.MustCompile(
	`(?m)^let\s+([a-z_][\w']*)\s*=\s*try\s+Sys\.getenv\s+"([^"\n]{1,128})"\s+` +
		`with\s+Not_found\s*->\s*"([^"\n\r]{0,512})"`,
)

// ocamlOpenRe matches `open Module.Path`.
var ocamlOpenRe = regexp.MustCompile(
	`(?m)^\s*open\s+([A-Z][\w.]*)`,
)

func sniffOCaml(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range ocamlEnvOptionValueRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range ocamlEnvTryRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 || seen[m[2]] {
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
	for _, m := range ocamlLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range ocamlOpenRe.FindAllStringSubmatchIndex(content, -1) {
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
