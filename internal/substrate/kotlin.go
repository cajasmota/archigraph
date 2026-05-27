// Kotlin constant-binding sniffer (#2762 Phase 0 T2).
//
// Recognises top-level binding shapes:
//   - `const val X = "literal"`
//   - `val X = "literal"` / `val X: String = "literal"`
//   - `val X = System.getenv("Y") ?: "default"`
//   - `import com.example.X` / `import com.example.X as Y` (cross-file binding)
//
// Intentionally narrow per #2762: top-level only. Companion-object constants
// and class members fall outside Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("kotlin", sniffKotlin) }

// ktLiteralRe matches `[const ]val NAME[: String] = "value"`.
//
// Capture groups: 1=name, 2=value.
var ktLiteralRe = regexp.MustCompile(
	`(?m)^\s*(?:public\s+|private\s+|protected\s+|internal\s+|)?` +
		`(?:const\s+)?val\s+([A-Za-z_][\w]*)\s*` +
		`(?::\s*String\s*)?=\s*"([^"\n\r]{0,512})"`,
)

// ktEnvElvisRe matches `val NAME = System.getenv("Y") ?: "default"`.
//
// Capture groups: 1=name, 2=env-var, 3=default literal.
var ktEnvElvisRe = regexp.MustCompile(
	`(?m)^\s*(?:public\s+|private\s+|protected\s+|internal\s+|)?` +
		`(?:const\s+)?val\s+([A-Za-z_][\w]*)\s*` +
		`(?::\s*String\s*)?=\s*System\.getenv\s*\(\s*"([^"]{1,128})"\s*\)\s*\?:\s*` +
		`"([^"\n\r]{0,512})"`,
)

// ktImportRe matches `import com.example.Foo` and `import com.example.Foo as Bar`.
//
// Capture groups: 1=qualified path, 2=alias (optional).
var ktImportRe = regexp.MustCompile(
	`(?m)^\s*import\s+([\w.]+)(?:\s+as\s+([A-Za-z_][\w]*))?\s*$`,
)

// sniffKotlin implements the Kotlin Binding sniffer.
func sniffKotlin(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range ktEnvElvisRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 {
			continue
		}
		name := content[m[2]:m[3]]
		envVar := content[m[4]:m[5]]
		def := content[m[6]:m[7]]
		out = append(out, Binding{
			Ident:      name,
			Line:       lineOfOffset(content, m[2]),
			Value:      def,
			EnvVar:     envVar,
			Provenance: ProvenanceEnvFallback,
			Confidence: 0.85,
		})
		seen[m[2]] = true
	}

	for _, m := range ktLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		if seen[m[2]] {
			continue
		}
		name := content[m[2]:m[3]]
		value := content[m[4]:m[5]]
		out = append(out, Binding{
			Ident:      name,
			Line:       lineOfOffset(content, m[2]),
			Value:      value,
			Provenance: ProvenanceLiteral,
			Confidence: 1.0,
		})
	}

	for _, m := range ktImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		qualified := content[m[2]:m[3]]
		var alias string
		if m[4] >= 0 {
			alias = content[m[4]:m[5]]
		}
		local := alias
		module := qualified
		if local == "" {
			if dot := strings.LastIndex(qualified, "."); dot >= 0 {
				local = qualified[dot+1:]
				module = qualified[:dot]
			} else {
				local = qualified
			}
		}
		if local == "" || local == "*" {
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
