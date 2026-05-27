// Haskell constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `name = "literal"` (top-level, optionally preceded by a `name :: String`
//     signature on the prior line — we don't require it)
//   - `import Module[.Sub] [as Alias]`
//
// Env-var resolution in Haskell happens inside IO (`lookupEnv :: String ->
// IO (Maybe String)`), so the const+env-fallback shape is monadic and
// usually wrapped in an `unsafePerformIO`. That idiom is too varied for a
// regex sniffer; we leave env_fallback as not_applicable here and report
// honestly in the registry.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("haskell", sniffHaskell) }

// haskellLiteralRe matches a top-level `name = "value"` (lower-case ident).
//
// Capture groups: 1=name, 2=value.
var haskellLiteralRe = regexp.MustCompile(
	`(?m)^([a-z_][\w']*)\s*=\s*"([^"\n\r]{0,512})"\s*$`,
)

// haskellImportRe matches `import [qualified] Module.Path [as Alias]`.
//
// Capture groups: 1=module path, 2=optional alias.
var haskellImportRe = regexp.MustCompile(
	`(?m)^import\s+(?:qualified\s+)?([A-Z][\w.]*)(?:\s+as\s+([A-Z][\w]*))?`,
)

func sniffHaskell(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding

	for _, m := range haskellLiteralRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
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
	for _, m := range haskellImportRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		module := content[m[2]:m[3]]
		var local string
		if m[4] >= 0 {
			local = content[m[4]:m[5]]
		} else {
			local = module
			if dot := strings.LastIndex(module, "."); dot >= 0 {
				local = module[dot+1:]
			}
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
