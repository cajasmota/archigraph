// Rust constant-binding sniffer (#2762 Phase 0 T2).
//
// Recognises top-level binding shapes:
//   - `const X: &str = "literal";`
//   - `static X: &str = "literal";`
//   - `pub const X: &str = "literal";`
//   - `const X: &'static str = "literal";`
//   - `static X: &str = env::var("Y").unwrap_or("default".into());`
//   - `static X: &str = env::var("Y").unwrap_or_else(|_| "default".into());`
//   - `use crate::foo::Bar;` / `use foo::Bar as Baz;`  (cross-file binding)
//
// Intentionally narrow per #2762: top-level `&str` only. `String`-typed
// constants (which require `.to_string()` at use-sites) and macro-defined
// constants fall outside Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("rust", sniffRust) }

// rustLiteralRe matches `[pub ](const|static) NAME: &['static ]?str = "value";`.
//
// Capture groups: 1=name, 2=value.
var rustLiteralRe = regexp.MustCompile(
	`(?m)^\s*(?:pub(?:\s*\([^)]*\))?\s+)?(?:const|static)\s+([A-Za-z_][\w]*)\s*` +
		`:\s*&(?:'static\s+)?str\s*=\s*` +
		`"([^"\n\r]{0,512})"\s*;`,
)

// rustEnvUnwrapOrRe matches the `env::var("Y").unwrap_or("default".into())`
// and `.unwrap_or_else(|_| "default".into())` patterns. The `.into()` /
// `.to_string()` / `.to_owned()` suffix is optional.
//
// Capture groups: 1=name, 2=env-var, 3=default literal.
var rustEnvUnwrapOrRe = regexp.MustCompile(
	`(?m)^\s*(?:pub(?:\s*\([^)]*\))?\s+)?(?:const|static|let)\s+([A-Za-z_][\w]*)\s*` +
		`(?::\s*[^=]+)?=\s*(?:std::)?env::var\s*\(\s*"([^"]{1,128})"\s*\)\s*` +
		`\.unwrap_or(?:_else)?\s*\(\s*(?:\|[^|]*\|\s*)?` +
		`"([^"\n\r]{0,512})"(?:\s*\.(?:into|to_string|to_owned)\s*\(\s*\))?\s*\)`,
)

// rustUseRe matches `use path::to::Name;` and `use path::Name as Alias;`.
// Multi-import `use path::{a, b}` is parsed by splitting the body.
//
// Capture groups: 1=path (without trailing item), 2=alias-form rebinding
// (optional), 3=braced multi-item list (optional).
var rustUseRe = regexp.MustCompile(
	`(?m)^\s*(?:pub\s+)?use\s+([\w:]+?)(?:::\{([^}]+)\}|\s+as\s+([A-Za-z_][\w]*))?\s*;`,
)

// sniffRust implements the Rust Binding sniffer.
func sniffRust(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range rustEnvUnwrapOrRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range rustLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range rustUseRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 {
			continue
		}
		base := content[m[2]:m[3]]
		braced := ""
		alias := ""
		if m[4] >= 0 {
			braced = content[m[4]:m[5]]
		}
		if m[6] >= 0 {
			alias = content[m[6]:m[7]]
		}
		line := lineOfOffset(content, m[2])
		if braced != "" {
			for _, spec := range strings.Split(braced, ",") {
				spec = strings.TrimSpace(spec)
				if spec == "" || spec == "self" || spec == "*" {
					continue
				}
				local := spec
				remote := spec
				if asIdx := strings.Index(spec, " as "); asIdx > 0 {
					remote = strings.TrimSpace(spec[:asIdx])
					local = strings.TrimSpace(spec[asIdx+4:])
				}
				if !isRustIdent(local) {
					continue
				}
				out = append(out, Binding{
					Ident:        local,
					Line:         line,
					Provenance:   ProvenanceCrossFile,
					Confidence:   0.6,
					ImportSource: base + "::" + remote,
				})
			}
			continue
		}
		local := alias
		module := base
		if local == "" {
			// Path "a::b::c" → local=c, module=a::b
			if idx := strings.LastIndex(base, "::"); idx >= 0 {
				local = base[idx+2:]
				module = base[:idx]
			} else {
				local = base
			}
		}
		if local == "" || local == "*" {
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

// isRustIdent reports whether s is a valid Rust identifier.
func isRustIdent(s string) bool {
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
