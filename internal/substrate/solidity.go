// Solidity constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises file-level binding shapes:
//   - `string constant NAME = "literal";`
//   - `string public constant NAME = "literal";`
//   - `bytes32 constant NAME = "literal";` (treated as literal)
//   - `import "./path.sol";` and `import {X, Y as Z} from "./path.sol";`
//
// Solidity has no runtime env-var access — all state is on-chain or
// passed as constructor args. The env_fallback shape is not_applicable.
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("solidity", sniffSolidity) }

// solidityLiteralRe matches `[string|bytes32] [visibility] constant NAME = "value";`.
// Capture groups: 1=name, 2=value.
var solidityLiteralRe = regexp.MustCompile(
	`(?m)^\s*(?:string|bytes32|bytes)\s+(?:(?:public|private|internal)\s+)?constant\s+` +
		`([A-Za-z_][\w]*)\s*=\s*"([^"\n\r]{0,512})"\s*;`,
)

// solidityImportFullRe matches `import "./path.sol";`.
var solidityImportFullRe = regexp.MustCompile(
	`(?m)^\s*import\s+"([^"\n]+)"\s*;`,
)

// solidityImportNamedRe matches `import {X, Y as Z} from "./path.sol";`.
// Capture groups: 1=specifier list, 2=module path.
var solidityImportNamedRe = regexp.MustCompile(
	`(?m)^\s*import\s*\{([^}]+)\}\s*from\s*"([^"\n]+)"\s*;`,
)

func sniffSolidity(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding

	for _, m := range solidityLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range solidityImportFullRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		module := content[m[2]:m[3]]
		// Local name: last path segment without .sol.
		local := module
		if slash := strings.LastIndex(local, "/"); slash >= 0 {
			local = local[slash+1:]
		}
		local = strings.TrimSuffix(local, ".sol")
		if !isJSIdent(local) {
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
	for _, m := range solidityImportNamedRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 6 {
			continue
		}
		specifiers := content[m[2]:m[3]]
		module := content[m[4]:m[5]]
		line := lineOfOffset(content, m[0])
		for _, spec := range strings.Split(specifiers, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			local := spec
			if asIdx := strings.Index(spec, " as "); asIdx > 0 {
				local = strings.TrimSpace(spec[asIdx+4:])
			}
			if !isJSIdent(local) {
				continue
			}
			out = append(out, Binding{
				Ident:        local,
				Line:         line,
				Provenance:   ProvenanceCrossFile,
				Confidence:   0.6,
				ImportSource: module,
			})
		}
	}
	return out
}
