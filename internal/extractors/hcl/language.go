package hcl

import (
	tshcl "github.com/smacker/go-tree-sitter/hcl"

	"github.com/cajasmota/grafel/internal/treesitter/ts"
	tssmacker "github.com/cajasmota/grafel/internal/treesitter/ts/smacker"
)

// hclGrammar returns the tree-sitter grammar for HCL for the extractor's
// inline-parse fallback (B2 Phase 1, #5418, ADR 0023). The extractor traverses
// the binding-agnostic ts façade; this is the single place that names a concrete
// binding. Smacker-backed in both build configurations (no official HCL
// grammar module is wired yet), so the file is untagged: `go build` and
// `go build -tags ts_official` both compile it unchanged.
func hclGrammar() ts.Language { return tssmacker.WrapLanguage(tshcl.GetLanguage()) }

// hclAdapter is the binding adapter used to construct parsers in the fallback.
var hclAdapter = tssmacker.New()
