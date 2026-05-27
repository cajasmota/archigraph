// Java constant-binding sniffer (#2761 Phase 0 T1).
//
// Recognises:
//   - `[public|private|protected] [static] final String X = "literal";`
//   - `[public|private|protected] [static] final String X = System.getenv("Y") != null ? System.getenv("Y") : "default";`
//   - `[public|private|protected] [static] final String X = Optional.ofNullable(System.getenv("Y")).orElse("default");`
//   - `import com.example.X;` and `import static com.example.X;`
//
// Intentionally narrow per #2761: String-typed constants only. Other JVM
// types (long/int/Pattern) fall outside Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("java", sniffJava) }

// javaLiteralRe matches a `[mods] [static] final String NAME = "value";` field
// declaration. The modifier set is captured loosely so visibility doesn't
// gate detection.
//
// Capture groups: 1=name, 2=value.
var javaLiteralRe = regexp.MustCompile(
	`(?m)(?:^|\n)\s*(?:public\s+|private\s+|protected\s+|)?` +
		`(?:static\s+)?final\s+String\s+([A-Za-z_$][\w$]*)\s*` +
		`=\s*"([^"\n\r]{0,512})"\s*;`,
)

// javaEnvTernaryRe matches the System.getenv("Y") != null ? ... : "default"
// pattern (and its inverted-operand twin). Conservative: requires the
// fallback literal at the tail.
//
// Capture groups: 1=name, 2=env-var, 3=default literal.
var javaEnvTernaryRe = regexp.MustCompile(
	`(?m)(?:^|\n)\s*(?:public\s+|private\s+|protected\s+|)?` +
		`(?:static\s+)?final\s+String\s+([A-Za-z_$][\w$]*)\s*` +
		`=\s*System\.getenv\s*\(\s*"([^"]{1,128})"\s*\)` +
		`\s*!=\s*null\s*\?\s*[^:]+:\s*"([^"\n\r]{0,512})"`,
)

// javaEnvOptionalRe matches Optional.ofNullable(System.getenv("Y")).orElse("default").
//
// Capture groups: 1=name, 2=env-var, 3=default literal.
var javaEnvOptionalRe = regexp.MustCompile(
	`(?m)(?:^|\n)\s*(?:public\s+|private\s+|protected\s+|)?` +
		`(?:static\s+)?final\s+String\s+([A-Za-z_$][\w$]*)\s*` +
		`=\s*Optional\.ofNullable\s*\(\s*System\.getenv\s*\(\s*"([^"]{1,128})"\s*\)\s*\)` +
		`\.orElse\s*\(\s*"([^"\n\r]{0,512})"\s*\)`,
)

// javaImportRe matches `import [static] package.qualified.Name;`. The local
// binding name is the last dotted segment.
var javaImportRe = regexp.MustCompile(
	`(?m)^\s*import\s+(?:static\s+)?([\w.]+);`,
)

// sniffJava implements the Java Binding sniffer.
func sniffJava(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range javaEnvTernaryRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range javaEnvOptionalRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 {
			continue
		}
		if seen[m[2]] {
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

	for _, m := range javaLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range javaImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		qualified := content[m[2]:m[3]]
		dot := strings.LastIndex(qualified, ".")
		if dot < 0 || dot == len(qualified)-1 {
			continue
		}
		local := qualified[dot+1:]
		module := qualified[:dot]
		if local == "*" {
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
