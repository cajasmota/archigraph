package links

import (
	"path/filepath"
	"testing"
)

// sameAsLinks returns every SAME_AS (method=same_as) link in the group's
// links.json, keyed by ordered "source|target".
func sameAsLinks(t *testing.T, home, group string) map[string]Link {
	t.Helper()
	doc, err := readDoc(filepath.Join(home, "groups", group+"-links.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Link{}
	for _, l := range doc.Links {
		if l.Method == MethodSameAs {
			out[l.Source+"|"+l.Target] = l
		}
	}
	return out
}

// TestSameAsPass_SharedDomainModelLinks (positive) — the same domain model
// defined in two shared-lib repos (Python class + TypeScript interface)
// with overlapping field names must be linked with a single SAME_AS edge.
func TestSameAsPass_SharedDomainModelLinks(t *testing.T) {
	root := fixtureRoot(t)

	// Python shared lib: Order class with field children via CONTAINS.
	writeFixture(t, root, fixtureGraph{
		Repo: "py-shared",
		Entities: []map[string]any{
			{"id": "o", "name": "Order", "kind": "SCOPE.Component", "subtype": "class", "source_file": "py_shared/models.py"},
			{"id": "f1", "name": "Order.id", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "py_shared/models.py"},
			{"id": "f2", "name": "Order.user_id", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "py_shared/models.py"},
			{"id": "f3", "name": "Order.total_cents", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "py_shared/models.py"},
		},
		Edges: []map[string]string{
			{"from_id": "o", "to_id": "f1", "kind": "CONTAINS"},
			{"from_id": "o", "to_id": "f2", "kind": "CONTAINS"},
			{"from_id": "o", "to_id": "f3", "kind": "CONTAINS"},
		},
	})

	// JS shared lib: Order interface with comma-separated `fields` property.
	// camelCase variants must normalize to the snake_case Python ones.
	writeFixture(t, root, fixtureGraph{
		Repo: "js-shared",
		Entities: []map[string]any{
			{
				"id":          "o",
				"name":        "Order",
				"kind":        "SCOPE.Schema",
				"subtype":     "interface",
				"source_file": "src/types.ts",
				"properties":  map[string]any{"fields": "id, userId, totalCents"},
			},
		},
	})

	home := filepath.Join(root, "ag-home-sa-pos")
	if _, err := RunAllPasses("sapos", root, home); err != nil {
		t.Fatal(err)
	}

	links := sameAsLinks(t, home, "sapos")
	if len(links) != 1 {
		t.Fatalf("expected exactly 1 SAME_AS link, got %d: %+v", len(links), links)
	}
	for _, l := range links {
		if l.Relation != RelationSameAs {
			t.Errorf("relation = %q, want %q", l.Relation, RelationSameAs)
		}
		if l.Identifier == nil || *l.Identifier != "order" {
			t.Errorf("identifier = %v, want \"order\"", l.Identifier)
		}
		// Perfect field overlap → top of the band.
		if l.Confidence < sameAsBandHigh-1e-9 {
			t.Errorf("confidence = %v, want ~%v (full overlap)", l.Confidence, sameAsBandHigh)
		}
		// Endpoints must be ordered and cross-repo.
		if l.Source >= l.Target {
			t.Errorf("endpoints not ordered: %s !< %s", l.Source, l.Target)
		}
	}
}

// TestSameAsPass_UnrelatedConfigNotMerged (negative) — two same-named
// `Config` types in two NON-shared service repos must NOT be linked, even
// though they share a name. The shared-lib gate alone excludes them.
func TestSameAsPass_UnrelatedConfigNotMerged(t *testing.T) {
	root := fixtureRoot(t)

	writeFixture(t, root, fixtureGraph{
		Repo: "orders", // a service, not a shared lib
		Entities: []map[string]any{
			{"id": "c", "name": "Config", "kind": "SCOPE.Component", "subtype": "class", "source_file": "orders/config.py"},
			{"id": "f1", "name": "Config.db_url", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "orders/config.py"},
			{"id": "f2", "name": "Config.timeout", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "orders/config.py"},
		},
		Edges: []map[string]string{
			{"from_id": "c", "to_id": "f1", "kind": "CONTAINS"},
			{"from_id": "c", "to_id": "f2", "kind": "CONTAINS"},
		},
	})

	writeFixture(t, root, fixtureGraph{
		Repo: "billing", // another service, identical type name
		Entities: []map[string]any{
			{"id": "c", "name": "Config", "kind": "SCOPE.Component", "subtype": "class", "source_file": "billing/config.go"},
			{"id": "f1", "name": "Config.db_url", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "billing/config.go"},
			{"id": "f2", "name": "Config.timeout", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "billing/config.go"},
		},
		Edges: []map[string]string{
			{"from_id": "c", "to_id": "f1", "kind": "CONTAINS"},
			{"from_id": "c", "to_id": "f2", "kind": "CONTAINS"},
		},
	})

	home := filepath.Join(root, "ag-home-sa-neg1")
	if _, err := RunAllPasses("saneg1", root, home); err != nil {
		t.Fatal(err)
	}

	if links := sameAsLinks(t, home, "saneg1"); len(links) != 0 {
		t.Fatalf("expected 0 SAME_AS links for unrelated service Configs, got %d: %+v", len(links), links)
	}
}

// TestSameAsPass_SharedButStructurallyDisjointNotMerged (negative) — two
// same-named types living in shared-lib repos but with NO overlapping
// field names must NOT be merged: passing the location + name gates is not
// enough; the structural-overlap gate must fail them.
func TestSameAsPass_SharedButStructurallyDisjointNotMerged(t *testing.T) {
	root := fixtureRoot(t)

	writeFixture(t, root, fixtureGraph{
		Repo: "py-shared",
		Entities: []map[string]any{
			{"id": "s", "name": "Settings", "kind": "SCOPE.Component", "subtype": "class", "source_file": "py_shared/settings.py"},
			{"id": "f1", "name": "Settings.alpha", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "py_shared/settings.py"},
			{"id": "f2", "name": "Settings.beta", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "py_shared/settings.py"},
		},
		Edges: []map[string]string{
			{"from_id": "s", "to_id": "f1", "kind": "CONTAINS"},
			{"from_id": "s", "to_id": "f2", "kind": "CONTAINS"},
		},
	})

	writeFixture(t, root, fixtureGraph{
		Repo: "js-shared",
		Entities: []map[string]any{
			{
				"id":          "s",
				"name":        "Settings",
				"kind":        "SCOPE.Schema",
				"subtype":     "interface",
				"source_file": "src/settings.ts",
				"properties":  map[string]any{"fields": "gamma, delta, epsilon"},
			},
		},
	})

	home := filepath.Join(root, "ag-home-sa-neg2")
	if _, err := RunAllPasses("saneg2", root, home); err != nil {
		t.Fatal(err)
	}

	if links := sameAsLinks(t, home, "saneg2"); len(links) != 0 {
		t.Fatalf("expected 0 SAME_AS links for structurally-disjoint shared types, got %d: %+v", len(links), links)
	}
}

// TestSameAsPass_FieldEntitiesNeverLinked (negative) — leaf field entities
// (Order.id in two repos) must never themselves be linked as models.
func TestSameAsPass_FieldEntitiesNeverLinked(t *testing.T) {
	root := fixtureRoot(t)

	for _, repo := range []string{"py-shared", "js-shared"} {
		writeFixture(t, root, fixtureGraph{
			Repo: repo,
			Entities: []map[string]any{
				{"id": "f", "name": "Order.id", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "models"},
			},
		})
	}

	home := filepath.Join(root, "ag-home-sa-neg3")
	if _, err := RunAllPasses("saneg3", root, home); err != nil {
		t.Fatal(err)
	}
	if links := sameAsLinks(t, home, "saneg3"); len(links) != 0 {
		t.Fatalf("expected 0 SAME_AS links for bare field entities, got %d: %+v", len(links), links)
	}
}

// TestSameAsPass_Idempotent — re-running the pass yields a stable set of
// links (method-segregated overwrite, deterministic ids).
func TestSameAsPass_Idempotent(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, fixtureGraph{
		Repo: "py-shared",
		Entities: []map[string]any{
			{"id": "o", "name": "Money", "kind": "SCOPE.Component", "subtype": "class", "source_file": "m.py"},
			{"id": "f1", "name": "Money.amount", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "m.py"},
			{"id": "f2", "name": "Money.currency", "kind": "SCOPE.Schema", "subtype": "field", "source_file": "m.py"},
		},
		Edges: []map[string]string{
			{"from_id": "o", "to_id": "f1", "kind": "CONTAINS"},
			{"from_id": "o", "to_id": "f2", "kind": "CONTAINS"},
		},
	})
	writeFixture(t, root, fixtureGraph{
		Repo: "contracts",
		Entities: []map[string]any{
			{
				"id": "o", "name": "Money", "kind": "SCOPE.Schema", "subtype": "interface",
				"source_file": "money.ts", "properties": map[string]any{"fields": "amount, currency"},
			},
		},
	})

	home := filepath.Join(root, "ag-home-sa-idem")
	if _, err := RunAllPasses("saidem", root, home); err != nil {
		t.Fatal(err)
	}
	first := sameAsLinks(t, home, "saidem")
	if len(first) != 1 {
		t.Fatalf("first run: expected 1 link, got %d", len(first))
	}
	if _, err := RunAllPasses("saidem", root, home); err != nil {
		t.Fatal(err)
	}
	second := sameAsLinks(t, home, "saidem")
	if len(second) != 1 {
		t.Fatalf("second run: expected 1 link, got %d", len(second))
	}
	for k, l := range first {
		if l2, ok := second[k]; !ok || l2.ID != l.ID {
			t.Errorf("link %s not stable across runs", k)
		}
	}
}
