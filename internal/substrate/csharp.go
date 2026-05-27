// C# constant-binding sniffer (#2762 Phase 0 T2).
//
// Recognises:
//   - `[public|private|protected|internal] [static] const string X = "literal";`
//   - `[public|private|protected|internal] static readonly string X = "literal";`
//   - `... = Environment.GetEnvironmentVariable("Y") ?? "default";`
//   - `using System.Foo;` / `using Bar = System.Foo;` (cross-file binding)
//
// Intentionally narrow per #2762: string-typed fields only. Properties with
// getters, expression-bodied members, and non-string consts fall outside
// Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("csharp", sniffCSharp) }

// csLiteralRe matches `[mods] [static] (const|readonly) string NAME = "value";`.
// The modifier set is captured loosely so visibility does not gate detection.
//
// Capture groups: 1=name, 2=value.
var csLiteralRe = regexp.MustCompile(
	`(?m)(?:^|\n)\s*(?:public\s+|private\s+|protected\s+|internal\s+|)?` +
		`(?:static\s+)?(?:const|readonly)\s+string\s+([A-Za-z_][\w]*)\s*` +
		`=\s*"([^"\n\r]{0,512})"\s*;`,
)

// csEnvNullCoalesceRe matches the
// `Environment.GetEnvironmentVariable("Y") ?? "default"` pattern as the RHS
// of a field-style declaration. Both `string` and `var` LHS forms are
// accepted (var is a top-level statement in newer C#).
//
// Capture groups: 1=name, 2=env-var, 3=default literal.
var csEnvNullCoalesceRe = regexp.MustCompile(
	`(?m)(?:^|\n)\s*(?:public\s+|private\s+|protected\s+|internal\s+|)?` +
		`(?:static\s+)?(?:readonly\s+)?(?:string|var)\s+([A-Za-z_][\w]*)\s*` +
		`=\s*Environment\.GetEnvironmentVariable\s*\(\s*"([^"]{1,128})"\s*\)\s*\?\?\s*` +
		`"([^"\n\r]{0,512})"\s*;`,
)

// csUsingRe matches `using [Alias = ]Qualified.Name;`. The local binding is
// the alias when present, else the trailing dotted segment.
//
// Capture groups: 1=alias (optional), 2=qualified name.
var csUsingRe = regexp.MustCompile(
	`(?m)^\s*using\s+(?:static\s+)?(?:([A-Za-z_][\w]*)\s*=\s*)?([\w.]+)\s*;`,
)

// sniffCSharp implements the C# Binding sniffer.
func sniffCSharp(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range csEnvNullCoalesceRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range csLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range csUsingRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		var alias, qualified string
		if m[2] >= 0 {
			alias = content[m[2]:m[3]]
		}
		qualified = content[m[4]:m[5]]
		local := alias
		module := qualified
		if local == "" {
			if dot := strings.LastIndex(qualified, "."); dot >= 0 {
				local = qualified[dot+1:]
				module = qualified[:dot]
			} else {
				local = qualified
			}
		}
		if local == "" {
			continue
		}
		out = append(out, Binding{
			Ident:        local,
			Line:         lineOfOffset(content, m[4]),
			Provenance:   ProvenanceCrossFile,
			Confidence:   0.6,
			ImportSource: module,
		})
	}

	return out
}
