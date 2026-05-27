// Swift constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `let X = "literal"` / `let X: String = "literal"`
//   - `static let X = "literal"` (top-level inside `enum`/`struct` namespaces)
//   - `let X = ProcessInfo.processInfo.environment["Y"] ?? "default"`
//   - `import Foundation` / `import struct Foundation.Date`
//
// `var` declarations are excluded — convention for constants in Swift is
// `let`. The struct/enum-scoped `static let` form is common and included.
package substrate

import "regexp"

func init() { Register("swift", sniffSwift) }

// swiftLiteralRe matches `[public|internal|private] [static] let NAME [: T] = "value"`.
//
// Capture groups: 1=name, 2=value.
var swiftLiteralRe = regexp.MustCompile(
	`(?m)^\s*(?:(?:public|internal|private|fileprivate|open)\s+)?` +
		`(?:static\s+)?let\s+([A-Za-z_][\w]*)\s*` +
		`(?::\s*[A-Za-z_][\w<>\[\],\s.]*\s*)?` +
		`=\s*"([^"\n\r]{0,512})"`,
)

// swiftProcessEnvRe matches `ProcessInfo.processInfo.environment["Y"] ?? "default"`.
// Capture groups: 1=name, 2=env-var, 3=default.
var swiftProcessEnvRe = regexp.MustCompile(
	`(?m)^\s*(?:(?:public|internal|private|fileprivate|open)\s+)?` +
		`(?:static\s+)?let\s+([A-Za-z_][\w]*)\s*` +
		`(?::\s*[A-Za-z_][\w<>\[\],\s.]*\s*)?` +
		`=\s*ProcessInfo\.processInfo\.environment\s*\[\s*"([^"\n\r]{1,128})"\s*\]\s*` +
		`\?\?\s*"([^"\n\r]{0,512})"`,
)

// swiftImportRe matches `import [kind] ModuleName[.Submember]`.
// Capture group 1 is the module path; the local binding name is the last
// dotted segment.
var swiftImportRe = regexp.MustCompile(
	`(?m)^\s*import\s+(?:(?:struct|class|enum|protocol|func|var|let|typealias)\s+)?` +
		`([A-Za-z_][\w.]*)`,
)

func sniffSwift(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range swiftProcessEnvRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 {
			continue
		}
		out = append(out, Binding{
			Ident:      content[m[2]:m[3]],
			Line:       lineOfOffset(content, m[2]),
			Value:      content[m[6]:m[7]],
			EnvVar:     content[m[4]:m[5]],
			Provenance: ProvenanceEnvFallback,
			Confidence: 0.85,
		})
		seen[m[2]] = true
	}
	for _, m := range swiftLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range swiftImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		qualified := content[m[2]:m[3]]
		// Local name: last dotted segment (or whole path for top-level).
		local := qualified
		for i := len(qualified) - 1; i >= 0; i-- {
			if qualified[i] == '.' {
				local = qualified[i+1:]
				break
			}
		}
		out = append(out, Binding{
			Ident:        local,
			Line:         lineOfOffset(content, m[2]),
			Provenance:   ProvenanceCrossFile,
			Confidence:   0.6,
			ImportSource: qualified,
		})
	}
	return out
}
