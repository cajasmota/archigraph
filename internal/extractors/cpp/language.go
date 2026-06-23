package cpp

import (
	"github.com/cajasmota/grafel/internal/treesitter/ts"
	tsc "github.com/cajasmota/grafel/internal/treesitter/ts/grammars/c"
	tscpp "github.com/cajasmota/grafel/internal/treesitter/ts/grammars/cpp"
	tsofficial "github.com/cajasmota/grafel/internal/treesitter/ts/official"
)

// C/C++ grammar providers for the extractor's inline-parse fallback (B2 cutover,
// #5418, ADR 0023). The extractor traverses the binding-agnostic ts façade; this
// is the single place that names a concrete binding.

// cGrammar returns the tree-sitter grammar for C.
func cGrammar() ts.Language { return tsc.Language() }

// cppGrammar returns the tree-sitter grammar for C++.
func cppGrammar() ts.Language { return tscpp.Language() }

// cppAdapter is the binding adapter used to construct parsers in the fallback.
var cppAdapter = tsofficial.New()
