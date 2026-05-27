// PHP constant-binding sniffer (#2762 Phase 0 T2).
//
// Recognises top-level binding shapes:
//   - `const X = "literal";`  (file-level or namespaced const declaration)
//   - `define("X", "literal");`
//   - `$x = "literal";`  (module-level variable assignment)
//   - `$x = getenv("Y") ?: "default";`
//   - `$x = getenv("Y") ?? "default";`
//   - `use Foo\Bar;` / `use Foo\Bar as Baz;`  (cross-file binding)
//
// Intentionally narrow per #2762: top-level only. Class constants and
// trait-scoped assignments fall outside Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("php", sniffPHP) }

// phpConstRe matches `const NAME = "value";` at the start of a line. The
// `^` anchor keeps class-body constants out.
//
// Capture groups: 1=name, 2=double-quoted value, 3=single-quoted value.
var phpConstRe = regexp.MustCompile(
	`(?m)^const\s+([A-Za-z_][\w]*)\s*=\s*` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*;`,
)

// phpDefineRe matches `define("X", "value");` and the single-quoted twin.
//
// Capture groups: 1=double-quoted name, 2=single-quoted name,
// 3=double-quoted value, 4=single-quoted value.
var phpDefineRe = regexp.MustCompile(
	`(?m)^\s*define\s*\(\s*(?:"([A-Za-z_][\w]*)"|'([A-Za-z_][\w]*)')\s*,\s*` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*\)\s*;`,
)

// phpVarLiteralRe matches `$name = "value";` at top level.
//
// Capture groups: 1=name (without $), 2=double-quoted value,
// 3=single-quoted value.
var phpVarLiteralRe = regexp.MustCompile(
	`(?m)^\$([A-Za-z_][\w]*)\s*=\s*` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*;`,
)

// phpEnvFallbackRe matches `$name = getenv("Y") ?: "default";` and the
// `?? "default"` null-coalesce variant. Also matches the const form
// `const NAME = getenv("Y") ?: "default";`.
//
// Capture groups: 1=name (variable form), 2=name (const form),
// 3=env-var, 4=double-quoted default, 5=single-quoted default.
var phpEnvFallbackRe = regexp.MustCompile(
	`(?m)^(?:\$([A-Za-z_][\w]*)|const\s+([A-Za-z_][\w]*))\s*=\s*` +
		`getenv\s*\(\s*["']([^"']{1,128})["']\s*\)\s*(?:\?:|\?\?)\s*` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')`,
)

// phpUseRe matches `use Foo\Bar;` and `use Foo\Bar as Baz;`.
//
// Capture groups: 1=fully qualified path, 2=optional alias.
var phpUseRe = regexp.MustCompile(
	`(?m)^\s*use\s+([\w\\]+)(?:\s+as\s+([A-Za-z_][\w]*))?\s*;`,
)

// sniffPHP implements the PHP Binding sniffer.
func sniffPHP(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range phpEnvFallbackRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 12 {
			continue
		}
		var name string
		switch {
		case m[2] >= 0:
			name = content[m[2]:m[3]]
		case m[4] >= 0:
			name = content[m[4]:m[5]]
		default:
			continue
		}
		envVar := content[m[6]:m[7]]
		var def string
		switch {
		case m[8] >= 0:
			def = content[m[8]:m[9]]
		case m[10] >= 0:
			def = content[m[10]:m[11]]
		default:
			continue
		}
		out = append(out, Binding{
			Ident:      name,
			Line:       lineOfOffset(content, m[0]),
			Value:      def,
			EnvVar:     envVar,
			Provenance: ProvenanceEnvFallback,
			Confidence: 0.85,
		})
		seen[m[0]] = true
	}

	for _, m := range phpConstRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 || seen[m[0]] {
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

	for _, m := range phpDefineRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 {
			continue
		}
		var name string
		switch {
		case m[2] >= 0:
			name = content[m[2]:m[3]]
		case m[4] >= 0:
			name = content[m[4]:m[5]]
		default:
			continue
		}
		var value string
		switch {
		case m[6] >= 0:
			value = content[m[6]:m[7]]
		case m[8] >= 0:
			value = content[m[8]:m[9]]
		default:
			continue
		}
		out = append(out, Binding{
			Ident:      name,
			Line:       lineOfOffset(content, m[0]),
			Value:      value,
			Provenance: ProvenanceLiteral,
			Confidence: 1.0,
		})
	}

	for _, m := range phpVarLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 || seen[m[0]] {
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

	for _, m := range phpUseRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		qualified := content[m[2]:m[3]]
		var local string
		if m[4] >= 0 {
			local = content[m[4]:m[5]]
		} else {
			if bs := strings.LastIndex(qualified, "\\"); bs >= 0 {
				local = qualified[bs+1:]
			} else {
				local = qualified
			}
		}
		module := qualified
		if bs := strings.LastIndex(qualified, "\\"); bs >= 0 {
			module = qualified[:bs]
		}
		if local == "" {
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
