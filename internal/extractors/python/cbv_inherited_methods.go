// cbv_inherited_methods.go — Generic CBV inherited-method annotation.
//
// Issue #2011 — Django generic class-based views and DRF generic
// viewsets/views inherit a fixed contract of HTTP method handlers from
// their base classes. A `class PermitViewSet(ModelViewSet)` exposes
// `list / retrieve / create / update / partial_update / destroy`
// without declaring them. Earlier waves attempted to model this by
// synthesizing fake SCOPE.Operation entities for the inherited methods,
// which produced spurious "method exists but has no source" findings
// in audits AND inflated entity counts on every DRF project.
//
// Option A (this implementation): annotate the SCOPE.Component/class
// entity with an `inherited_methods` property listing every inherited
// method name from each recognised base class, comma-joined and
// deduplicated. Downstream tools that need to enumerate the class's
// API surface read the property directly without us having to
// fabricate operation entities or invent a synthetic provenance.
//
// We only handle the well-known generic CBV / DRF base classes
// enumerated in `cbvBaseInheritedMethods` below. Anything else gets no
// annotation — a deliberate trade-off: it's better to under-annotate
// than to invent inheritance from arbitrary user-defined bases whose
// method sets we don't know.

package python

import (
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

// cbvBaseInheritedMethods maps the bare leaf name of each recognised
// Django / DRF generic base class to the set of method names it
// contributes to its subclass's HTTP surface.
//
// Source: Django docs (generic class-based views) + DRF docs (generic
// viewsets / generic API views). The list intentionally covers ONLY
// the canonical HTTP-handler methods — helpers like `get_queryset`,
// `get_serializer_class`, `perform_create` etc. are excluded because
// subclasses override them as customization hooks rather than treat
// them as part of the public API contract.
var cbvBaseInheritedMethods = map[string][]string{
	// Django generic class-based views (django.views.generic.*).
	"View":                {"get", "post", "put", "patch", "delete", "head", "options", "trace"},
	"TemplateView":        {"get"},
	"RedirectView":        {"get", "post", "put", "patch", "delete", "head", "options"},
	"ListView":            {"get"},
	"DetailView":          {"get"},
	"FormView":            {"get", "post"},
	"CreateView":          {"get", "post"},
	"UpdateView":          {"get", "post"},
	"DeleteView":          {"get", "post", "delete"},
	"ArchiveIndexView":    {"get"},
	"YearArchiveView":     {"get"},
	"MonthArchiveView":    {"get"},
	"DayArchiveView":      {"get"},
	"WeekArchiveView":     {"get"},
	"TodayArchiveView":    {"get"},
	"DateDetailView":      {"get"},
	"ProcessFormView":     {"get", "post"},
	"BaseCreateView":      {"get", "post"},
	"BaseUpdateView":      {"get", "post"},
	"BaseDeleteView":      {"get", "post", "delete"},

	// DRF generic API views (rest_framework.generics.*).
	"GenericAPIView":      {},
	"CreateAPIView":       {"post"},
	"ListAPIView":         {"get"},
	"RetrieveAPIView":     {"get"},
	"DestroyAPIView":      {"delete"},
	"UpdateAPIView":       {"put", "patch"},
	"ListCreateAPIView":   {"get", "post"},
	"RetrieveUpdateAPIView":         {"get", "put", "patch"},
	"RetrieveDestroyAPIView":        {"get", "delete"},
	"RetrieveUpdateDestroyAPIView":  {"get", "put", "patch", "delete"},

	// DRF viewsets (rest_framework.viewsets.*).
	"ViewSet":             {},
	"GenericViewSet":      {},
	"ReadOnlyModelViewSet": {"list", "retrieve"},
	"ModelViewSet":        {"list", "retrieve", "create", "update", "partial_update", "destroy"},

	// DRF mixins (rest_framework.mixins.*).
	"CreateModelMixin":           {"create"},
	"ListModelMixin":             {"list"},
	"RetrieveModelMixin":         {"retrieve"},
	"UpdateModelMixin":           {"update", "partial_update"},
	"DestroyModelMixin":          {"destroy"},
}

// emitCBVInheritedMethodAnnotations walks every class entity in the
// file's slice. For each class whose EXTENDS edges include one or more
// recognised CBV / DRF base classes, it stamps an `inherited_methods`
// property holding the comma-joined union of method names contributed
// by those bases.
//
// We read EXTENDS edges (emitted by extractBaseClasses, #698) rather
// than re-parse the class header, so the rule fires consistently
// regardless of whether the base is `View`, `views.View`, or
// `django.views.generic.View` — extractBaseClasses normalises the leaf.
//
// Mutates *entities in place. Safe with nil/empty inputs.
func emitCBVInheritedMethodAnnotations(_ *sitter.Node, file extractor.FileInput, entities *[]types.EntityRecord) {
	if entities == nil || len(*entities) == 0 {
		return
	}
	for i := range *entities {
		e := &(*entities)[i]
		if e.SourceFile != file.Path {
			continue
		}
		if e.Kind != "SCOPE.Component" || e.Subtype != "class" {
			continue
		}
		seen := make(map[string]struct{})
		hadBase := false
		for _, r := range e.Relationships {
			if r.Kind != "EXTENDS" {
				continue
			}
			baseLeaf := r.Properties["base_name"]
			if baseLeaf == "" {
				baseLeaf = extractLeafFromExtendsToID(r.ToID)
			}
			if dot := strings.LastIndexByte(baseLeaf, '.'); dot >= 0 {
				baseLeaf = baseLeaf[dot+1:]
			}
			methods, ok := cbvBaseInheritedMethods[baseLeaf]
			if !ok {
				continue
			}
			hadBase = true
			for _, m := range methods {
				seen[m] = struct{}{}
			}
		}
		if !hadBase {
			continue
		}
		// Sort for deterministic property values across runs.
		ordered := make([]string, 0, len(seen))
		for m := range seen {
			ordered = append(ordered, m)
		}
		sort.Strings(ordered)
		if e.Properties == nil {
			e.Properties = make(map[string]string)
		}
		e.Properties["inherited_methods"] = strings.Join(ordered, ",")
		// Bases tag is useful for downstream audit; comma-joined leaves.
		bases := make([]string, 0)
		for _, r := range e.Relationships {
			if r.Kind != "EXTENDS" {
				continue
			}
			baseLeaf := r.Properties["base_name"]
			if baseLeaf == "" {
				baseLeaf = extractLeafFromExtendsToID(r.ToID)
			}
			if dot := strings.LastIndexByte(baseLeaf, '.'); dot >= 0 {
				baseLeaf = baseLeaf[dot+1:]
			}
			if _, ok := cbvBaseInheritedMethods[baseLeaf]; ok {
				bases = append(bases, baseLeaf)
			}
		}
		if len(bases) > 0 {
			e.Properties["cbv_bases"] = strings.Join(bases, ",")
		}
	}
}

// extractLeafFromExtendsToID returns the trailing colon-separated
// segment of an EXTENDS edge ToID. The structural-ref shape used by
// extractBaseClasses ends with the bare class name. Empty input yields
// "".
func extractLeafFromExtendsToID(toID string) string {
	if toID == "" {
		return ""
	}
	if idx := strings.LastIndexByte(toID, ':'); idx >= 0 {
		return toID[idx+1:]
	}
	return toID
}
