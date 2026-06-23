package yaml

import (
	"github.com/cajasmota/grafel/internal/treesitter/ts"
	tsyaml "github.com/cajasmota/grafel/internal/treesitter/ts/grammars/yaml"
	tsofficial "github.com/cajasmota/grafel/internal/treesitter/ts/official"
)

// yamlGrammar returns the tree-sitter grammar for YAML for the extractor's
// inline-parse fallback (B2 cutover, #5418, ADR 0023). The extractor traverses
// the binding-agnostic ts façade; this is the single place that names a concrete
// binding.
func yamlGrammar() ts.Language { return tsyaml.Language() }

// yamlAdapter is the binding adapter used to construct parsers in the fallback.
var yamlAdapter = tsofficial.New()
