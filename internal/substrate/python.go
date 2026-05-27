// Python constant-binding sniffer (#2761 Phase 0 T1).
//
// Recognises module-level binding shapes:
//   - X = "literal"  /  X = 'literal'
//   - X: str = "literal"  (PEP 526 annotated assignments)
//   - X = os.getenv("Y", "default")
//   - X = os.environ.get("Y", "default")
//   - X = os.environ.get("Y") or "default"
//   - from .mod import X  / from package.mod import X as Y  (cross-file)
//
// Intentionally narrow per #2761: module-level only (no indentation before
// `=`). Class- and function-scoped constants do not participate in Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("python", sniffPython) }

// pyLiteralRe matches a module-level `NAME = "value"` (or `NAME: T = "value"`)
// assignment. The `^` anchor requires no leading whitespace so class and
// function bodies are excluded.
//
// Capture groups: 1=name, 2=double-quoted value, 3=single-quoted value.
var pyLiteralRe = regexp.MustCompile(
	`(?m)^([A-Za-z_][\w]*)\s*` +
		`(?::\s*[\w\[\],\s.]+\s*)?` +
		`=\s*` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*(?:#.*)?$`,
)

// pyGetenvRe matches `os.getenv("Y", "default")` and the `os.environ.get`
// equivalents, on either form `X = os.getenv(...)` or `X = os.environ.get(...)`.
//
// Capture groups: 1=name, 2=env-var name, 3=double-quoted default,
// 4=single-quoted default. Default may be absent — handled by the
// alternate regex below.
var pyGetenvRe = regexp.MustCompile(
	`(?m)^([A-Za-z_][\w]*)\s*=\s*` +
		`os\.(?:getenv|environ\.get)\s*\(\s*` +
		`["']([^"']{1,128})["']` +
		`(?:\s*,\s*(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})'))?` +
		`\s*\)`,
)

// pyGetenvOrRe matches `X = os.getenv("Y") or "default"`.
//
// Capture groups: 1=name, 2=env-var, 3=double-quoted default,
// 4=single-quoted default.
var pyGetenvOrRe = regexp.MustCompile(
	`(?m)^([A-Za-z_][\w]*)\s*=\s*` +
		`os\.(?:getenv|environ\.get)\s*\(\s*["']([^"']{1,128})["']\s*\)\s*` +
		`or\s+` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')`,
)

// pyImportFromRe matches `from <module> import a, b as c, ...`. Captures
// the module path and the comma-separated specifier list.
var pyImportFromRe = regexp.MustCompile(
	`(?m)^from\s+([\w.]+)\s+import\s+([^\n#]+)`,
)

// sniffPython implements the Python Binding sniffer.
func sniffPython(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	// Track which names we've already bound to avoid double-emitting when
	// the literal regex also matches a multi-form pattern.
	seen := map[int]bool{}

	for _, m := range pyGetenvRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 {
			continue
		}
		name := content[m[2]:m[3]]
		envVar := content[m[4]:m[5]]
		var fallback string
		switch {
		case m[6] >= 0:
			fallback = content[m[6]:m[7]]
		case m[8] >= 0:
			fallback = content[m[8]:m[9]]
		default:
			// No default argument inside getenv(); fall through to the
			// pyGetenvOrRe matcher which may attach an `or "default"`
			// fallback.
			continue
		}
		out = append(out, Binding{
			Ident:      name,
			Line:       lineOfOffset(content, m[2]),
			Value:      fallback,
			EnvVar:     envVar,
			Provenance: ProvenanceEnvFallback,
			Confidence: 0.85,
		})
		seen[m[2]] = true
	}

	for _, m := range pyGetenvOrRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 {
			continue
		}
		if seen[m[2]] {
			continue
		}
		name := content[m[2]:m[3]]
		envVar := content[m[4]:m[5]]
		var fallback string
		switch {
		case m[6] >= 0:
			fallback = content[m[6]:m[7]]
		case m[8] >= 0:
			fallback = content[m[8]:m[9]]
		default:
			continue
		}
		out = append(out, Binding{
			Ident:      name,
			Line:       lineOfOffset(content, m[2]),
			Value:      fallback,
			EnvVar:     envVar,
			Provenance: ProvenanceEnvFallback,
			Confidence: 0.85,
		})
		seen[m[2]] = true
	}

	for _, m := range pyLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 {
			continue
		}
		if seen[m[2]] {
			continue
		}
		name := content[m[2]:m[3]]
		var value string
		switch {
		case m[4] >= 0:
			value = content[m[4]:m[5]]
		case m[6] >= 0:
			value = content[m[6]:m[7]]
		default:
			continue
		}
		out = append(out, Binding{
			Ident:      name,
			Line:       lineOfOffset(content, m[2]),
			Value:      value,
			Provenance: ProvenanceLiteral,
			Confidence: 1.0,
		})
	}

	for _, m := range pyImportFromRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		module := content[m[2]:m[3]]
		specs := content[m[4]:m[5]]
		line := lineOfOffset(content, m[2])
		// Strip parenthesised wrapping `from x import (a, b)`.
		specs = strings.TrimSpace(specs)
		specs = strings.TrimPrefix(specs, "(")
		specs = strings.TrimSuffix(specs, ")")
		for _, spec := range strings.Split(specs, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" || spec == "*" {
				continue
			}
			local := spec
			if asIdx := strings.Index(spec, " as "); asIdx > 0 {
				local = strings.TrimSpace(spec[asIdx+4:])
			}
			if !isPyIdent(local) {
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
	}

	return out
}

func isPyIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
				return false
			}
			continue
		}
		if !(r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}
	return true
}
