// Dart constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `const X = "literal";` / `const String X = "literal";`
//   - `final X = "literal";`  / `final String X = "literal";`
//   - `const X = String.fromEnvironment("Y", defaultValue: "default");`
//   - `const X = String.fromEnvironment("Y");` (no fallback — value left empty)
//   - `final X = Platform.environment["Y"] ?? "default";`
//   - `import 'package:foo/bar.dart';` and `import 'package:foo/bar.dart' as alias;`
//
// Intentionally narrow per #2763: top-level only, string-typed only. Class
// fields and local declarations are outside Phase 0 scope.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("dart", sniffDart) }

// dartLiteralRe matches a top-level `const|final [String] NAME = "value";`.
// Capture groups: 1=name, 2=double-quoted value, 3=single-quoted value.
var dartLiteralRe = regexp.MustCompile(
	`(?m)^(?:const|final)\s+(?:String\s+)?([A-Za-z_][\w]*)\s*` +
		`=\s*(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*;`,
)

// dartFromEnvRe matches `String.fromEnvironment("Y", defaultValue: "default")`.
// Capture groups: 1=name, 2=env-var, 3=double-quoted default, 4=single-quoted default.
var dartFromEnvRe = regexp.MustCompile(
	`(?m)^(?:const|final)\s+(?:String\s+)?([A-Za-z_][\w]*)\s*` +
		`=\s*String\.fromEnvironment\s*\(\s*["']([^"']{1,128})["']\s*,\s*` +
		`defaultValue\s*:\s*(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')\s*\)`,
)

// dartPlatformEnvRe matches `Platform.environment["Y"] ?? "default"`.
// Capture groups: 1=name, 2=env-var, 3=double-quoted default, 4=single-quoted default.
var dartPlatformEnvRe = regexp.MustCompile(
	`(?m)^(?:const|final)\s+(?:String\s+)?([A-Za-z_][\w]*)\s*` +
		`=\s*Platform\.environment\s*\[\s*["']([^"']{1,128})["']\s*\]\s*` +
		`\?\?\s*(?:"([^"\n\r]{0,512})"|'([^'\n\r]{0,512})')`,
)

// dartImportRe matches `import 'pkg/path.dart';` and `import 'pkg/path.dart' as alias;`.
// Capture groups: 1=module path, 2=optional alias.
var dartImportRe = regexp.MustCompile(
	`(?m)^\s*import\s+['"]([^'"\n]+)['"](?:\s+as\s+([A-Za-z_][\w]*))?\s*;`,
)

func sniffDart(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range dartFromEnvRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range dartPlatformEnvRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 || seen[m[2]] {
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
	for _, m := range dartLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range dartImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		module := content[m[2]:m[3]]
		var local string
		if m[4] >= 0 {
			local = content[m[4]:m[5]]
		} else {
			// Default local name: last path segment without .dart suffix.
			local = module
			if slash := strings.LastIndex(local, "/"); slash >= 0 {
				local = local[slash+1:]
			}
			local = strings.TrimSuffix(local, ".dart")
			if !isJSIdent(local) {
				continue
			}
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

// pickQuoted returns the content of whichever of the two alternation
// capture groups matched (double-quoted first, then single-quoted). The
// helper centralises the two-quote-form repetition that appears in every
// sniffer in this package.
func pickQuoted(content string, m []int, dq, sq int) string {
	switch {
	case m[dq] >= 0:
		return content[m[dq]:m[dq+1]]
	case m[sq] >= 0:
		return content[m[sq]:m[sq+1]]
	}
	return ""
}
