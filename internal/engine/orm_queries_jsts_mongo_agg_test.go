package engine

import (
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

// runMongoAgg drives scanJSMongoAggregation over `src` and collects the
// emitted stage entities + join edges.
func runMongoAgg(t *testing.T, src string) ([]types.EntityRecord, []types.RelationshipRecord) {
	t.Helper()
	funcs := indexEnclosingFunctions("javascript", src)
	var ents []types.EntityRecord
	var rels []types.RelationshipRecord
	scanJSMongoAggregation(src, funcs, "svc/agg.js", "javascript",
		func(e types.EntityRecord) { ents = append(ents, e) },
		func(r types.RelationshipRecord) { rels = append(rels, r) },
	)
	return ents, rels
}

// stageSubtypesInOrder returns the Subtype of each stage entity in emission
// order — used to assert the pipeline order is preserved.
func stageSubtypesInOrder(ents []types.EntityRecord) []string {
	var out []string
	for _, e := range ents {
		out = append(out, e.Subtype)
	}
	return out
}

func findStage(ents []types.EntityRecord, subtype string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Subtype == subtype {
			return &ents[i]
		}
	}
	return nil
}

func findJoinTo(rels []types.RelationshipRecord, toClass string) *types.RelationshipRecord {
	for i := range rels {
		if rels[i].Kind == string(types.RelationshipKindJoinsCollection) &&
			rels[i].ToID == "Class:"+toClass {
			return &rels[i]
		}
	}
	return nil
}

// Mongoose model, multi-line pipeline with $match, $lookup (classic form),
// $unwind, $group, $sort, $limit. Asserts the $lookup `from`, the stage
// subtypes + order, the $group._id and accumulators.
func TestMongoAgg_Mongoose_LookupGroupOrder(t *testing.T) {
	src := `
const mongoose = require('mongoose');
async function report() {
  return Order.aggregate([
    { $match: { status: 'paid' } },
    { $lookup: {
        from: 'customers',
        localField: 'customerId',
        foreignField: '_id',
        as: 'customer'
    } },
    { $unwind: '$customer' },
    { $group: {
        _id: '$customer.region',
        total: { $sum: '$amount' },
        count: { $sum: 1 }
    } },
    { $sort: { total: -1 } },
    { $limit: 10 },
  ]);
}
`
	ents, rels := runMongoAgg(t, src)

	gotOrder := stageSubtypesInOrder(ents)
	wantOrder := []string{"$match", "$lookup", "$unwind", "$group", "$sort", "$limit"}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("stage order = %v, want %v", gotOrder, wantOrder)
	}

	// stage_index must match position.
	for i, e := range ents {
		if e.Properties["stage_index"] != itoa(i) {
			t.Errorf("stage %d (%s) stage_index = %q, want %d",
				i, e.Subtype, e.Properties["stage_index"], i)
		}
		if e.Properties["collection"] != "Order" {
			t.Errorf("stage %d collection = %q, want Order", i, e.Properties["collection"])
		}
	}

	// $lookup stage carries the from/local/foreign/as props.
	lk := findStage(ents, "$lookup")
	if lk == nil {
		t.Fatal("no $lookup stage entity emitted")
	}
	if lk.Properties["from"] != "customers" {
		t.Errorf("$lookup from = %q, want customers", lk.Properties["from"])
	}
	if lk.Properties["local_field"] != "customerId" {
		t.Errorf("$lookup local_field = %q, want customerId", lk.Properties["local_field"])
	}
	if lk.Properties["foreign_field"] != "_id" {
		t.Errorf("$lookup foreign_field = %q, want _id", lk.Properties["foreign_field"])
	}
	if lk.Properties["as"] != "customer" {
		t.Errorf("$lookup as = %q, want customer", lk.Properties["as"])
	}

	// $group._id + accumulator names.
	grp := findStage(ents, "$group")
	if grp == nil {
		t.Fatal("no $group stage entity emitted")
	}
	if grp.Properties["group_id"] != "'$customer.region'" {
		t.Errorf("$group group_id = %q, want '$customer.region'", grp.Properties["group_id"])
	}
	if grp.Properties["accumulators"] != "total,count" {
		t.Errorf("$group accumulators = %q, want total,count", grp.Properties["accumulators"])
	}

	// JOIN edge: Order -> Customer (singularised), with field props.
	join := findJoinTo(rels, "Customer")
	if join == nil {
		t.Fatalf("no JOINS_COLLECTION edge to Class:Customer; rels=%+v", rels)
	}
	if join.FromID != "Class:Order" {
		t.Errorf("join FromID = %q, want Class:Order", join.FromID)
	}
	if join.Properties["stage"] != "lookup" {
		t.Errorf("join stage = %q, want lookup", join.Properties["stage"])
	}
	if join.Properties["local_field"] != "customerId" ||
		join.Properties["foreign_field"] != "_id" ||
		join.Properties["as"] != "customer" {
		t.Errorf("join field props wrong: %+v", join.Properties)
	}
}

// Raw native driver, db.collection('orders').aggregate, with a $lookup that
// uses the sub-pipeline form (from + pipeline + as, no local/foreignField)
// and a $facet stage. Asserts the join `from`, the facet sub-pipeline names,
// and that the receiver collection resolves from .collection('orders').
func TestMongoAgg_RawDriver_LookupPipelineAndFacet(t *testing.T) {
	src := `
const { MongoClient } = require('mongodb');
async function run(db) {
  return db.collection('orders').aggregate([
    { $match: { active: true } },
    { $lookup: {
        from: 'products',
        as: 'items',
        pipeline: [
          { $match: { inStock: true } },
          { $project: { name: 1, price: 1 } }
        ]
    } },
    { $facet: {
        byStatus: [ { $group: { _id: '$status', n: { $sum: 1 } } } ],
        byMonth: [ { $group: { _id: '$month', n: { $sum: 1 } } } ]
    } },
  ]);
}
`
	ents, rels := runMongoAgg(t, src)

	gotOrder := stageSubtypesInOrder(ents)
	wantOrder := []string{"$match", "$lookup", "$facet"}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("stage order = %v, want %v (nested pipeline stages must NOT leak as top-level)", gotOrder, wantOrder)
	}

	for _, e := range ents {
		if e.Properties["collection"] != "orders" {
			t.Errorf("stage %s collection = %q, want orders", e.Subtype, e.Properties["collection"])
		}
	}

	lk := findStage(ents, "$lookup")
	if lk == nil {
		t.Fatal("no $lookup stage entity emitted")
	}
	if lk.Properties["from"] != "products" {
		t.Errorf("$lookup from = %q, want products", lk.Properties["from"])
	}
	if lk.Properties["as"] != "items" {
		t.Errorf("$lookup as = %q, want items", lk.Properties["as"])
	}

	// $facet sub-pipeline names.
	fc := findStage(ents, "$facet")
	if fc == nil {
		t.Fatal("no $facet stage entity emitted")
	}
	if fc.Properties["facets"] != "byStatus,byMonth" {
		t.Errorf("$facet facets = %q, want byStatus,byMonth", fc.Properties["facets"])
	}

	// JOIN edge orders -> Product (singularised). 'orders' aggregating coll
	// singularises to Order on the FromID side.
	join := findJoinTo(rels, "Product")
	if join == nil {
		t.Fatalf("no JOINS_COLLECTION edge to Class:Product; rels=%+v", rels)
	}
	if join.FromID != "Class:Order" {
		t.Errorf("join FromID = %q, want Class:Order", join.FromID)
	}
	if join.Properties["as"] != "items" {
		t.Errorf("join as = %q, want items", join.Properties["as"])
	}
}

// $graphLookup also emits a cross-collection join edge.
func TestMongoAgg_GraphLookup_EmitsJoin(t *testing.T) {
	src := `
const mongoose = require('mongoose');
function tree() {
  return Employee.aggregate([
    { $match: { active: true } },
    { $graphLookup: {
        from: 'employees',
        startWith: '$reportsTo',
        connectFromField: 'reportsTo',
        connectToField: '_id',
        as: 'hierarchy'
    } },
  ]);
}
`
	ents, rels := runMongoAgg(t, src)

	gl := findStage(ents, "$graphLookup")
	if gl == nil {
		t.Fatal("no $graphLookup stage entity emitted")
	}
	if gl.Properties["from"] != "employees" {
		t.Errorf("$graphLookup from = %q, want employees", gl.Properties["from"])
	}

	join := findJoinTo(rels, "Employee")
	if join == nil {
		t.Fatalf("no JOINS_COLLECTION edge to Class:Employee; rels=%+v", rels)
	}
	if join.Properties["stage"] != "graphLookup" {
		t.Errorf("join stage = %q, want graphLookup", join.Properties["stage"])
	}
	if join.Properties["as"] != "hierarchy" {
		t.Errorf("join as = %q, want hierarchy", join.Properties["as"])
	}
}

// HONEST LIMIT: a dynamically-built pipeline (variable, not inline literal)
// must NOT produce fabricated stages or joins.
func TestMongoAgg_DynamicPipeline_Unresolved(t *testing.T) {
	src := `
const mongoose = require('mongoose');
function dyn(stages) {
  const pipeline = stages;
  return Order.aggregate(pipeline);
}
`
	ents, rels := runMongoAgg(t, src)
	if len(ents) != 0 {
		t.Errorf("dynamic pipeline produced %d stage entities, want 0: %+v", len(ents), ents)
	}
	if len(rels) != 0 {
		t.Errorf("dynamic pipeline produced %d join edges, want 0: %+v", len(rels), rels)
	}
}

// Guard: nested commas/objects/strings inside a stage must not split it.
func TestMongoAgg_NestedAndStringCommasDoNotSplit(t *testing.T) {
	src := `
const mongoose = require('mongoose');
function f() {
  return Sale.aggregate([
    { $match: { $or: [ { a: 1 }, { b: 2 } ], note: 'a, b, c' } },
    { $project: { label: { $concat: ['x', ',', 'y'] } } },
  ]);
}
`
	ents, _ := runMongoAgg(t, src)
	got := stageSubtypesInOrder(ents)
	want := []string{"$match", "$project"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("stage order = %v, want %v (nested/string commas leaked)", got, want)
	}
}
