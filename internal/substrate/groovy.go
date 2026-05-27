// Groovy constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `[static] final String X = "literal"` / `def X = "literal"`
//   - `[static] final String X = System.getenv("Y") ?: "default"`
//   - `import a.b.C` / `import static a.b.C.MEMBER`
//
// Intentionally narrow per #2763: String-typed and `def`-typed top-level
// declarations only.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("groovy", sniffGroovy) }

// groovyLiteralRe matches `[mods] [static] final String NAME = "value"` or
// `def NAME = "value"`. Trailing `;` is optional in Groovy.
//
// Capture groups: 1=name, 2=double-quoted value, 3=single-quoted value.
var groovyLiteralRe = regexp.MustCompile(
	`(?m)^(?:(?:public|private|protected)\s+)?(?:static\s+)?(?:final\s+String|def|String)\s+` +
		`([A-Za-z_$][\w$]*)\s*=\s*(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*;?`,
)

// groovyEnvElvisRe matches `... = System.getenv("Y") ?: "default"`. Groovy
// elides parens around getenv too, but we require the explicit form here
// for safety.
//
// Capture groups: 1=name, 2=env-var, 3=double-quoted default, 4=single-quoted default.
var groovyEnvElvisRe = regexp.MustCompile(
	`(?m)^(?:(?:public|private|protected)\s+)?(?:static\s+)?(?:final\s+String|def|String)\s+` +
		`([A-Za-z_$][\w$]*)\s*=\s*System\.getenv\s*\(\s*["']([^"']{1,128})["']\s*\)\s*` +
		`\?\:\s*(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')`,
)

// groovyImportRe matches `import [static] a.b.C` (with optional trailing ;).
// Capture group 1 is the fully-qualified path; the local binding name is
// the last dotted segment.
var groovyImportRe = regexp.MustCompile(
	`(?m)^\s*import\s+(?:static\s+)?([\w.]+)\s*;?`,
)

func sniffGroovy(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range groovyEnvElvisRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 {
			continue
		}
		def := pickQuoted(content, m, 6, 8)
		out = append(out, Binding{
			Ident:      content[m[2]:m[3]],
			Line:       lineOfOffset(content, m[2]),
			Value:      def,
			EnvVar:     content[m[4]:m[5]],
			Provenance: ProvenanceEnvFallback,
			Confidence: 0.85,
		})
		seen[m[2]] = true
	}
	for _, m := range groovyLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 || seen[m[2]] {
			continue
		}
		value := pickQuoted(content, m, 4, 6)
		out = append(out, Binding{
			Ident:      content[m[2]:m[3]],
			Line:       lineOfOffset(content, m[2]),
			Value:      value,
			Provenance: ProvenanceLiteral,
			Confidence: 1.0,
		})
	}
	for _, m := range groovyImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		qualified := content[m[2]:m[3]]
		dot := strings.LastIndex(qualified, ".")
		if dot < 0 || dot == len(qualified)-1 {
			continue
		}
		local := qualified[dot+1:]
		module := qualified[:dot]
		if local == "*" {
			continue
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
