// Nim constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `const NAME* = "literal"` / `const NAME = "literal"`
//   - `let NAME = "literal"`
//   - `let NAME = getEnv("Y", "default")` / `let NAME = os.getEnv("Y", "default")`
//   - `import module` / `import module1, module2`
//
// Nim's `const` is compile-time-only and the canonical constant form.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("nim", sniffNim) }

// nimLiteralRe matches `(const|let) NAME[*] = "value"` at the start of a
// line. The `*` is Nim's export marker.
//
// Capture groups: 1=name, 2=value.
var nimLiteralRe = regexp.MustCompile(
	`(?m)^(?:const|let)\s+([A-Za-z_][\w]*)\*?\s*=\s*"([^"\n\r]{0,512})"`,
)

// nimGetEnvRe matches `let NAME = [os.]getEnv("Y", "default")`.
// Capture groups: 1=name, 2=env-var, 3=default.
var nimGetEnvRe = regexp.MustCompile(
	`(?m)^(?:const|let)\s+([A-Za-z_][\w]*)\*?\s*=\s*` +
		`(?:os\.)?getEnv\s*\(\s*"([^"\n]{1,128})"\s*,\s*"([^"\n\r]{0,512})"\s*\)`,
)

// nimImportRe matches `import a, b, c` (one or more modules per line).
// `[^\S\n]` is "horizontal whitespace only" — keeps the match on a single
// physical line so consecutive `import` statements don't get merged.
var nimImportRe = regexp.MustCompile(
	`(?m)^import[^\S\n]+([\w/., \t]+)$`,
)

func sniffNim(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range nimGetEnvRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range nimLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range nimImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		line := lineOfOffset(content, m[2])
		modules := strings.Split(content[m[2]:m[3]], ",")
		for _, mod := range modules {
			mod = strings.TrimSpace(mod)
			if mod == "" {
				continue
			}
			local := mod
			if slash := strings.LastIndex(mod, "/"); slash >= 0 {
				local = mod[slash+1:]
			}
			if !isJSIdent(local) {
				continue
			}
			out = append(out, Binding{
				Ident:        local,
				Line:         line,
				Provenance:   ProvenanceCrossFile,
				Confidence:   0.6,
				ImportSource: mod,
			})
		}
	}
	return out
}
