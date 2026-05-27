// Scala constant-binding sniffer (#2762 Phase 0 T2).
//
// Recognises top-level binding shapes:
//   - `val X = "literal"` / `val X: String = "literal"`
//   - `final val X = "literal"`
//   - `val X = sys.env.getOrElse("Y", "default")`
//   - `import com.example.X` / `import com.example.{X, Y}` (cross-file binding)
//   - `import com.example.{X => Y}` (rebinding)
//
// Intentionally narrow per #2762: top-level only. Object-scoped vals and
// class members fall outside Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("scala", sniffScala) }

// scLiteralRe matches `[final ]val NAME[: String] = "value"`.
//
// Capture groups: 1=name, 2=value.
var scLiteralRe = regexp.MustCompile(
	`(?m)^\s*(?:final\s+)?val\s+([A-Za-z_][\w]*)\s*` +
		`(?::\s*String\s*)?=\s*"([^"\n\r]{0,512})"`,
)

// scEnvGetOrElseRe matches `val NAME = sys.env.getOrElse("Y", "default")`.
//
// Capture groups: 1=name, 2=env-var, 3=default literal.
var scEnvGetOrElseRe = regexp.MustCompile(
	`(?m)^\s*(?:final\s+)?val\s+([A-Za-z_][\w]*)\s*` +
		`(?::\s*String\s*)?=\s*sys\.env\.getOrElse\s*\(\s*"([^"]{1,128})"\s*,\s*` +
		`"([^"\n\r]{0,512})"\s*\)`,
)

// scImportRe matches `import com.example.X`, `import com.example.{X, Y}`,
// and `import com.example.{X => Y}`.
//
// Capture groups: 1=base path, 2=braced multi-spec (optional).
var scImportRe = regexp.MustCompile(
	`(?m)^\s*import\s+([\w.]+?)(?:\.\{([^}]+)\})?\s*$`,
)

// sniffScala implements the Scala Binding sniffer.
func sniffScala(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range scEnvGetOrElseRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range scLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range scImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		base := content[m[2]:m[3]]
		braced := ""
		if m[4] >= 0 {
			braced = content[m[4]:m[5]]
		}
		line := lineOfOffset(content, m[2])
		if braced != "" {
			for _, spec := range strings.Split(braced, ",") {
				spec = strings.TrimSpace(spec)
				if spec == "" || spec == "_" {
					continue
				}
				local := spec
				remote := spec
				if arrIdx := strings.Index(spec, "=>"); arrIdx > 0 {
					remote = strings.TrimSpace(spec[:arrIdx])
					local = strings.TrimSpace(spec[arrIdx+2:])
				}
				if local == "" || local == "_" {
					continue
				}
				out = append(out, Binding{
					Ident:        local,
					Line:         line,
					Provenance:   ProvenanceCrossFile,
					Confidence:   0.6,
					ImportSource: base + "." + remote,
				})
			}
			continue
		}
		local := base
		module := base
		if dot := strings.LastIndex(base, "."); dot >= 0 {
			local = base[dot+1:]
			module = base[:dot]
		}
		if local == "" || local == "_" {
			continue
		}
		out = append(out, Binding{
			Ident:        local,
			Line:         line,
			Provenance:   ProvenanceCrossFile,
			Confidence:   0.6,
			ImportSource: module,
		})
	}

	return out
}
