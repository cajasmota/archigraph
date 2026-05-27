// Ruby constant-binding sniffer (#2762 Phase 0 T2).
//
// Recognises top-level binding shapes:
//   - `X = "literal"` / `X = 'literal'` (Ruby constants are conventionally
//     SCREAMING_CASE but the grammar accepts any leading-uppercase name)
//   - `X = ENV.fetch("Y", "default")`
//   - `X = ENV.fetch("Y") { "default" }` (block form — narrow regex)
//   - `X = ENV["Y"] || "default"`
//   - `require "lib/foo"` / `require_relative "./bar"` (cross-file binding)
//
// Intentionally narrow per #2762: module-level only (no leading whitespace
// before the assignment). Per-class constants and method-body assignments
// fall outside Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("ruby", sniffRuby) }

// rubyLiteralRe matches `NAME = "value"` / `NAME = 'value'` at top level.
// Ruby identifiers begin with a letter or underscore; constants are
// uppercase-leading by convention but Ruby allows any name to bind a string.
//
// Capture groups: 1=name, 2=double-quoted value, 3=single-quoted value.
var rubyLiteralRe = regexp.MustCompile(
	`(?m)^([A-Z][A-Za-z_0-9]*|[a-z_][A-Za-z_0-9]*)\s*=\s*` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*(?:#.*)?$`,
)

// rubyEnvFetchRe matches `NAME = ENV.fetch("Y", "default")` and the
// `ENV.fetch("Y") { "default" }` block-fallback form.
//
// Capture groups: 1=name, 2=env-var, 3=double-quoted positional default,
// 4=single-quoted positional default, 5=double-quoted block default,
// 6=single-quoted block default.
var rubyEnvFetchRe = regexp.MustCompile(
	`(?m)^([A-Z][A-Za-z_0-9]*|[a-z_][A-Za-z_0-9]*)\s*=\s*` +
		`ENV\.fetch\s*\(\s*["']([^"']{1,128})["']` +
		`(?:\s*,\s*(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})'))?` +
		`\s*\)\s*` +
		`(?:\{\s*(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*\})?`,
)

// rubyEnvBracketOrRe matches `NAME = ENV["Y"] || "default"`.
//
// Capture groups: 1=name, 2=env-var, 3=double-quoted default,
// 4=single-quoted default.
var rubyEnvBracketOrRe = regexp.MustCompile(
	`(?m)^([A-Z][A-Za-z_0-9]*|[a-z_][A-Za-z_0-9]*)\s*=\s*` +
		`ENV\s*\[\s*["']([^"']{1,128})["']\s*\]\s*\|\|\s*` +
		`(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')`,
)

// rubyRequireRe matches `require "lib/foo"` and `require_relative "./bar"`.
// The local binding name is the basename of the module path (last segment).
var rubyRequireRe = regexp.MustCompile(
	`(?m)^\s*(?:require|require_relative)\s+["']([^"'\n]+)["']`,
)

// sniffRuby implements the Ruby Binding sniffer.
func sniffRuby(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range rubyEnvFetchRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 14 {
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
		case m[10] >= 0:
			def = content[m[10]:m[11]]
		case m[12] >= 0:
			def = content[m[12]:m[13]]
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

	for _, m := range rubyEnvBracketOrRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 {
			continue
		}
		if seen[m[2]] {
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

	for _, m := range rubyLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range rubyRequireRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		module := content[m[2]:m[3]]
		local := module
		if slash := strings.LastIndex(local, "/"); slash >= 0 {
			local = local[slash+1:]
		}
		local = strings.TrimSuffix(local, ".rb")
		if local == "" {
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
