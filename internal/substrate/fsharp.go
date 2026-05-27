// F# constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises module-level binding shapes:
//   - `let X = "literal"` / `let X : string = "literal"`
//   - `[<Literal>] let X = "literal"`
//   - `let X = Environment.GetEnvironmentVariable("Y") |> Option.ofObj |> Option.defaultValue "default"`
//   - `let X = defaultArg (Environment.GetEnvironmentVariable("Y") |> Option.ofObj) "default"`
//   - `open Module.Path`
package substrate

import (
	"regexp"
	"strings"
)

func init() { Register("fsharp", sniffFSharp) }

// fsharpLiteralRe matches `[[<Literal>]] let NAME [: string] = "value"`.
// Capture groups: 1=name, 2=value.
var fsharpLiteralRe = regexp.MustCompile(
	`(?m)^\s*(?:\[<Literal>\]\s+)?let\s+([A-Za-z_][\w']*)\s*` +
		`(?::\s*string\s*)?` +
		`=\s*"([^"\n\r]{0,512})"`,
)

// fsharpEnvDefaultArgRe matches the `defaultArg ... "default"` shape.
// Capture groups: 1=name, 2=env-var, 3=default.
var fsharpEnvDefaultArgRe = regexp.MustCompile(
	`(?m)^\s*let\s+([A-Za-z_][\w']*)\s*=\s*defaultArg\s+\(\s*` +
		`Environment\.GetEnvironmentVariable\s*\(?\s*"([^"\n]{1,128})"\s*\)?` +
		`\s*\|>\s*Option\.ofObj\s*\)\s*"([^"\n\r]{0,512})"`,
)

// fsharpEnvPipeRe matches the `|> Option.defaultValue "default"` shape.
// Capture groups: 1=name, 2=env-var, 3=default.
var fsharpEnvPipeRe = regexp.MustCompile(
	`(?m)^\s*let\s+([A-Za-z_][\w']*)\s*=\s*` +
		`Environment\.GetEnvironmentVariable\s*\(?\s*"([^"\n]{1,128})"\s*\)?` +
		`\s*\|>\s*Option\.ofObj\s*\|>\s*Option\.defaultValue\s+"([^"\n\r]{0,512})"`,
)

// fsharpOpenRe matches `open Module.Path`.
var fsharpOpenRe = regexp.MustCompile(
	`(?m)^\s*open\s+([\w.]+)`,
)

func sniffFSharp(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range fsharpEnvDefaultArgRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range fsharpEnvPipeRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 8 || seen[m[2]] {
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
	for _, m := range fsharpLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range fsharpOpenRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		qualified := content[m[2]:m[3]]
		local := qualified
		if dot := strings.LastIndex(qualified, "."); dot >= 0 {
			local = qualified[dot+1:]
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
