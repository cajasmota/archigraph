// Elixir constant-binding sniffer (#2762 Phase 0 T2).
//
// Recognises module-level binding shapes:
//   - `@name "literal"`  (module attribute, the Elixir constant idiom)
//   - `@name 'literal'`
//   - `@name System.get_env("Y", "default")`
//   - `@name System.get_env("Y") || "default"`
//   - `alias Foo.Bar` / `alias Foo.{Bar, Baz}` (cross-file binding)
//
// Intentionally narrow per #2762: module attribute form only. Function-body
// pattern bindings and `defp`/`def` parameters fall outside Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("elixir", sniffElixir) }

// exAttrLiteralRe matches a `@name "value"` module attribute. Indentation
// is permitted because Elixir always nests attributes inside `defmodule`.
//
// Capture groups: 1=name, 2=double-quoted value, 3=single-quoted value.
var exAttrLiteralRe = regexp.MustCompile(
	`(?m)^\s*@([a-z_][\w]*)\s+` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*(?:#.*)?$`,
)

// exAttrEnvRe matches `@name System.get_env("Y", "default")`.
//
// Capture groups: 1=name, 2=env-var, 3=double-quoted default,
// 4=single-quoted default.
var exAttrEnvRe = regexp.MustCompile(
	`(?m)^\s*@([a-z_][\w]*)\s+System\.get_env\s*\(\s*["']([^"']{1,128})["']` +
		`(?:\s*,\s*(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})'))?` +
		`\s*\)`,
)

// exAttrEnvOrRe matches `@name System.get_env("Y") || "default"`.
//
// Capture groups: 1=name, 2=env-var, 3=double-quoted default,
// 4=single-quoted default.
var exAttrEnvOrRe = regexp.MustCompile(
	`(?m)^\s*@([a-z_][\w]*)\s+System\.get_env\s*\(\s*["']([^"']{1,128})["']\s*\)\s*\|\|\s*` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')`,
)

// exAliasRe matches `alias Foo.Bar`, `alias Foo.{Bar, Baz}`, and
// `alias Foo.Bar, as: Quux`.
//
// Capture groups: 1=base path, 2=braced multi-spec (optional),
// 3=alias rebinding (optional).
var exAliasRe = regexp.MustCompile(
	`(?m)^\s*alias\s+([\w.]+?)(?:\.\{([^}]+)\}|,\s*as:\s*([A-Z][\w]*))?\s*$`,
)

// sniffElixir implements the Elixir Binding sniffer.
func sniffElixir(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range exAttrEnvRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 {
			continue
		}
		name := content[m[2]:m[3]]
		envVar := content[m[4]:m[5]]
		var def string
		switch {
		case m[6] >= 0:
			def = content[m[6]:m[7]]
		case m[8] >= 0:
			def = content[m[8]:m[9]]
		default:
			continue
		}
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

	for _, m := range exAttrEnvOrRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 || seen[m[2]] {
			continue
		}
		name := content[m[2]:m[3]]
		envVar := content[m[4]:m[5]]
		var def string
		switch {
		case m[6] >= 0:
			def = content[m[6]:m[7]]
		case m[8] >= 0:
			def = content[m[8]:m[9]]
		default:
			continue
		}
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

	for _, m := range exAttrLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 || seen[m[2]] {
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

	for _, m := range exAliasRe.FindAllStringSubmatchIndex(content, -1) {
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
				if spec == "" {
					continue
				}
				local := spec
				if !isElixirModuleSegment(local) {
					continue
				}
				out = append(out, Binding{
					Ident:        local,
					Line:         line,
					Provenance:   ProvenanceCrossFile,
					Confidence:   0.6,
					ImportSource: base + "." + spec,
				})
			}
			continue
		}
		local := alias
		module := base
		if local == "" {
			if dot := strings.LastIndex(base, "."); dot >= 0 {
				local = base[dot+1:]
				module = base[:dot]
			} else {
				local = base
			}
		}
		if !isElixirModuleSegment(local) {
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

// isElixirModuleSegment reports whether s is a valid trailing module
// segment (CamelCase identifier).
func isElixirModuleSegment(s string) bool {
	if s == "" {
		return false
	}
	if !(s[0] >= 'A' && s[0] <= 'Z') {
		return false
	}
	for _, r := range s[1:] {
		if !(r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}
	return true
}
