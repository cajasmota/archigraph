// JS/TS constant-binding sniffer (#2761 Phase 0 T1).
//
// Recognises:
//   - `const X = "literal"` / `let X = "literal"` / `var X = "literal"`
//   - `const X = process.env.Y ?? "default"`
//   - `const X = process.env.Y || "default"`
//   - `const X = import.meta.env.Y ?? "default"`
//   - `const X = import.meta.env.Y || "default"`
//   - `export const X = "literal"` (re-exportable)
//   - `import { X } from "./other"` (cross-file rebinding — Value left empty)
//
// Intentionally narrow per #2761: identifier-on-LHS, string-literal-on-RHS
// (or env-fallback with a literal fallback). Object/array values, function
// calls, and template literals fall outside Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("jsts", sniffJSTS) }

// jstsLiteralRe matches `[export ](const|let|var) X = "value"`. The export
// prefix is optional; the propagation pass treats every binding as
// potentially re-exportable when an IMPORTS edge points at it.
//
// Capture groups: 1=name, 2=double-quoted value, 3=single-quoted value,
// 4=backtick value (only when the backtick body contains no ${...}).
var jstsLiteralRe = regexp.MustCompile(
	`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*` +
		`(?::\s*[A-Za-z_$][\w$<>\[\],\s]*\s*)?` + // optional TS type annotation
		`=\s*` +
		"(?:\"([^\"\\n\\r]{0,512})\"|'([^'\\n\\r]{0,512})'|`([^`\\n\\r$]{0,512})`)" +
		`\s*[;,\n]`,
)

// jstsEnvFallbackRe matches the env-var ?? / || fallback pattern:
//
//	const X = process.env.Y ?? "default"
//	const X = process.env.Y || "default"
//	const X = import.meta.env.Y ?? "default"
//	const X = import.meta.env.Y || "default"
//	const X = process.env["Y"] ?? "default"
//	const X = import.meta.env["Y"] ?? "default"
//
// Capture groups: 1=name, 2=dotted env-var name, 3=bracketed env-var name,
// 4=double-quoted default, 5=single-quoted default.
var jstsEnvFallbackRe = regexp.MustCompile(
	`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*` +
		`(?::\s*[A-Za-z_$][\w$<>\[\],\s]*\s*)?` +
		`=\s*(?:process|import\s*\.\s*meta)\s*\.\s*env\s*` +
		`(?:\.([A-Za-z_$][\w$]*)|\["([^"]+)"\])` +
		`\s*(?:\?\?|\|\|)\s*` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')`,
)

// jstsImportRe matches `import { X } from "./mod"` and `import { X as Y }
// from "./mod"`. Used to record cross-file rebindings whose value the
// propagation pass resolves by following the IMPORTS edge.
//
// Capture group 1 is the import specifier list (may contain multiple
// comma-separated names with optional `as` rebinding). Capture group 2 is
// the module path.
var jstsImportRe = regexp.MustCompile(
	`(?m)^\s*import\s*(?:[A-Za-z_$][\w$]*\s*,\s*)?` +
		`\{([^}]+)\}\s*from\s*["']([^"'\n]+)["']`,
)

// sniffJSTS implements the JS/TS Binding sniffer.
func sniffJSTS(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	out = appendJSTSLiterals(out, content)
	out = appendJSTSEnvFallbacks(out, content)
	out = appendJSTSImports(out, content)
	return out
}

func appendJSTSLiterals(out []Binding, content string) []Binding {
	for _, m := range jstsLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 {
			continue
		}
		name := content[m[2]:m[3]]
		var value string
		switch {
		case m[4] >= 0:
			value = content[m[4]:m[5]]
		case m[6] >= 0:
			value = content[m[6]:m[7]]
		case m[8] >= 0:
			value = content[m[8]:m[9]]
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
	return out
}

func appendJSTSEnvFallbacks(out []Binding, content string) []Binding {
	for _, m := range jstsEnvFallbackRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 12 {
			continue
		}
		name := content[m[2]:m[3]]
		var envVar string
		switch {
		case m[4] >= 0:
			envVar = content[m[4]:m[5]]
		case m[6] >= 0:
			envVar = content[m[6]:m[7]]
		}
		var fallback string
		switch {
		case m[8] >= 0:
			fallback = content[m[8]:m[9]]
		case m[10] >= 0:
			fallback = content[m[10]:m[11]]
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
	}
	return out
}

func appendJSTSImports(out []Binding, content string) []Binding {
	for _, m := range jstsImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		specifiers := content[m[2]:m[3]]
		modulePath := content[m[4]:m[5]]
		line := lineOfOffset(content, m[2])
		for _, spec := range strings.Split(specifiers, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			// `X as Y` → local name is Y; remote is X.
			local := spec
			if asIdx := strings.Index(spec, " as "); asIdx > 0 {
				local = strings.TrimSpace(spec[asIdx+4:])
			}
			if !isJSIdent(local) {
				continue
			}
			out = append(out, Binding{
				Ident:        local,
				Line:         line,
				Provenance:   ProvenanceCrossFile,
				Confidence:   0.6,
				ImportSource: modulePath,
			})
		}
	}
	return out
}

// isJSIdent reports whether s is a valid JS/TS identifier.
func isJSIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !(r == '_' || r == '$' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
				return false
			}
			continue
		}
		if !(r == '_' || r == '$' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}
	return true
}

// lineOfOffset returns the 1-indexed line containing byte offset off in
// content. Linear scan — fine for typical source files.
func lineOfOffset(content string, off int) int {
	if off <= 0 {
		return 1
	}
	if off > len(content) {
		off = len(content)
	}
	line := 1
	for i := 0; i < off; i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return line
}
