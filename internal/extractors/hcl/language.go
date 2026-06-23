package hcl

import (
	"github.com/cajasmota/grafel/internal/treesitter/ts"
	tshcl "github.com/cajasmota/grafel/internal/treesitter/ts/grammars/hcl"
	tsofficial "github.com/cajasmota/grafel/internal/treesitter/ts/official"
)

// hclGrammar returns the tree-sitter grammar for HCL for the extractor's
// inline-parse fallback (B2 cutover, #5418, ADR 0023). The extractor traverses
// the binding-agnostic ts façade; this is the single place that names a concrete
// binding.
func hclGrammar() ts.Language { return tshcl.Language() }

// hclAdapter is the binding adapter used to construct parsers in the fallback.
var hclAdapter = tsofficial.New()
