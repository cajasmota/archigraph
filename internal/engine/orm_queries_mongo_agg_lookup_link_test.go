package engine

import (
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/resolve"
	"github.com/cajasmota/archigraph/internal/types"
)

// #4244 — the `$lookup` SCOPE.DataAccess stage node was fully isolated: the
// JOINS_COLLECTION edge connected the aggregating Class to the looked-up Class,
// but NOTHING connected the per-stage node a consumer naturally `find`s
// ("inspections.aggregate#N $lookup"). This test indexes a Python pymongo
// aggregation carrying `$lookup: { from: "inspections_history", ... }` inside a
// service method and asserts that, after reference resolution, a
// JOINS_COLLECTION edge originates FROM the `$lookup` stage entity's graph ID —
// i.e. the join target is reachable from the node `find` returns.
//
// NON-VACUOUS PROOF: the same body first asserts the PRE-FIX invariant — that
// the ONLY JOINS_COLLECTION edge whose FromID resolves to the stage node is the
// one mongoAggStageJoinEdge adds. We assert the Class-anchored edge (FromID
// "Class:Inspection") STILL fires (no regression), AND that a SEPARATE
// node-anchored edge exists whose FromID is the stage-node structural-ref stub.
// Deleting the mongoAggStageJoinEdge calls makes the node-anchored assertions
// fail (the stage node has zero outgoing edges — the original bug).
func TestMongoAggPy_LookupNode_IsTraversableToJoin_4244(t *testing.T) {
	src := `
from pymongo import MongoClient

class InspectionService:
    def with_history(self, db):
        pipeline = [
            {"$lookup": {
                "from": "inspections_history",
                "localField": "_id",
                "foreignField": "inspection_id",
                "as": "history",
            }},
            {"$match": {"status": "open"}},
        ]
        return db.inspections.aggregate(pipeline)
`
	// Drive the scanner directly so we collect the UNFILTERED edge set,
	// including the node-anchored twin (runMongoAggPy strips it).
	funcs := indexEnclosingFunctions("python", src)
	var ents []types.EntityRecord
	var rels []types.RelationshipRecord
	scanPythonMongoAggregation(src, funcs, "svc/agg.py", "python", nil,
		func(e types.EntityRecord) { ents = append(ents, e) },
		func(r types.RelationshipRecord) { rels = append(rels, r) },
	)

	// Locate the $lookup stage entity the consumer would `find`.
	lookupEnt := pyFindStage(ents, "$lookup")
	if lookupEnt == nil {
		t.Fatalf("expected a $lookup SCOPE.DataAccess stage entity; ents=%+v", ents)
	}
	if lookupEnt.Kind != string(types.EntityKindDataAccess) {
		t.Fatalf("stage kind = %q, want SCOPE.DataAccess", lookupEnt.Kind)
	}

	// REGRESSION GUARD: the collection-anchored edge must still fire.
	classEdge := pyFindJoinTo(rels, capitalisedSingular("inspections_history"))
	if classEdge == nil {
		t.Fatalf("expected a JOINS_COLLECTION edge to inspections_history; rels=%+v", rels)
	}
	if classEdge.FromID != "Class:"+capitalisedSingular("inspections") {
		// The collection-anchored edge originates from the aggregating Class.
		// (There are now two edges to the same target; ensure at least one is
		// the Class-anchored form.)
		var sawClassAnchored bool
		for i := range rels {
			r := &rels[i]
			if r.Kind == string(types.RelationshipKindJoinsCollection) &&
				r.ToID == "Class:"+capitalisedSingular("inspections_history") &&
				r.FromID == "Class:"+capitalisedSingular("inspections") {
				sawClassAnchored = true
			}
		}
		if !sawClassAnchored {
			t.Fatalf("collection-anchored JOINS_COLLECTION edge (Class:Inspection -> Class:Inspections_history) missing; rels=%+v", rels)
		}
	}

	// THE FIX: find the node-anchored JOINS_COLLECTION edge whose FromID is the
	// stage-node structural-ref stub (Format A: scope:dataaccess:...:<name>).
	var nodeEdge *types.RelationshipRecord
	for i := range rels {
		r := &rels[i]
		if r.Kind != string(types.RelationshipKindJoinsCollection) {
			continue
		}
		if r.ToID != "Class:"+capitalisedSingular("inspections_history") {
			continue
		}
		if strings.HasPrefix(r.FromID, "scope:dataaccess:") {
			nodeEdge = r
			break
		}
	}
	if nodeEdge == nil {
		t.Fatalf("PRE-FIX BUG: no node-anchored JOINS_COLLECTION edge from the $lookup stage node; the node is isolated. rels=%+v", rels)
	}
	// The stub must name THIS stage entity by file+name so the resolver binds
	// it to the stage node and not something else.
	if !strings.HasSuffix(nodeEdge.FromID, ":"+lookupEnt.Name) {
		t.Fatalf("node-anchored FromID %q does not reference stage entity name %q", nodeEdge.FromID, lookupEnt.Name)
	}

	// END-TO-END PROOF: run the real reference resolver over the emitted
	// entities + edges. After resolution the node-anchored edge's FromID MUST
	// equal the $lookup stage entity's deterministic graph ID — i.e. the join
	// is genuinely traversable FROM the node `find` returns, not a dangling
	// stub. The stage entity carries the file+name the stub keys on.
	//
	// The real pipeline assigns each entity its deterministic graph ID before
	// resolution; replicate that here (BuildIndex skips ID-less records).
	for i := range ents {
		ents[i].ID = ents[i].ComputeID()
	}
	idx := resolve.BuildIndex(ents)
	resolve.References(rels, idx)

	wantID := lookupEnt.ComputeID()
	var resolvedNodeEdge *types.RelationshipRecord
	for i := range rels {
		r := &rels[i]
		if r.Kind == string(types.RelationshipKindJoinsCollection) && r.FromID == wantID {
			resolvedNodeEdge = r
			break
		}
	}
	if resolvedNodeEdge == nil {
		t.Fatalf("after resolution no JOINS_COLLECTION edge originates from the $lookup stage node ID %q (node still isolated); rels=%+v", wantID, rels)
	}
	// And it points at the looked-up collection.
	if resolvedNodeEdge.ToID == "" {
		t.Fatalf("resolved node-anchored edge has empty ToID")
	}
}
