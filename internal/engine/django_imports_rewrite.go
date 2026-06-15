// Django models-import suffix rewrite.
//
// The YAML rule
//   from \S+\.models import (\w+)
// in `internal/engine/rules/python/frameworks/django.yaml` emits an
// IMPORTS edge with `ToID = Model:<captured-name>`. Django + DRF
// codebases routinely re-export Serializer / ViewSet / View classes
// through a sibling `models` module (or via barrel-style
// `from app.models import UserSerializer`), so the naive Model: prefix
// surfaces as kind-mismatch bug-resolver edges at resolve time (60
// instances on client-fixture-a per PR #580 wave-9 residual analysis).
//
// This pass rewrites the ToID prefix in-place on suffix heuristics:
//   Model:<X>Serializer        → Component:<X>Serializer
//   Model:<X>(View|ViewSet|...) → View:<X>...
//
// Other Model: edges (genuine Django ORM model imports) are left
// untouched. Python-only — gated by detector.go before invocation.
//
// Refs PR #580 wave-10 Chain-fix A.

package engine

import (
	"strings"

	"github.com/cajasmota/grafel/internal/types"
)

// viewSuffixes lists the Django CBV / DRF ViewSet base-class name
// suffixes the YAML `source_patterns` register as `entity_type: View`.
// Kept in sync with the View regex in django.yaml (lines 80-82).
var viewSuffixes = []string{
	"ViewSet",
	"Viewset", // tolerate the lowercase-`set` typo common in older codebases
	"ListView",
	"DetailView",
	"CreateView",
	"UpdateView",
	"DeleteView",
	"TemplateView",
	"FormView",
	"APIView",
	// Plain "View" comes last so "ListView" etc. match first via
	// length-ordering in classifyName below.
	"View",
}

// rewritePythonModelImports rewrites Model:<name> IMPORTS edge targets
// to Component:<name> when the captured name ends in "Serializer", or
// View:<name> when it ends in any viewSuffixes entry. The slice is
// modified in place (no allocation). Returns the same slice for
// composability with the surrounding pipeline.
func rewritePythonModelImports(rels []types.RelationshipRecord) []types.RelationshipRecord {
	for i := range rels {
		r := &rels[i]
		// Both IMPORTS (from django.yaml `from X.models import Y`) and
		// DEPENDS_ON (from sqlalchemy.yaml `from X import <PascalCase>`,
		// which fires on every Python file regardless of framework
		// detection) emit `Model:<name>` ToIDs that mismatch when the
		// captured name is actually a DRF Serializer or a CBV/ViewSet
		// class re-exported through a sibling `models` module.
		if r.Kind != "IMPORTS" && r.Kind != "DEPENDS_ON" {
			continue
		}
		const prefix = "Model:"
		if !strings.HasPrefix(r.ToID, prefix) {
			continue
		}
		name := r.ToID[len(prefix):]
		if name == "" {
			continue
		}
		switch classifyDjangoModelImportName(name) {
		case "Component":
			r.ToID = "Component:" + name
		case "View":
			r.ToID = "View:" + name
		}
	}
	return rels
}

// classifyDjangoModelImportName returns "Component" for names ending in
// "Serializer", "View" for names ending in any Django/DRF view-class
// suffix, and "" otherwise (meaning: keep the original Model: prefix).
func classifyDjangoModelImportName(name string) string {
	if strings.HasSuffix(name, "Serializer") {
		return "Component"
	}
	for _, suf := range viewSuffixes {
		if strings.HasSuffix(name, suf) {
			return "View"
		}
	}
	return ""
}
