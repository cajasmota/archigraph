package dashboard

// issue4849_schema_isolation_test.go — diagnostic for issue #4849.
//
// Coordinator's live finding: ~15.7% of acme-v3 nodes render isolated
// (Schema-isolated ≈1,359) and that count did NOT drop after the DTO
// field-membership reindex (#4845/#4856), even though DTO classes now have
// CONTAINS→field children and the dashboard SHAPE endpoint returns those
// fields correctly.
//
// Open question: is the bulk graph payload (/api/graph/{group}) DROPPING the
// new class→field CONTAINS edges (a payload bug), or are the remaining isolated
// Schemas legitimately edge-less (enums / standalone type-aliases / config
// schemas)?
//
// These tests pin the payload's edge-build behaviour for the DTO-class→field
// case so a regression (or a kind/degree filter) would be caught here rather
// than only in a live reindex.
//
// VERDICT (encoded by these tests): the bulk payload does NOT drop a
// Schema(class)→Schema(field) CONTAINS edge. The edge-build loop in
// serveGraphDense emits ANY relationship whose two endpoints are both visible,
// with no kind/degree/hub filter; Schema is not in the excluded set (only
// External and Module are). So once the field-membership CONTAINS edges are
// present in r.Doc.Relationships (post-reindex), both the class and the field
// become connected (degree≥1) and the edge appears in the payload.
//
// Therefore a *persisting* isolated-Schema count after reindex is NOT a payload
// bug for class→field edges — it is either (a) the live graph not yet
// re-indexed with #4845/#4856, or (b) Schemas that are legitimately edge-less
// (enums / type-aliases / config schemas with no field children and no other
// semantic edge). See TestIssue4849_LegitLeafSchemaIsIsolated for the latter.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

// graphResp is a parsed /api/graph payload for assertion convenience.
type graphResp struct {
	nodes []map[string]interface{}
	edges []map[string]interface{}
}

// hasEdge reports whether the payload contains an edge from→to of the given kind.
func (g graphResp) hasEdge(from, to, kind string) bool {
	for _, e := range g.edges {
		ef, _ := e["from_id"].(string)
		et, _ := e["to_id"].(string)
		ek, _ := e["kind"].(string)
		if ef == from && et == to && ek == kind {
			return true
		}
	}
	return false
}

// nodeDegree returns the "degree" field of a payload node as an int.
func nodeDegree(n map[string]interface{}) int {
	d, _ := n["degree"].(float64) // JSON numbers decode to float64
	return int(d)
}

// fetchGraphResponse calls GET /api/graph/testgrp and returns nodes + edges.
func fetchGraphResponse(t *testing.T, ts *httptest.Server) graphResp {
	t.Helper()
	resp, err := http.Get(ts.URL + "/api/graph/testgrp")
	if err != nil {
		t.Fatalf("GET /api/graph/testgrp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	out := graphResp{}
	if raw, ok := body["nodes"].([]interface{}); ok {
		for _, n := range raw {
			if m, ok := n.(map[string]interface{}); ok {
				out.nodes = append(out.nodes, m)
			}
		}
	}
	if raw, ok := body["edges"].([]interface{}); ok {
		for _, e := range raw {
			if m, ok := e.(map[string]interface{}); ok {
				out.edges = append(out.edges, m)
			}
		}
	}
	return out
}

// schemaClassEntity builds a DTO class Schema node (as JS/TS extractor emits:
// SCOPE.Schema, subtype "class"/"interface").
func schemaClassEntity(id, name string) graph.Entity {
	return graph.Entity{ID: id, Name: name, Kind: "SCOPE.Schema", Subtype: "class"}
}

// schemaFieldEntity builds a DTO field Schema node (SCOPE.Schema/field, #679),
// the child linked by the #4845 class→field CONTAINS edge.
func schemaFieldEntity(id, name string) graph.Entity {
	return graph.Entity{ID: id, Name: name, Kind: "SCOPE.Schema", Subtype: "field"}
}

// TestIssue4849_ClassFieldContainsEdgeInPayload is the core diagnostic.
//
// It builds the exact post-#4845 shape — a DTO class Schema CONTAINS a field
// Schema — and asserts the bulk /api/graph payload:
//  1. includes BOTH the class and the field node,
//  2. reports degree≥1 for BOTH (so neither reads as isolated),
//  3. emits the CONTAINS edge between them.
//
// If a future change adds a kind filter, a min-degree/hub prune, or otherwise
// drops Schema↔Schema CONTAINS edges from the bulk payload, this test fails —
// pinning #4849's "is the payload dropping the edges?" question to a guard.
func TestIssue4849_ClassFieldContainsEdgeInPayload(t *testing.T) {
	cls := schemaClassEntity("dto_update_group_device", "UpdateGroupDeviceSettingDto")
	fld := schemaFieldEntity("dto_field_enrolled", "enrolled")

	rels := []graph.Relationship{
		{
			ID:     graph.RelationshipID(cls.ID, fld.ID, "CONTAINS"),
			FromID: cls.ID,
			ToID:   fld.ID,
			Kind:   "CONTAINS",
		},
	}

	grp := makeGraphTestGroup([]graph.Entity{cls, fld}, rels)
	ts := newGraphTestServer(t, grp)
	resp := fetchGraphResponse(t, ts)

	// (1) both nodes present
	clsNode := nodeByID(resp.nodes, cls.ID)
	fldNode := nodeByID(resp.nodes, fld.ID)
	if clsNode == nil {
		t.Fatalf("class Schema node %q missing from payload", cls.ID)
	}
	if fldNode == nil {
		t.Fatalf("field Schema node %q missing from payload", fld.ID)
	}

	// (2) both connected (degree≥1) — neither is isolated
	if deg := nodeDegree(clsNode); deg < 1 {
		t.Errorf("class Schema degree = %d, want ≥1 (would render isolated)", deg)
	}
	if deg := nodeDegree(fldNode); deg < 1 {
		t.Errorf("field Schema degree = %d, want ≥1 (would render isolated)", deg)
	}

	// (3) the CONTAINS edge is present in the payload between the two Schemas
	if !resp.hasEdge("testrepo::"+cls.ID, "testrepo::"+fld.ID, "CONTAINS") {
		t.Errorf("class→field CONTAINS edge dropped from payload: payload bug for #4849")
	}
}

// TestIssue4849_LegitLeafSchemaIsIsolated documents the NOT-a-bug case: a
// Schema with no edges at all (an enum / standalone type-alias / config schema
// with no field children) legitimately reads as isolated. This is expected
// behaviour, not a payload bug — it is what remains in the isolated-Schema
// count after the class→field edges land.
func TestIssue4849_LegitLeafSchemaIsIsolated(t *testing.T) {
	// A standalone enum/type-alias Schema with no relationships at all.
	leaf := graph.Entity{ID: "enum_device_status", Name: "DeviceStatus", Kind: "SCOPE.Schema", Subtype: "enum"}

	grp := makeGraphTestGroup([]graph.Entity{leaf}, nil)
	ts := newGraphTestServer(t, grp)
	resp := fetchGraphResponse(t, ts)

	leafNode := nodeByID(resp.nodes, leaf.ID)
	if leafNode == nil {
		t.Fatalf("leaf Schema node %q missing from payload", leaf.ID)
	}
	if deg := nodeDegree(leafNode); deg != 0 {
		t.Errorf("edge-less leaf Schema degree = %d, want 0 (legitimately isolated)", deg)
	}
	if len(resp.edges) != 0 {
		t.Errorf("expected no edges for an edge-less leaf Schema, got %d", len(resp.edges))
	}
}

// TestIssue4849_ModuleOnlyContainsSchemaIsIsolated reproduces the "by design"
// rendering filter the issue calls out: a field Schema whose ONLY edge is
// CONTAINS from its DIRECTORY/Module parent (hidden by default) reads as
// isolated, because the Module node — and its incident CONTAINS edge — are
// excluded from the default payload. This is the pre-#4845 state and is the
// reason field-membership extraction (class→field CONTAINS) was needed.
func TestIssue4849_ModuleOnlyContainsSchemaIsIsolated(t *testing.T) {
	mod := graph.Entity{ID: "mod_devices", Name: "devices", Kind: "SCOPE." + "Module"}
	fld := schemaFieldEntity("dto_field_orphan", "enrolled")

	rels := []graph.Relationship{
		{
			ID:     graph.RelationshipID(mod.ID, fld.ID, "CONTAINS"),
			FromID: mod.ID,
			ToID:   fld.ID,
			Kind:   "CONTAINS",
		},
	}

	grp := makeGraphTestGroup([]graph.Entity{mod, fld}, rels)
	ts := newGraphTestServer(t, grp)
	resp := fetchGraphResponse(t, ts)

	// Module is hidden by default.
	if nodeByID(resp.nodes, mod.ID) != nil {
		t.Errorf("Module node should be excluded from default payload")
	}
	// The field is present but isolated (its only edge was the hidden module
	// CONTAINS). NOTE: buildDegreeMap counts raw relationships, so degree may be
	// ≥1 here even though the rendered edge is dropped — what matters for the
	// rendered "isolated" look is that no EDGE referencing it survives.
	if nodeByID(resp.nodes, fld.ID) == nil {
		t.Fatalf("field Schema node %q missing from payload", fld.ID)
	}
	if resp.hasEdge("testrepo::"+mod.ID, "testrepo::"+fld.ID, "CONTAINS") {
		t.Errorf("module→field CONTAINS edge must be dropped (module hidden)")
	}
	if len(resp.edges) != 0 {
		t.Errorf("no edges should render when the only edge is module→field; got %d", len(resp.edges))
	}
}
