// C/C++ constant-binding sniffer (#2762 Phase 0 T2).
//
// Recognises:
//   - `#define NAME "literal"` (preprocessor macro)
//   - `const char* NAME = "literal";` / `const char NAME[] = "literal";`
//   - `static const char* NAME = "literal";`
//   - `constexpr const char* NAME = "literal";` / `constexpr auto NAME = "literal";`
//   - `const std::string NAME = "literal";` (C++)
//   - `const char* NAME = getenv("Y") ? getenv("Y") : "default";`
//   - `#include "header.h"` / `#include <header>` (cross-file binding)
//
// Intentionally narrow per #2762: top-level only. Class members, namespaced
// constants, and template-parameter constants fall outside Phase 0.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("c-cpp", sniffCCPP) }

// cppDefineRe matches `#define NAME "value"` (single-line, string-valued).
//
// Capture groups: 1=name, 2=value.
var cppDefineRe = regexp.MustCompile(
	`(?m)^\s*#\s*define\s+([A-Za-z_][\w]*)\s+"([^"\n\r]{0,512})"\s*$`,
)

// cppLiteralRe matches `[static|constexpr|extern]* [const]? (char\s*\*|char\s+NAME\[\]|std::string|auto)
// NAME = "value";`. The type-position is intentionally permissive — Phase 0
// only needs to bind a name to a string literal, not preserve type info.
//
// Capture groups: 1=name (pointer form), 2=value (pointer form),
// 3=name (array form), 4=value (array form).
var cppLiteralRe = regexp.MustCompile(
	`(?m)(?:^|\n)\s*(?:static\s+|extern\s+|constexpr\s+|)*` +
		`(?:const\s+)?(?:char\s*\*|std::string|auto)\s+([A-Za-z_][\w]*)\s*` +
		`=\s*"([^"\n\r]{0,512})"\s*;` +
		`|(?:^|\n)\s*(?:static\s+|extern\s+|constexpr\s+|)*` +
		`(?:const\s+)?char\s+([A-Za-z_][\w]*)\s*\[\s*\]\s*=\s*"([^"\n\r]{0,512})"\s*;`,
)

// cppEnvTernaryRe matches `... NAME = getenv("Y") ? getenv("Y") : "default";`.
// The type-position is permissive for the same reason as cppLiteralRe.
//
// Capture groups: 1=name, 2=env-var, 3=default literal.
var cppEnvTernaryRe = regexp.MustCompile(
	`(?m)(?:^|\n)\s*(?:static\s+|extern\s+|constexpr\s+|)*` +
		`(?:const\s+)?(?:char\s*\*|std::string|auto)\s+([A-Za-z_][\w]*)\s*` +
		`=\s*getenv\s*\(\s*"([^"]{1,128})"\s*\)\s*\?\s*[^:]+:\s*` +
		`"([^"\n\r]{0,512})"\s*;`,
)

// cppIncludeRe matches `#include "header.h"` and `#include <header>`. The
// local binding name is the basename without extension.
//
// Capture groups: 1=quoted-form path, 2=bracket-form path.
var cppIncludeRe = regexp.MustCompile(
	`(?m)^\s*#\s*include\s+(?:"([^"\n]+)"|<([^>\n]+)>)`,
)

// sniffCCPP implements the C/C++ Binding sniffer.
func sniffCCPP(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range cppEnvTernaryRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range cppDefineRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
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

	for _, m := range cppLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 10 {
			continue
		}
		var name, value string
		var nameStart int
		switch {
		case m[2] >= 0:
			name = content[m[2]:m[3]]
			value = content[m[4]:m[5]]
			nameStart = m[2]
		case m[6] >= 0:
			name = content[m[6]:m[7]]
			value = content[m[8]:m[9]]
			nameStart = m[6]
		default:
			continue
		}
		if seen[nameStart] {
			continue
		}
		out = append(out, Binding{
			Ident:      name,
			Line:       lineOfOffset(content, nameStart),
			Value:      value,
			Provenance: ProvenanceLiteral,
			Confidence: 1.0,
		})
	}

	for _, m := range cppIncludeRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		var path string
		switch {
		case m[2] >= 0:
			path = content[m[2]:m[3]]
		case m[4] >= 0:
			path = content[m[4]:m[5]]
		default:
			continue
		}
		local := path
		if slash := strings.LastIndexAny(local, "/\\"); slash >= 0 {
			local = local[slash+1:]
		}
		// Strip a trailing .h / .hpp / .hh / .hxx for the local binding name.
		for _, ext := range []string{".hpp", ".hxx", ".hh", ".h"} {
			if strings.HasSuffix(local, ext) {
				local = local[:len(local)-len(ext)]
				break
			}
		}
		if local == "" {
			continue
		}
		out = append(out, Binding{
			Ident:        local,
			Line:         lineOfOffset(content, m[0]),
			Provenance:   ProvenanceCrossFile,
			Confidence:   0.6,
			ImportSource: path,
		})
	}

	return out
}
