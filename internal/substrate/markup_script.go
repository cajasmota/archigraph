// Markup-with-script substrate dispatcher (#2763 Phase 0 T3).
//
// Svelte (.svelte), Vue (.vue), and Astro (.astro) single-file components
// embed JS/TS inside `<script>` blocks. The constant-binding shapes inside
// those blocks are identical to plain JS/TS, so we extract every
// `<script>...</script>` body and run sniffJSTS over the concatenation.
//
// The token positions inside the embedded script blocks differ from the
// outer file, but for Phase 0 only the ident → value mapping matters; the
// line numbers are best-effort against the original file content.
package substrate

import "regexp"

func init() {
	Register("svelte", sniffMarkupScript)
	Register("vue", sniffMarkupScript)
	Register("astro", sniffMarkupScript)
}

// scriptBlockRe matches `<script ...>` ... `</script>` (case-insensitive).
// Capture group 1 is the inner body.
var scriptBlockRe = regexp.MustCompile(
	`(?si)<script\b[^>]*>(.*?)</script>`,
)

// sniffMarkupScript extracts every <script> block and runs sniffJSTS over
// each, offsetting line numbers so the recorded Line matches the original
// markup file (not the script-only slice).
func sniffMarkupScript(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	for _, m := range scriptBlockRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		body := content[m[2]:m[3]]
		bodyLineOffset := lineOfOffset(content, m[2]) - 1
		for _, b := range sniffJSTS(body) {
			b.Line += bodyLineOffset
			out = append(out, b)
		}
	}
	return out
}
