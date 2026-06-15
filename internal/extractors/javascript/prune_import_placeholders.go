// prune_import_placeholders.go — Issue #742: eliminate dangling SCOPE.Component
// import-placeholder entities from the JS/TS extractor's emission.
//
// # Problem
//
// The emitImport function previously emitted a separate SCOPE.Component/import
// entity per import_statement (Name = module path, Subtype = "import"). That
// entity served only as a carrier for the IMPORTS relationship; nothing else
// referenced it because REFERENCES edges from function bodies resolve to the
// real external entity (e.g. ext:react:useState), not the placeholder.
// Result: the overwhelming majority of SCOPE.Component/import entities had
// zero inbound edges — the dominant orphan class on every JS/TS corpus:
//
//   - fixture-b: 288 dangling import entities
//   - fixture-c: 103 dangling import entities
//   - fixture-e:  31 dangling import entities
//
// # Fix (Option 1 from #742 / mirrors Java #681/#694 and Python #693/#715)
//
// Attach IMPORTS relationships directly to the per-file SCOPE.Component entity
// (subtype="file", Name=file path) that every extractor emits via fileEntity
// at the top of Extract. The resolver's BuildImportTable reads IMPORTS edges
// from ANY entity's Relationships — it does not require the carrier to be a
// SCOPE.Component/import entity.
//
// This eliminates all import-placeholder SCOPE.Component/import entities
// without losing any resolver-visible information. The IMPORTS edge and its
// Properties are preserved; only the redundant SCOPE.Component/import wrapper
// entity is dropped.
//
// # Correctness invariants
//
// 1. The file entity (x.entities[0]) always exists when collectImports runs
//    (it is the first entity appended — the fileEntity at the top of Extract).
// 2. The IMPORTS relationship FromID is still the file path — unchanged.
// 3. The resolver's BuildImportTable reads rel.FromID (falling back to
//    r.SourceFile) — both still equal the file path.
// 4. The cross-file import linker (#566) consults a per-entity-id → repo
//    index; the file entity is already stamped by the indexer (issue #570),
//    so the rewrite still fires for the IMPORTS edges on the file entity.
// 5. Existing SCOPE.Component entities for classes (subtype="class") and
//    the file entity (subtype="file") are NOT affected.
// 6. The importByLocal binding table used by the receiver binder is built
//    in collectFileImports (before walk), so it is NOT affected by dropping
//    the import entity post-walk.
//
// # Tests
//
// See prune_import_placeholders_test.go for:
// - External npm import does NOT produce a SCOPE.Component/import entity.
// - Relative import does NOT produce a SCOPE.Component/import entity.
// - IMPORTS edge + properties are present on the file entity.
// - Real class/function SCOPE.Component entities are unaffected.
// - CALLS and CONTAINS edges from existing test corpus still pass.

package javascript

import (
	"github.com/cajasmota/grafel/internal/types"
)

// attachImportRelationshipsJS moves the IMPORTS relationships from standalone
// SCOPE.Component/import placeholder entities onto fileEntity (the per-file
// SCOPE.Component with subtype="file") and returns only the non-import
// entities (classes, functions, const_call, etc.) so the caller can
// replace the entity slice without the now-redundant import placeholders.
//
// importEnts is the portion of x.entities emitted by collectImports/emitImport.
// On return each entity whose Kind=="SCOPE.Component" and Subtype=="import" is
// dropped and its IMPORTS relationships are appended to fileEntity.Relationships.
// All other entities are passed through unchanged.
//
// fileEntity must be non-nil; it is typically &x.entities[0] (the file entity
// appended first in Extract).
func attachImportRelationshipsJS(importEnts []types.EntityRecord, fileEntity *types.EntityRecord) []types.EntityRecord {
	if fileEntity == nil {
		// Safety: if there is no file entity (shouldn't happen), preserve
		// old behaviour by returning importEnts unmodified so no IMPORTS
		// edges are lost.
		return importEnts
	}
	var remaining []types.EntityRecord
	for i := range importEnts {
		e := &importEnts[i]
		if e.Kind == "SCOPE.Component" && e.Subtype == "import" {
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
