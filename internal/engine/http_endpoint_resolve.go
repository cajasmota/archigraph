// Phase-2 post-pass for the synthetic http_endpoint entities emitted by
// http_endpoint_synthesis.go.
//
// Phase 1 emits one synthetic http_endpoint per route with a
// `source_handler` property of the form "<HandlerKind>:<HandlerName>"
// but deliberately does NOT emit edges to the handler. Emitting unresolved
// edges in Phase 1 inflated bug-rate because every dangling target counts
// as a resolver failure.
//
// Phase 2 (this file) runs AFTER the merged entity table is assembled but
// BEFORE EntityIDs are stamped. It:
//
//  1. Builds a (kind, name, sourceFile) → record-pointer index over the
//     merged set.
//  2. For each synthetic http_endpoint with a source_handler property:
//     a. Parses the property into (handlerKind, handlerName).
//     b. Resolves to a real entity in the same SourceFile (handlers and
//     their owning routes always live in the same file by construction
//     of Phase 1).
//     c. If resolved: appends an IMPLEMENTS edge (handler → synthetic)
//     to the handler's embedded Relationships, then clears the
//     source_handler property (its job is done).
//     d. If NOT resolved: marks the synthetic for removal so it never
//     reaches the resolver as an orphan.
//
// Returning a NEW slice of EntityRecords (with unresolved synthetics
// dropped) keeps the data flow obvious and avoids in-place slice
// shuffling at the call site.
//
// Refs #534 Phase 2.
package engine

import (
	"strings"

	"github.com/cajasmota/archigraph/internal/types"
)

// resolverKindEquivalents maps a synthesizer-emitted handler Kind to
// the list of fallback Kinds the resolver should try when the exact
// match misses. The synthesizers were written against an older
// extractor convention (Controller / View) but the per-language
// extractors land function-shaped handlers as SCOPE.Operation and
// class-shaped handlers as SCOPE.Class. Without this fallback the
// resolver drops every Flask / FastAPI / Express endpoint whose
// handler is a plain function. #753.
var resolverKindEquivalents = map[string][]string{
	"Controller":      {"SCOPE.Operation", "SCOPE.Function", "View"},
	"View":            {"SCOPE.Operation", "SCOPE.Class", "Controller"},
	"SCOPE.Operation": {"Controller", "View"},
	"SCOPE.Class":     {"View", "Controller"},
	"SCOPE.Function":  {"SCOPE.Operation", "Controller"},
}

// ResolveHTTPEndpointStats reports counters for a single resolve pass.
// Exposed so cmd/archigraph can log a stats line analogous to the
// import-aware resolver line.
type ResolveHTTPEndpointStats struct {
	Synthetics      int // total http_endpoint records seen
	HandlerResolved int // source_handler resolved → IMPLEMENTS edge emitted
	HandlerDropped  int // synthetics dropped because source_handler unresolved
	NoHandlerProp   int // synthetics with no source_handler property (kept as-is)
}

// ResolveHTTPEndpointHandlers runs the Phase-2 post-pass over `merged`.
// Returns a (possibly shorter) slice with unresolved synthetics removed,
// plus stats for verbose logging.
//
// `merged` MUST already be sorted in canonical order (entity-id
// disambiguation depends on first-writer-wins). The slice may be
// returned as-is if no synthetics were dropped.
func ResolveHTTPEndpointHandlers(merged []types.EntityRecord) ([]types.EntityRecord, ResolveHTTPEndpointStats) {
	var stats ResolveHTTPEndpointStats

	// (kind, name, sourceFile) → index into `merged`.
	type key struct{ kind, name, sourceFile string }
	idx := make(map[key]int, len(merged))
	// (kind, name) → first index — used as cross-file fallback for
	// handlers declared in a different module than the route synthetic
	// (Django composed routes, Express imported controllers, etc.).
	// See #753: the original same-file-only resolver dropped every
	// Django-composed and imported-controller endpoint because the
	// view/controller body lives in a different file than the URL
	// dispatcher. Falling back to a global (kind, name) match keeps
	// those endpoints alive so the corpus-wide response-shape pass
	// can locate and scan the actual handler body.
	type knKey struct{ kind, name string }
	globalIdx := make(map[knKey]int, len(merged))
	for i := range merged {
		r := &merged[i]
		if r.Kind == httpEndpointKind {
			continue // never resolve a synthetic against another synthetic
		}
		k := key{r.Kind, r.Name, r.SourceFile}
		if _, ok := idx[k]; !ok {
			idx[k] = i
		}
		gk := knKey{r.Kind, r.Name}
		if _, ok := globalIdx[gk]; !ok {
			globalIdx[gk] = i
		}
	}

	// Collect indices of synthetics to drop (unresolved handlers).
	drop := map[int]bool{}

	for i := range merged {
		r := &merged[i]
		if r.Kind != httpEndpointKind {
			continue
		}
		stats.Synthetics++

		handlerRef := ""
		if r.Properties != nil {
			handlerRef = r.Properties["source_handler"]
		}
		if handlerRef == "" {
			stats.NoHandlerProp++
			continue
		}

		// source_handler is "<HandlerKind>:<HandlerName>" — split on the
		// FIRST colon only because Spring-style names can themselves
		// contain a colon-less path identifier but kinds never do.
		hk, hn, ok := splitHandlerRef(handlerRef)
		if !ok {
			// Malformed — drop the synthetic to avoid leaking the bad
			// reference into the graph.
			drop[i] = true
			stats.HandlerDropped++
			continue
		}

		// Prefer same-file match (handlers and route synthetics are
		// often emitted from the same file by Phase 1 construction).
		handlerIdx, found := idx[key{hk, hn, r.SourceFile}]
		if !found {
			// Cross-file fallback (#753). Django composed routes record
			// a `View:<ViewSet>` handler reference whose entity lives in
			// views.py while the synthetic lives in urls.py. Express
			// imported controllers have the same shape — handler in
			// controllers/users.js, route registration in routes.js.
			// Try the global (kind, name) index before giving up.
			//
			// Skip the cross-file fallback when the reference is
			// Kind="Route" + Name=<path> — that's Spring's
			// "synthesizer didn't have the method name" placeholder
			// and would always collide with the synthetic itself.
			if hk == "Route" {
				stats.HandlerDropped++
				drop[i] = true
				continue
			}
			handlerIdx, found = globalIdx[knKey{hk, hn}]
			if !found {
				// Cross-kind fallback. Synthesizers historically emit
				// `Controller:<name>` but the Python YAML rules + the
				// generic SCOPE extractor produce `SCOPE.Operation`
				// for function-shaped handlers (Flask def, FastAPI
				// def, Express function expressions). Likewise the
				// Java AST pass emits `SCOPE.Operation:Class.method`
				// while older synthesizers still emit `Controller:method`.
				// Try the known equivalence classes before dropping —
				// without this fallback every Flask synthetic with a
				// Controller-shaped ref gets dropped because the
				// matching entity has kind SCOPE.Operation. #753.
				for _, altKind := range resolverKindEquivalents[hk] {
					if hi, ok := globalIdx[knKey{altKind, hn}]; ok {
						handlerIdx = hi
						found = true
						break
					}
				}
			}
			if !found {
				stats.HandlerDropped++
				drop[i] = true
				continue
			}
		}

		// Resolved. Append an embedded IMPLEMENTS edge on the handler.
		// Use placeholder ID stubs (Kind:Name) for the endpoints; the
		// resolver in buildDocument rewrites these against the stamped
		// entity index after we return.
		handler := &merged[handlerIdx]
		fromStub := handler.Kind + ":" + handler.Name
		toStub := r.Kind + ":" + r.Name
		handler.Relationships = append(handler.Relationships, types.RelationshipRecord{
			FromID: fromStub,
			ToID:   toStub,
			Kind:   implementsEdgeKind,
			Properties: map[string]string{
				"pattern_type": "http_endpoint_synthesis_resolved",
				"framework":    propOr(r, "framework", ""),
			},
		})
		// Clear the now-redundant property.
		delete(r.Properties, "source_handler")
		stats.HandlerResolved++
	}

	if len(drop) == 0 {
		return merged, stats
	}
	out := make([]types.EntityRecord, 0, len(merged)-len(drop))
	for i := range merged {
		if drop[i] {
			continue
		}
		out = append(out, merged[i])
	}
	return out, stats
}

// splitHandlerRef parses "<Kind>:<Name>" into its parts. Returns ok=false
// when the input lacks a colon or has an empty kind/name.
func splitHandlerRef(ref string) (kind, name string, ok bool) {
	i := strings.Index(ref, ":")
	if i <= 0 || i == len(ref)-1 {
		return "", "", false
	}
	return ref[:i], ref[i+1:], true
}

// propOr returns r.Properties[k] or fallback if missing/nil.
func propOr(r *types.EntityRecord, k, fallback string) string {
	if r.Properties == nil {
		return fallback
	}
	if v, ok := r.Properties[k]; ok && v != "" {
		return v
	}
	return fallback
}
