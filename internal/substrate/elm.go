// Elm constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `name = "literal"` (with or without `name : String` annotation on the
//     preceding line)
//   - `import Foo` / `import Foo.Bar as Baz`
//
// Elm has no env-var access at compile time — values come in via flags or
// ports at runtime. The env_fallback shape is therefore not_applicable; the
// sniffer only emits literal and cross-file bindings.
package substrate

import "regexp"

func init() { Register("elm", sniffElm) }

// elmLiteralRe matches a top-level `name = "value"`. Elm uses two-space
// indentation inside `let` blocks; the `^` anchor excludes those.
//
// Capture groups: 1=name, 2=value.
var elmLiteralRe = regexp.MustCompile(
	`(?m)^([a-z][\w]*)\s*=\s*"([^"\n\r]{0,512})"\s*$`,
)

// elmImportRe matches `import Mod[.Sub] [as Alias] [exposing (..)]`.
// Capture groups: 1=module path, 2=optional alias.
var elmImportRe = regexp.MustCompile(
	`(?m)^import\s+([A-Z][\w.]*)(?:\s+as\s+([A-Z][\w]*))?`,
)

func sniffElm(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding

	for _, m := range elmLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range elmImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		module := content[m[2]:m[3]]
		var local string
		if m[4] >= 0 {
			local = content[m[4]:m[5]]
		} else {
			local = module
			for i := len(module) - 1; i >= 0; i-- {
				if module[i] == '.' {
					local = module[i+1:]
					break
				}
			}
		}
		out = append(out, Binding{
			Ident:        local,
			Line:         lineOfOffset(content, m[2]),
			Provenance:   ProvenanceCrossFile,
			Confidence:   0.6,
			ImportSource: module,
		})
	}
	return out
}
