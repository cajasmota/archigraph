// Phase-D MCP query handlers that operate against the lazy-mmap graph
// cache instead of re-parsing graph.json. These are the building blocks
// the daemon will expose over its RPC surface in the follow-up wiring
// PR (the standalone `grafel mcp serve` is deleted per ADR-0017).
//
// The four handlers covered here map onto the canonical MCP tools
// listed in the Phase-D plan:
//
//	read_entity     -> ReadEntity
//	find_references -> FindReferences
//	list_residuals  -> ListResiduals
//	submit_repair   -> SubmitRepair (graph-cache-aware existence check)
//
// list_residuals and submit_repair touch the on-disk
// enrichment-candidates.json / repair.json files; the graph cache is
// consulted only to validate target entity IDs cheaply (without a JSON
// reparse). That alone removes the per-call 50 MB JSON allocation.

package mcp

import (
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"

	fb "github.com/cajasmota/grafel/internal/graph/fbgraph"
)

// EntityView is a flat, copy-once projection of an Entity decoded from
// the mmap'd buffer. Callers receive plain Go strings so they can use
// the value after the underlying mmap is released by the cache.
type EntityView struct {
	ID            string
	QualifiedName string
	Kind          string
	Subtype       string
	Module        string
	Name          string
	SourceFile    string
	SourceLine    int
	SourceCol     int
}

// RelationshipView mirrors EntityView for relationships.
type RelationshipView struct {
	FromID string
	ToID   string
	Kind   string
}

// QueryService is the per-cache façade exposed to MCP handlers. It
// holds no graph state of its own — every call routes through the
// cache.
type QueryService struct {
	cache *Cache
}

// NewQueryService wires a QueryService to a cache. Pass the daemon's
// long-lived cache here.
func NewQueryService(c *Cache) *QueryService {
	return &QueryService{cache: c}
}

// ReadEntity returns the entity with the given ID from graph.fb at
// fbPath. Returns (nil, nil) when the entity is absent from the graph.
//
// Memory: this allocates ~9 strings (one per Entity field) plus the
// surrounding struct. Compare to graph.json's full document unmarshal
// (~640 k allocs / ~50 MB on the fixture-b graph).
func (q *QueryService) ReadEntity(fbPath, id string) (*EntityView, error) {
	r, release, err := q.cache.Get(fbPath)
	if err != nil {
		return nil, err
	}
	defer release()
	ent := r.LookupEntityByID(id)
	if ent == nil {
		return nil, nil
	}
	return entityViewFrom(ent), nil
}

// FindReferences returns every inbound edge (find_references). The
// scan is O(R) over the relationship vector but allocates only the
// returned slice + per-hit RelationshipView wrappers.
func (q *QueryService) FindReferences(fbPath, id string) ([]RelationshipView, error) {
	r, release, err := q.cache.Get(fbPath)
	if err != nil {
		return nil, err
	}
	defer release()
	rels := r.IterateRelationshipsToID(id)
	out := make([]RelationshipView, 0, len(rels))
	for _, rel := range rels {
		out = append(out, relViewFrom(rel))
	}
	return out, nil
}

// ListEntitiesByKind is the FilterEntitiesByKind path. It is used by
// the residuals/repair flows below to enumerate repair-candidate
// targets without a JSON parse.
func (q *QueryService) ListEntitiesByKind(fbPath, kind string) ([]EntityView, error) {
	r, release, err := q.cache.Get(fbPath)
	if err != nil {
		return nil, err
	}
	defer release()
	ents := r.FilterEntitiesByKind(kind)
	out := make([]EntityView, 0, len(ents))
	for _, e := range ents {
		out = append(out, *entityViewFrom(e))
	}
	return out, nil
}

// EntityExists is the cheap existence check used by submit_repair's
// "bind_to_entity" validation. It decodes nothing beyond the id field.
func (q *QueryService) EntityExists(fbPath, id string) (bool, error) {
	r, release, err := q.cache.Get(fbPath)
	if err != nil {
		return false, err
	}
	defer release()
	return r.LookupEntityByID(id) != nil, nil
}

// ResidualEdge is the on-graph projection of a repair_edge candidate.
// Phase D's list_residuals walks the relationship vector once, filters
// by a "disposition" property tag, and returns a stable er:<hex> id
// derived from (from_id, to_id, kind) so subsequent submit_repair
// calls can address the edge.
type ResidualEdge struct {
	ID         string // "er:" + 16 hex chars of fnv1a(fromID, toID, kind)
	FromID     string
	ToID       string
	Kind       string
	Properties map[string]string
}

// ListResiduals returns every relationship tagged as a residual (the
// disposition property is one of the bug/unresolved markers emitted by
// the resolver in pass 4 of the indexer; see ADR-0015).
//
// limit/offset bound the returned slice (pagination — same semantics as
// the standalone MCP tool's grafel_list_residuals).
func (q *QueryService) ListResiduals(fbPath string, limit, offset int) ([]ResidualEdge, error) {
	if limit <= 0 {
		limit = 20
	}
	r, release, err := q.cache.Get(fbPath)
	if err != nil {
		return nil, err
	}
	defer release()

	out := make([]ResidualEdge, 0, limit)
	seen := 0
	for i := 0; i < r.RelationshipCount(); i++ {
		rel := r.RelationshipAt(i)
		if rel == nil {
			continue
		}
		props := relPropsMap(rel)
		if !isResidualProps(props) {
			continue
		}
		if seen < offset {
			seen++
			continue
		}
		if len(out) >= limit {
			break
		}
		out = append(out, ResidualEdge{
			ID:         residualEdgeID(string(rel.FromId()), string(rel.ToId()), string(rel.Kind())),
			FromID:     string(rel.FromId()),
			ToID:       string(rel.ToId()),
			Kind:       string(rel.Kind()),
			Properties: props,
		})
		seen++
	}
	return out, nil
}

// RepairResolution is the agent's proposal payload for SubmitRepair.
// Matches the wire shape of grafel_submit_repair but with the
// existence checks promoted to the call boundary.
type RepairResolution struct {
	EdgeID         string
	Kind           string // bind_to_entity | reclassify_as_external | reclassify_as_dynamic | reclassify_as_resolved | abandon
	TargetEntityID string
	Module         string
	NewTarget      string
	Confidence     float64
	Reasoning      string
	Source         string
}

// SubmitRepair validates a resolution against the cached graph. It
// returns the canonical (fromID, toID, kind) triple resolved from
// EdgeID so the caller can append the resolution to repair.json. We
// keep the on-disk write out of this layer — handlers own
// persistence; this is the cache-aware validation core.
//
// Validation rules:
//   - The residual edge must exist (and be flagged residual).
//   - For bind_to_entity, TargetEntityID must resolve to an entity.
//   - For reclassify_as_resolved, NewTarget must resolve to an entity.
//   - For reclassify_as_external, Module must be non-empty.
func (q *QueryService) SubmitRepair(fbPath string, res RepairResolution) (*ResidualEdge, error) {
	r, release, err := q.cache.Get(fbPath)
	if err != nil {
		return nil, err
	}
	defer release()

	var match *ResidualEdge
	for i := 0; i < r.RelationshipCount(); i++ {
		rel := r.RelationshipAt(i)
		if rel == nil {
			continue
		}
		id := residualEdgeID(string(rel.FromId()), string(rel.ToId()), string(rel.Kind()))
		if id != res.EdgeID {
			continue
		}
		props := relPropsMap(rel)
		if !isResidualProps(props) {
			return nil, fmt.Errorf("edge %s is not a residual", res.EdgeID)
		}
		match = &ResidualEdge{
			ID:         id,
			FromID:     string(rel.FromId()),
			ToID:       string(rel.ToId()),
			Kind:       string(rel.Kind()),
			Properties: props,
		}
		break
	}
	if match == nil {
		return nil, fmt.Errorf("residual edge %s not found", res.EdgeID)
	}

	switch res.Kind {
	case "bind_to_entity":
		if res.TargetEntityID == "" {
			return nil, errors.New("bind_to_entity requires target_entity_id")
		}
		if r.LookupEntityByID(res.TargetEntityID) == nil {
			return nil, fmt.Errorf("target entity %s not found in graph", res.TargetEntityID)
		}
	case "reclassify_as_resolved":
		if res.NewTarget == "" {
			return nil, errors.New("reclassify_as_resolved requires new_target")
		}
		if r.LookupEntityByID(res.NewTarget) == nil {
			return nil, fmt.Errorf("new_target %s not found in graph", res.NewTarget)
		}
	case "reclassify_as_external":
		if res.Module == "" {
			return nil, errors.New("reclassify_as_external requires module")
		}
	case "reclassify_as_dynamic", "abandon":
		// no graph-side validation
	default:
		return nil, fmt.Errorf("unknown repair kind: %s", res.Kind)
	}
	return match, nil
}

// --- helpers ---------------------------------------------------------

func entityViewFrom(e *fb.Entity) *EntityView {
	return &EntityView{
		ID:            string(e.Id()),
		QualifiedName: string(e.QualifiedName()),
		Kind:          string(e.Kind()),
		Subtype:       string(e.Subtype()),
		Module:        string(e.Module()),
		Name:          string(e.Name()),
		SourceFile:    string(e.SourceFile()),
		SourceLine:    int(e.SourceLine()),
		SourceCol:     int(e.SourceCol()),
	}
}

func relViewFrom(r *fb.Relationship) RelationshipView {
	return RelationshipView{
		FromID: string(r.FromId()),
		ToID:   string(r.ToId()),
		Kind:   string(r.Kind()),
	}
}

func relPropsMap(r *fb.Relationship) map[string]string {
	n := r.PropertiesLength()
	if n == 0 {
		return nil
	}
	out := make(map[string]string, n)
	for i := 0; i < n; i++ {
		var pe fb.PropertyEntry
		if !r.Properties(&pe, i) {
			continue
		}
		out[string(pe.Key())] = string(pe.Value())
	}
	return out
}

// isResidualProps decides whether a relationship is a residual edge
// based on its disposition property. Matches the keys emitted by the
// resolver in pass 4 of the indexer.
func isResidualProps(props map[string]string) bool {
	if len(props) == 0 {
		return false
	}
	d := props["disposition"]
	switch d {
	case "bug_extractor", "bug_resolver", "residual", "unresolved":
		return true
	}
	// Also treat "repair_pending" / explicit repair tags as residuals.
	if props["repair_pending"] == "true" {
		return true
	}
	return false
}

// residualEdgeID derives a stable er:<hex16> identifier from the edge
// triple. fnv-1a 64-bit is plenty for collision avoidance inside a
// single graph (~10^9 edges before birthday risk); the hash is purely
// addressing, not security.
func residualEdgeID(fromID, toID, kind string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fromID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(toID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(kind))
	sum := h.Sum64()
	buf := make([]byte, 8)
	for i := 0; i < 8; i++ {
		buf[7-i] = byte(sum >> (i * 8))
	}
	return "er:" + hex.EncodeToString(buf)
}
