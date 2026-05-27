// Erlang constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises module-level binding shapes:
//   - `-define(NAME, "literal").` — preprocessor constants
//   - `-define(NAME, os:getenv("Y", "default")).` — env-fallback
//   - `-import(mod, [func/arity, ...]).` — cross-module reference
//
// Top-level `Name = "value"` assignments do not exist in Erlang outside
// function bodies; constants are conventionally defines.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("erlang", sniffErlang) }

// erlangDefineLiteralRe matches `-define(NAME, "value").`.
// Capture groups: 1=name, 2=value.
var erlangDefineLiteralRe = regexp.MustCompile(
	`(?m)^-define\(\s*([A-Z_][\w]*)\s*,\s*"([^"\n\r]{0,512})"\s*\)\s*\.`,
)

// erlangDefineGetenvRe matches `-define(NAME, os:getenv("Y", "default")).`.
// Capture groups: 1=name, 2=env-var, 3=default.
var erlangDefineGetenvRe = regexp.MustCompile(
	`(?m)^-define\(\s*([A-Z_][\w]*)\s*,\s*os:getenv\s*\(\s*"([^"\n]{1,128})"\s*,\s*"([^"\n\r]{0,512})"\s*\)\s*\)\s*\.`,
)

// erlangImportRe matches `-import(Module, [...]).`. Only the module is
// captured here; each imported function ident is recorded as a separate
// binding pointing at the same module.
var erlangImportRe = regexp.MustCompile(
	`(?m)^-import\(\s*([a-z][\w]*)\s*,\s*\[([^\]]+)\]\s*\)\s*\.`,
)

// erlangFunRefRe matches `name/arity` entries inside the import list.
var erlangFunRefRe = regexp.MustCompile(`([a-z_][\w]*)\s*/\s*\d+`)

func sniffErlang(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range erlangDefineGetenvRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range erlangDefineLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range erlangImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		module := content[m[2]:m[3]]
		list := content[m[4]:m[5]]
		line := lineOfOffset(content, m[2])
		for _, fm := range erlangFunRefRe.FindAllStringSubmatchIndex(list, -1) {
			if len(fm) < 4 {
				continue
			}
			name := strings.TrimSpace(list[fm[2]:fm[3]])
			out = append(out, Binding{
				Ident:        name,
				Line:         line,
				Provenance:   ProvenanceCrossFile,
				Confidence:   0.6,
				ImportSource: module,
			})
		}
	}
	return out
}
