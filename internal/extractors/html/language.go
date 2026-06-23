package html

import (
	"github.com/cajasmota/grafel/internal/treesitter/ts"
	tshtml "github.com/cajasmota/grafel/internal/treesitter/ts/grammars/html"
	tsofficial "github.com/cajasmota/grafel/internal/treesitter/ts/official"
)

// htmlGrammar returns the tree-sitter grammar for HTML for the extractor's
// inline-parse fallback (B2 cutover, #5418, ADR 0023). The extractor traverses
// the binding-agnostic ts façade; this is the single place that names a concrete
// binding.
func htmlGrammar() ts.Language { return tshtml.Language() }

// htmlAdapter is the binding adapter used to construct parsers in the fallback.
var htmlAdapter = tsofficial.New()
