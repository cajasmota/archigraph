package golang

import (
	"github.com/cajasmota/grafel/internal/treesitter/ts"
	tsgolang "github.com/cajasmota/grafel/internal/treesitter/ts/grammars/golang"
	tsofficial "github.com/cajasmota/grafel/internal/treesitter/ts/official"
)

// The Go extractor's inline-parse fallback uses the official
// tree-sitter/go-tree-sitter binding (B2 cutover, ADR 0023, #5418), ABI-pinned
// to tree-sitter-go v0.23.4 against runtime v0.24.0. The extractor traverses the
// ts façade; this is the single place that names a concrete binding.

func goGrammar() ts.Language { return tsgolang.Language() }

var goAdapter = tsofficial.New()
