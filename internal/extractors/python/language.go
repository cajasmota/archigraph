package python

import (
	"github.com/cajasmota/grafel/internal/treesitter/ts"
	tspython "github.com/cajasmota/grafel/internal/treesitter/ts/grammars/python"
	tsofficial "github.com/cajasmota/grafel/internal/treesitter/ts/official"
)

// The Python extractor's inline-parse fallback uses the official
// tree-sitter/go-tree-sitter binding (B2 cutover, ADR 0023, #5418), ABI-pinned
// to tree-sitter-python v0.23.6 (ABI 14) against runtime v0.24.0. The extractor
// traverses the ts façade; this is the single place that names a concrete
// binding.

func pythonGrammar() ts.Language { return tspython.Language() }

var pythonAdapter = tsofficial.New()
