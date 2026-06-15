package resolve

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// #4554 — same-file bare↔qualified method reconciliation for the
// `scope:operation:method:<lang>:<file>:<bare>` synthesis-time endpoint→handler
// bridge. NestJS (and every other producer synthesizer) stamps the bridge with
// the BARE method name, while the real controller method is indexed QUALIFIED
// (`Controller.method`). Without reconciliation the bridge's structural ref is
// left unresolved, surfacing as a phantom grey `scope.operation` handler node
// (no source file) that duplicates the real method — the endpoint shows TWO
// handlers. After the fix the bare structural ref binds to the qualified method
// so there is exactly ONE handler.
func TestReferences_OperationBareQualified_4554(t *testing.T) {
	const file = "src/modules/buildings/api/building.controller.ts"
	entities := []types.EntityRecord{
		entAt("aaaaaaaaaaaaaaaa", "SCOPE.Operation", "BuildingController.updateBuilding", file),
	}
	idx := BuildIndex(entities)

	// The bare-name synthesis-time bridge structural ref.
	rels := []types.RelationshipRecord{{
		FromID: "scope:operation:method:typescript:" + file + ":updateBuilding",
		ToID:   "http_endpoint_definition:http:PUT:/buildings/{id}",
		Kind:   "IMPLEMENTS",
	}}
	References(rels, idx)

	if rels[0].FromID != "aaaaaaaaaaaaaaaa" {
		t.Fatalf("bare structural ref did not bind to the qualified method "+
			"(phantom scope.operation handler duplicate): FromID=%s", rels[0].FromID)
	}
}

// Genuinely-unresolved handlers must still NOT bind (no false positive): a bare
// method name with no matching same-file method leaves the stub intact so the
// fallback paths / honest unresolved disposition still apply.
func TestReferences_OperationBareQualified_4554_Unresolved(t *testing.T) {
	const file = "src/modules/buildings/api/building.controller.ts"
	entities := []types.EntityRecord{
		entAt("aaaaaaaaaaaaaaaa", "SCOPE.Operation", "BuildingController.updateBuilding", file),
	}
	idx := BuildIndex(entities)

	rels := []types.RelationshipRecord{{
		FromID: "scope:operation:method:typescript:" + file + ":noSuchMethod",
		ToID:   "http_endpoint_definition:http:GET:/buildings",
		Kind:   "IMPLEMENTS",
	}}
	References(rels, idx)

	if rels[0].FromID == "aaaaaaaaaaaaaaaa" {
		t.Fatalf("unrelated bare name must not bind to a same-file method")
	}
}

// Ambiguous: two same-file methods share the bare name → no guess (stub kept).
func TestReferences_OperationBareQualified_4554_Ambiguous(t *testing.T) {
	const file = "src/modules/buildings/api/building.controller.ts"
	entities := []types.EntityRecord{
		entAt("aaaaaaaaaaaaaaaa", "SCOPE.Operation", "BuildingController.handle", file),
		entAt("bbbbbbbbbbbbbbbb", "SCOPE.Operation", "AuditController.handle", file),
	}
	idx := BuildIndex(entities)

	from := "scope:operation:method:typescript:" + file + ":handle"
	rels := []types.RelationshipRecord{{
		FromID: from,
		ToID:   "http_endpoint_definition:http:POST:/buildings/handle",
		Kind:   "IMPLEMENTS",
	}}
	References(rels, idx)

	if rels[0].FromID == "aaaaaaaaaaaaaaaa" || rels[0].FromID == "bbbbbbbbbbbbbbbb" {
		t.Fatalf("ambiguous bare name must not bind to either candidate: %s", rels[0].FromID)
	}
}
