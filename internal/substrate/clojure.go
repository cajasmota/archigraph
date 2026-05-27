// Clojure constant-binding sniffer (#2763 Phase 0 T3).
//
// Recognises top-level binding shapes:
//   - `(def name "literal")` / `(def ^:private name "literal")`
//   - `(def name (or (System/getenv "Y") "default"))`
//   - `(ns my.ns (:require [other.ns :as alias]))` — captures :as aliases
//
// S-expression syntax means line-anchored regex is fragile; we accept
// reasonable indentation and ignore in-string parens.
package substrate

import (
	"regexp"
)

func init() { Register("clojure", sniffClojure) }

// clojureLiteralRe matches `(def [meta] name "value")` at the start of a
// line. The optional `^:private` / `^{:doc "..."}` metadata is skipped via
// a permissive non-greedy match.
//
// Capture groups: 1=name, 2=value.
var clojureLiteralRe = regexp.MustCompile(
	`(?m)^\(def(?:[\s\^][^"\s]*\s*(?:"[^"\n]*"\s*)?)?\s+([A-Za-z_!?*+\-][\w!?*+\-]*)\s+` +
		`"([^"\n\r]{0,512})"\s*\)`,
)

// clojureEnvOrRe matches `(def name (or (System/getenv "Y") "default"))`.
// Capture groups: 1=name, 2=env-var, 3=default.
var clojureEnvOrRe = regexp.MustCompile(
	`(?m)^\(def\s+([A-Za-z_!?*+\-][\w!?*+\-]*)\s+` +
		`\(or\s+\(System/getenv\s+"([^"\n]{1,128})"\)\s+` +
		`"([^"\n\r]{0,512})"\s*\)\s*\)`,
)

// clojureRequireRe matches a single `[ns.path :as alias]` entry inside a
// (:require ...) form. Captured separately because Clojure's namespace
// form spans many lines.
//
// Capture groups: 1=namespace, 2=alias.
var clojureRequireRe = regexp.MustCompile(
	`\[([\w.\-]+)(?:\s+:as\s+([\w.\-]+))?\s*\]`,
)

// clojureNSRe scopes the require-match scan to the (ns ...) form so we
// don't accidentally match unrelated vectors.
var clojureNSRe = regexp.MustCompile(`(?s)\(ns\s+[^()]+\([\s\S]*?\)\s*\)`)

func sniffClojure(content string) []Binding {
	if content == "" {
		return nil
	}
	var out []Binding
	seen := map[int]bool{}

	for _, m := range clojureEnvOrRe.FindAllStringSubmatchIndex(content, -1) {
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
	for _, m := range clojureLiteralRe.FindAllStringSubmatchIndex(content, -1) {
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
	// Require aliases — scan within the (ns ...) form only.
	for _, ns := range clojureNSRe.FindAllStringIndex(content, -1) {
		block := content[ns[0]:ns[1]]
		for _, m := range clojureRequireRe.FindAllStringSubmatchIndex(block, -1) {
			if len(m) < 6 {
				continue
			}
			module := block[m[2]:m[3]]
			var local string
			if m[4] >= 0 {
				local = block[m[4]:m[5]]
			} else {
				local = module
			}
			out = append(out, Binding{
				Ident:        local,
				Line:         lineOfOffset(content, ns[0]+m[0]),
				Provenance:   ProvenanceCrossFile,
				Confidence:   0.6,
				ImportSource: module,
			})
		}
	}
	return out
}
