// Go constant-binding sniffer (#2761 Phase 0 T1).
//
// Recognises top-level binding shapes:
//   - `const X = "literal"`
//   - `var X = "literal"`
//   - `const X string = "literal"` / `var X string = "literal"`
//   - `X := "literal"` at package level (rare but legal in init blocks → skipped)
//   - `var X = os.Getenv("Y")` (env-var, no fallback → value left empty)
//   - `var X = cmp.Or(os.Getenv("Y"), "default")` / `if v := os.Getenv("Y"); v != "" { ... } else { ... }` (env-fallback)
//   - `import "pkg/path"` and `import alias "pkg/path"` (cross-file binding)
//
// Intentionally narrow per #2761: top-level only, string-typed only. The
// `:=` short-declaration form is excluded because it only occurs inside
// function bodies.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("go", sniffGo) }

// goLiteralRe matches a top-level `const|var NAME [string] = "value"`. The
// `^` anchor + leading-space exclusion keeps function-body bindings out.
//
// Capture groups: 1=name, 2=value.
var goLiteralRe = regexp.MustCompile(
	`(?m)^(?:const|var)\s+([A-Za-z_][\w]*)\s+(?:string\s+)?=\s*` +
		`"([^"\n\r]{0,512})"`,
)

// goEnvFallbackCmpOrRe matches `var X = cmp.Or(os.Getenv("Y"), "default")`,
// a common Go 1.22+ shape for env-with-default. Also matches
// strings.TrimSpace(os.Getenv("Y")) where Or is the outermost.
//
// Capture groups: 1=name, 2=env-var, 3=default literal.
var goEnvFallbackCmpOrRe = regexp.MustCompile(
	`(?m)^(?:const|var)\s+([A-Za-z_][\w]*)\s+(?:string\s+)?=\s*` +
		`cmp\.Or\s*\(\s*os\.Getenv\s*\(\s*"([^"]{1,128})"\s*\)\s*,\s*` +
		`"([^"\n\r]{0,512})"\s*\)`,
)

// goImportLineRe matches a single import spec line inside an import block
// or `import "pkg"` / `import alias "pkg"`. We capture both the alias and
// the module path so the propagation pass can map identifier → module.
//
// Capture groups: 1=optional alias, 2=module path.
var goImportLineRe = regexp.MustCompile(
	`(?m)^\s*(?:([A-Za-z_][\w]*)\s+)?"([^"\n]+)"`,
)

// goImportBlockRe matches the block-form `import ( ... )`.
var goImportBlockRe = regexp.MustCompile(
	`(?s)import\s*\(\s*(.*?)\s*\)`,
)

// goImportSingleRe matches the single-line `import "pkg"` form (not block).
var goImportSingleRe = regexp.MustCompile(
	`(?m)^import\s+(?:([A-Za-z_][\w]*)\s+)?"([^"\n]+)"`,
)

// sniffGo implements the Go Binding sniffer.
func sniffGo(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range goEnvFallbackCmpOrRe.FindAllStringSubmatchIndex(content, -1) {
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

	for _, m := range goLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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

	// Imports: scan block-form first, then single-line.
	for _, m := range goImportBlockRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		block := content[m[2]:m[3]]
		blockStart := m[2]
		for _, im := range goImportLineRe.FindAllStringSubmatchIndex(block, -1) {
			if len(im) < 6 {
				continue
			}
			var alias string
			if im[2] >= 0 {
				alias = block[im[2]:im[3]]
			}
			module := block[im[4]:im[5]]
			local := alias
			if local == "" {
				// Default local name: last path segment.
				if slash := strings.LastIndex(module, "/"); slash >= 0 {
					local = module[slash+1:]
				} else {
					local = module
				}
			}
			if local == "_" || local == "." {
				continue
			}
			out = append(out, Binding{
				Ident:        local,
				Line:         lineOfOffset(content, blockStart+im[0]),
				Provenance:   ProvenanceCrossFile,
				Confidence:   0.6,
				ImportSource: module,
			})
		}
	}

	for _, m := range goImportSingleRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		var alias string
		if m[2] >= 0 {
			alias = content[m[2]:m[3]]
		}
		module := content[m[4]:m[5]]
		local := alias
		if local == "" {
			if slash := strings.LastIndex(module, "/"); slash >= 0 {
				local = module[slash+1:]
			} else {
				local = module
			}
		}
		if local == "_" || local == "." {
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
