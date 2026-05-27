// Lua constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises module-level binding shapes:
//   - `X = "literal"` / `local X = "literal"`
//   - `X = os.getenv("Y") or "default"`
//   - `local X = require("mod")` / `local X = require "mod"`
//
// Lua has no `const` keyword (pre-5.4 `<const>` attribute is rare). The
// `local` and bare-assignment forms are both treated as bindings.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("lua", sniffLua) }

// luaLiteralRe matches a top-level `[local] NAME = "value"`. The `^` anchor
// excludes function-body bindings.
//
// Capture groups: 1=name, 2=double-quoted value, 3=single-quoted value.
var luaLiteralRe = regexp.MustCompile(
	`(?m)^(?:local\s+)?([A-Za-z_][\w]*)\s*=\s*` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*$`,
)

// luaEnvOrRe matches `[local] X = os.getenv("Y") or "default"`.
// Capture groups: 1=name, 2=env-var, 3=double-quoted default, 4=single-quoted default.
var luaEnvOrRe = regexp.MustCompile(
	`(?m)^(?:local\s+)?([A-Za-z_][\w]*)\s*=\s*` +
		`os\.getenv\s*\(\s*["']([^"']{1,128})["']\s*\)\s*or\s+` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')`,
)

// luaRequireRe matches `local X = require("mod")` / `local X = require "mod"`.
// Capture groups: 1=local name, 2=module path (double or single quoted).
var luaRequireRe = regexp.MustCompile(
	`(?m)^\s*local\s+([A-Za-z_][\w]*)\s*=\s*require\s*\(?\s*` +
		`["']([^"'\n]+)["']\s*\)?`,
)

func sniffLua(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range luaEnvOrRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range luaLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range luaRequireRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		local := content[m[2]:m[3]]
		module := content[m[4]:m[5]]
		// Lua module paths use dots; sanity-check the local binding name.
		if !isLuaIdent(local) || strings.ContainsAny(local, "\"'") {
			continue
		}
		out = append(out, Binding{
			Ident:        local,
			Line:         lineOfOffset(content, m[0]),
			Provenance:   ProvenanceCrossFile,
			Confidence:   0.6,
			ImportSource: module,
		})
	}
	return out
}

// isLuaIdent reports whether s is a valid Lua identifier.
func isLuaIdent(s string) bool {
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
