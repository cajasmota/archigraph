package main

import (
	"testing"
)

// TestClassShadowFold_ServiceKind_Kotlin asserts the #1700 fix for the Kotlin
// Spring case: a class annotated @Service emits SCOPE.Service (Kotlin extractor)
// while the hierarchy pass emits a companion SCOPE.Component/class shadow for
// the same (source_file, name). The fold must absorb the shadow so that
// ChannelRegistry resolves to exactly ONE node of kind SCOPE.Service.
//
// Before #1700 the fix only covered bare "Service" in frameworkClassKindPriority,
// not the "SCOPE.Service" kind that the Kotlin extractor actually emits.
func TestClassShadowFold_ServiceKind_Kotlin(t *testing.T) {
	doc := runIndexerOn(t, "testdata/spring_app", "spring_app", nil)

	byName := map[string][]entitySummary{}
	for _, e := range doc.Entities {
		if e.Name == "" {
			continue
		}
		byName[e.Name] = append(byName[e.Name], entitySummary{
			Kind:    e.Kind,
			Subtype: e.Subtype,
			Line:    e.StartLine,
		})
	}

	// ChannelRegistry is declared @Service in Kotlin. The Kotlin extractor
	// emits SCOPE.Service; the hierarchy extractor additionally emits
	// SCOPE.Component/class for the same symbol. After the fold exactly one
	// class-declaration node must remain (the SCOPE.Service survivor).
	nodes := byName["ChannelRegistry"]
	var decls []entitySummary
	for _, n := range nodes {
		if n.Subtype == "file" || n.Subtype == "import" || n.Subtype == "module" {
			continue
		}
		decls = append(decls, n)
	}
	if len(decls) != 1 {
		t.Errorf("ChannelRegistry: expected 1 class-declaration node after fold, got %d: %v",
			len(decls), decls)
	} else if decls[0].Kind != "SCOPE.Service" {
		t.Errorf("ChannelRegistry: surviving node should be SCOPE.Service, got kind=%s subtype=%s",
			decls[0].Kind, decls[0].Subtype)
	}

	// No INFERRED_FROM_CLASS_HIERARCHY shadow must survive for any entity.
	for _, e := range doc.Entities {
		if e.Properties["provenance"] == "INFERRED_FROM_CLASS_HIERARCHY" {
			t.Errorf("surviving INFERRED_FROM_CLASS_HIERARCHY shadow: %s (%s/%s)",
				e.Name, e.Kind, e.Subtype)
		}
	}
}

// TestClassShadowFold_ServiceKind_NestJS asserts that @Injectable NestJS
// classes (which emit SCOPE.Component/service from the custom extractor and
// SCOPE.Component/class from the AST extractor) collapse to a single node.
func TestClassShadowFold_ServiceKind_NestJS(t *testing.T) {
	doc := runIndexerOn(t, "testdata/nestjs_app", "nestjs_app", nil)

	if len(doc.Entities) == 0 {
		t.Fatal("nestjs_app: no entities extracted")
	}

	byName := map[string][]entitySummary{}
	for _, e := range doc.Entities {
		if e.Name == "" {
			continue
		}
		byName[e.Name] = append(byName[e.Name], entitySummary{
			Kind:    e.Kind,
			Subtype: e.Subtype,
			Line:    e.StartLine,
		})
	}

	for _, name := range []string{"ChannelRegistry", "NotificationService"} {
		nodes, ok := byName[name]
		if !ok {
			t.Errorf("%s: no entity found after indexing", name)
			continue
		}
		var decls []entitySummary
		for _, n := range nodes {
			if n.Subtype == "file" || n.Subtype == "import" || n.Subtype == "module" {
				continue
			}
			decls = append(decls, n)
		}
		if len(decls) != 1 {
			t.Errorf("%s: expected exactly 1 class-declaration node after fold, got %d: %v",
				name, len(decls), decls)
		}
	}

	for _, e := range doc.Entities {
		if e.Properties["provenance"] == "INFERRED_FROM_CLASS_HIERARCHY" {
			t.Errorf("surviving INFERRED_FROM_CLASS_HIERARCHY shadow: %s (%s/%s)",
				e.Name, e.Kind, e.Subtype)
		}
	}
}

// TestClassShadowFold_ServiceKind_NoDanglingEdges asserts the fold does not
// introduce new dangling hex-id edge endpoints in either fixture.
func TestClassShadowFold_ServiceKind_NoDanglingEdges(t *testing.T) {
	for _, tc := range []struct {
		path string
		tag  string
	}{
		{"testdata/spring_app", "spring_app"},
		{"testdata/nestjs_app", "nestjs_app"},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			t.Setenv("ARCHIGRAPH_DISABLE_1613_FOLD", "1")
			unfolded := runIndexerOn(t, tc.path, tc.tag, nil)
			t.Setenv("ARCHIGRAPH_DISABLE_1613_FOLD", "")
			folded := runIndexerOn(t, tc.path, tc.tag, nil)

			if d := danglingHexEndpoints(folded); d > danglingHexEndpoints(unfolded) {
				t.Errorf("fold introduced new dangling hex endpoints: folded=%d unfolded=%d",
					d, danglingHexEndpoints(unfolded))
			}
		})
	}
}

// entitySummary is a compact representation of an entity for test diagnostics.
type entitySummary struct {
	Kind    string
	Subtype string
	Line    int
}
