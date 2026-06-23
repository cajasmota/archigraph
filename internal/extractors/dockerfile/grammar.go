package dockerfile

import (
	"github.com/cajasmota/grafel/internal/treesitter/ts"
	tsdockerfile "github.com/cajasmota/grafel/internal/treesitter/ts/grammars/dockerfile"
	tsofficial "github.com/cajasmota/grafel/internal/treesitter/ts/official"
)

// Dockerfile grammar provider for the extractor's inline-parse fallback (B2
// cutover, #5418, ADR 0023). The extractor traverses the binding-agnostic ts
// façade; this is the single place that names a concrete binding.

func dockerfileGrammar() ts.Language { return tsdockerfile.Language() }

var dockerfileAdapter = tsofficial.New()
