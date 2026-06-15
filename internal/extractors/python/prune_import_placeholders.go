// prune_import_placeholders.go — Issue #693: eliminate dangling SCOPE.Component
// import-placeholder entities from the Python extractor's emission.
//
// # Problem
//
// Previously, extractImports emitted a separate SCOPE.Component/module entity
// per import_statement / import_from_statement (Name = module path, Subtype =
// "module"). That entity served only as a carrier for the IMPORTS relationship;
// nothing else referenced it because REFERENCES edges from function bodies
// resolve to the real external entity (ext:django:models), not the placeholder.
// Result: the overwhelming majority of SCOPE.Component/module entities had zero
// inbound edges — the dominant orphan class on every Python corpus after #689:
//
//   - django-realworld: 87 dangling module entities (93.4% of orphans)
//   - flask-realworld:  74 dangling module entities (79.0% of orphans)
//   - pandas:         2590 dangling module entities (92.9% of orphans)
//
// # Fix (Option 1 from #693 / mirrors Java #681/#694)
//
// Attach IMPORTS relationships directly to the per-file SCOPE.Component entity
// (subtype="file", Name=file path) that every extractor emits via
// extractor.FileEntity. The resolver's BuildImportTable reads IMPORTS edges
// from r.Relationships on ANY entity and uses rel.Properties (local_name /
// source_module / imported_name / wildcard) to build the per-file binding
// table — it does not care which entity is the carrier.
//
// This eliminates all import-placeholder SCOPE.Component/module entities
// without losing any resolver-visible information. The IMPORTS edge and its
// Properties are preserved; only the redundant SCOPE.Component/module wrapper
// entity is dropped.
//
// # Correctness invariants
//
// 1. The file entity (entities[0]) always exists when Extract is called
//    (it is the first entity appended — extractor.FileEntity).
// 2. The IMPORTS relationship FromID is still the file path — unchanged.
// 3. resolveImportToIDs walks all entities looking for IMPORTS edges; it now
//    checks all entities (not just SCOPE.Component/module) so the rewrite
//    still fires for external packages.
// 4. The resolver's BuildImportTable reads rel.FromID (falling back to
//    r.SourceFile) — both still equal the file path.
// 5. Same-package SCOPE.Component entities for classes (subtype="class") and
//    the file entity (subtype="file") are NOT affected.
//
// # Tests
//
// See prune_import_placeholders_test.go for:
// - External import does NOT produce a SCOPE.Component/module entity.
// - IMPORTS edge + properties are present on the file entity.
// - Real class/function SCOPE.Component entities are unaffected.
// - Existing imports_test.go rewritten helper findImportEdge updated.

package python

import (
	"github.com/cajasmota/grafel/internal/types"
)

// attachImportRelationships moves the IMPORTS relationships from standalone
// SCOPE.Component/module placeholder entities onto fileEntity (the per-file
// SCOPE.Component with subtype="file") and returns only the non-module
// entities (classes, functions, error patterns, etc.) so the caller can
// replace the entity slice without the now-redundant module placeholders.
//
// importEnts is the output of extractImports. On return each entity whose
// Kind=="SCOPE.Component" and Subtype=="module" is dropped and its IMPORTS
// relationship (if any) is appended to fileEntity.Relationships. All other
// entities are passed through unchanged.
//
// fileEntity must be non-nil; it is typically &entities[0] (the file entity
// appended first in Extract).
func attachImportRelationships(importEnts []types.EntityRecord, fileEntity *types.EntityRecord) []types.EntityRecord {
	if fileEntity == nil {
		// Safety: if there is no file entity (shouldn't happen) preserve old
		// behavior by returning importEnts unmodified so no IMPORTS edges
		// are lost.
		return importEnts
	}
	var remaining []types.EntityRecord
	for i := range importEnts {
		e := &importEnts[i]
		if e.Kind == "SCOPE.Component" && e.Subtype == "module" {
			// Transfer the IMPORTS relationship(s) onto the file entity.
			for _, r := range e.Relationships {
				if r.Kind == "IMPORTS" {
					fileEntity.Relationships = append(fileEntity.Relationships, r)
				}
			}
			// Drop the placeholder entity — do not append to remaining.
			continue
		}
		remaining = append(remaining, *e)
	}
	return remaining
}
