// Standard ML constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `val name = "literal"` / `val name : string = "literal"`
//   - `val name = case OS.Process.getEnv "Y" of SOME s => s | NONE => "default"`
//   - `open Module`
//
// SML's env-fallback shape is verbose (case on Option); we match the
// canonical idiom only.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("sml", sniffSML) }

// smlLiteralRe matches `val name [: string] = "value"`.
var smlLiteralRe = regexp.MustCompile(
	`(?m)^val\s+([a-z_][\w']*)\s*` +
		`(?::\s*string\s*)?` +
		`=\s*"([^"\n\r]{0,512})"`,
)

// smlEnvCaseRe matches `case OS.Process.getEnv "Y" of SOME s => s | NONE => "default"`.
// Capture groups: 1=name, 2=env-var, 3=default.
var smlEnvCaseRe = regexp.MustCompile(
	`(?m)^val\s+([a-z_][\w']*)\s*=\s*case\s+OS\.Process\.getEnv\s+"([^"\n]{1,128})"\s+` +
		`of\s+SOME\s+[a-z_]+\s*=>\s*[a-z_]+\s*\|\s*NONE\s*=>\s*"([^"\n\r]{0,512})"`,
)

// smlOpenRe matches `open Module.Path`.
var smlOpenRe = regexp.MustCompile(
	`(?m)^\s*open\s+([A-Z][\w.]*)`,
)

func sniffSML(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range smlEnvCaseRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range smlLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range smlOpenRe.FindAllStringSubmatchIndex(content, -1) {
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
