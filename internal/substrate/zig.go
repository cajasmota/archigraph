// Zig constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `const X = "literal";` / `pub const X = "literal";`
//   - `const X: []const u8 = "literal";`
//   - `const X = @import("module");`
//
// Zig has no runtime env-var access at compile time; runtime env reads
// require std.process.getEnvMap with allocator, which is too varied for a
// regex sniffer. env_fallback is therefore not_applicable here.
package substrate

import "regexp"

func init() { Register("zig", sniffZig) }

// zigLiteralRe matches `[pub] const NAME [: type] = "value";`.
// Capture groups: 1=name, 2=value.
var zigLiteralRe = regexp.MustCompile(
	`(?m)^\s*(?:pub\s+)?const\s+([A-Za-z_][\w]*)\s*` +
		`(?::\s*[\w\[\]\s_*]*\s*)?` +
		`=\s*"([^"\n\r]{0,512})"\s*;`,
)

// zigImportRe matches `[pub] const NAME = @import("module");`.
// Capture groups: 1=local name, 2=module path.
var zigImportRe = regexp.MustCompile(
	`(?m)^\s*(?:pub\s+)?const\s+([A-Za-z_][\w]*)\s*=\s*@import\s*\(\s*"([^"\n]+)"\s*\)\s*;`,
)

func sniffZig(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range zigImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		out = append(out, Binding{
			Ident:        content[m[2]:m[3]],
			Line:         lineOfOffset(content, m[2]),
			Provenance:   ProvenanceCrossFile,
			Confidence:   0.6,
			ImportSource: content[m[4]:m[5]],
		})
		seen[m[2]] = true
	}
	for _, m := range zigLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 || seen[m[2]] {
			continue
		}
		out = append(out, Binding{
			Ident:      content[m[2]:m[3]],
			Line:       lineOfOffset(content, m[2]),
			Value:      content[m[4]:m[5]],
			Provenance: ProvenanceLiteral,
			Confidence: 1.0,
		})
	}
	return out
}
