package javascript_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/resolve"
	"github.com/cajasmota/grafel/internal/types"

	_ "github.com/cajasmota/grafel/internal/custom/javascript"
)

// Issue #4365 LIVE-REPRO.
//
// Generalises the field-membership fix #4328 (TypeORM / class-validator /
// mongoose) to the remaining JS/TS ORMs: Sequelize, Drizzle, Objection,
// MikroORM and Prisma. Before this fix, the model/entity FIELD entities those
// extractors emit (Sequelize columns, Drizzle columns + FK refs, Objection
// relationMappings, MikroORM @Property/@relation, Prisma model fields) were
// standalone graph nodes with NO owning-class CONTAINS edge (orphans) and NO
// REFERENCES edge to their relation/FK target type.
//
// Each test below runs the REAL extractor AND the REAL resolve.BuildIndex
// symbol table over a faithful fixture and asserts:
//
//	(1) field membership — every emitted field/column/relation entity carries a
//	    CONTAINS edge FROM its owning model/entity class node; and
//	(2) relation/FK target — the relation or FK target type emits a REFERENCES
//	    edge to the target Class:<Name> stub, which RESOLVES against the
//	    same-file model node in the symbol table.

func loadRepro4365(t *testing.T, base, repoPath, lang string) extreg.FileInput {
	t.Helper()
	p := filepath.Join("testdata", "issue4365", base)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read repro %s: %v", p, err)
	}
	return extreg.FileInput{Path: repoPath, Language: lang, Content: b}
}

func extract4365(t *testing.T, name string, file extreg.FileInput) []types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get(name)
	if !ok {
		t.Fatalf("extractor %q not registered", name)
	}
	ents, err := e.Extract(context.Background(), file)
	if err != nil {
		t.Fatalf("extract %s: %v", name, err)
	}
	return ents
}

// containsToMember reports whether some entity declares a CONTAINS edge whose
// ToID names the given member entity ID.
func containsToMember(ents []types.EntityRecord, memberID string) bool {
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == string(types.RelationshipKindContains) && r.ToID == memberID {
				return true
			}
		}
	}
	return false
}

func referencesTo(ents []types.EntityRecord, stub string) bool {
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == string(types.RelationshipKindReferences) && r.ToID == stub {
				return true
			}
		}
	}
	return false
}

// assertFieldsAreMembers asserts every entity with a subtype in fieldSubtypes is
// a CONTAINS member, returning the orphan count.
func assertFieldsAreMembers(t *testing.T, ents []types.EntityRecord, fieldSubtypes map[string]bool, orm string) int {
	t.Helper()
	var fields []types.EntityRecord
	for _, e := range ents {
		if fieldSubtypes[e.Subtype] {
			fields = append(fields, e)
		}
	}
	if len(fields) == 0 {
		t.Fatalf("%s: no field/column/relation entities extracted", orm)
	}
	orphans := 0
	for _, fe := range fields {
		if !containsToMember(ents, fe.ID) {
			orphans++
			t.Errorf("%s ORPHAN field (no CONTAINS from owner): subtype=%s name=%q", orm, fe.Subtype, fe.Name)
		}
	}
	t.Logf("%s: orphaned field entities = %d / %d", orm, orphans, len(fields))
	return orphans
}

func dumpEnts(t *testing.T, ents []types.EntityRecord) {
	for _, e := range ents {
		t.Logf("ENTITY kind=%s subtype=%s name=%q rels=%d", e.Kind, e.Subtype, e.Name, len(e.Relationships))
		for _, r := range e.Relationships {
			t.Logf("   EDGE %s from=%s to=%s", r.Kind, r.FromID, r.ToID)
		}
	}
}

func TestIssue4365_Sequelize_FieldsAreMembers(t *testing.T) {
	file := loadRepro4365(t, "sequelize-user.model.js.txt", "src/models/user.model.js", "javascript")
	ents := extract4365(t, "custom_js_sequelize", file)
	dumpEnts(t, ents)

	assertFieldsAreMembers(t, ents, map[string]bool{"column": true, "foreign_key": true}, "sequelize")

	// FK references: Organization model class is REFERENCES'd.
	if !referencesTo(ents, "Class:Organization") {
		t.Errorf("sequelize: expected REFERENCES edge to Class:Organization for FK column")
	}

	// Resolve User model node in the symbol table.
	idx := resolve.BuildIndex(ents)
	if _, ok := idx.Lookup("Class:User"); !ok {
		t.Errorf("sequelize: Class:User stub did not resolve to the User model node")
	}
}

func TestIssue4365_Drizzle_FieldsAreMembers(t *testing.T) {
	file := loadRepro4365(t, "drizzle-schema.ts.txt", "src/db/schema.ts", "typescript")
	ents := extract4365(t, "custom_js_drizzle", file)
	dumpEnts(t, ents)

	assertFieldsAreMembers(t, ents, map[string]bool{"column": true, "foreign_key": true}, "drizzle")

	// .references(() => users.id) → REFERENCES to the users table model node.
	if !referencesTo(ents, "Class:users") {
		t.Errorf("drizzle: expected REFERENCES edge to Class:users for FK .references()")
	}
	idx := resolve.BuildIndex(ents)
	if _, ok := idx.Lookup("Class:users"); !ok {
		t.Errorf("drizzle: Class:users stub did not resolve to the users table node")
	}
}

func TestIssue4365_Objection_RelationsAreMembers(t *testing.T) {
	file := loadRepro4365(t, "objection-person.model.js.txt", "src/models/Person.js", "javascript")
	ents := extract4365(t, "custom_js_objection", file)
	dumpEnts(t, ents)

	assertFieldsAreMembers(t, ents, map[string]bool{"relation": true}, "objection")

	// relationMappings.pets → modelClass: Animal; parent → modelClass: Person.
	if !referencesTo(ents, "Class:Animal") {
		t.Errorf("objection: expected REFERENCES edge to Class:Animal for pets relation modelClass")
	}
	if !referencesTo(ents, "Class:Person") {
		t.Errorf("objection: expected REFERENCES edge to Class:Person (self-relation) modelClass")
	}
	idx := resolve.BuildIndex(ents)
	if _, ok := idx.Lookup("Class:Person"); !ok {
		t.Errorf("objection: Class:Person stub did not resolve to the Person model node")
	}
}

func TestIssue4365_MikroORM_FieldsAreMembers(t *testing.T) {
	file := loadRepro4365(t, "mikroorm-author.entity.ts.txt", "src/entities/author.entity.ts", "typescript")
	ents := extract4365(t, "custom_js_mikroorm", file)
	dumpEnts(t, ents)

	assertFieldsAreMembers(t, ents, map[string]bool{"field": true, "relation": true}, "mikro-orm")

	// @ManyToOne(() => Publisher) and @OneToMany(() => Book) thunk targets.
	if !referencesTo(ents, "Class:Publisher") {
		t.Errorf("mikro-orm: expected REFERENCES edge to Class:Publisher for @ManyToOne thunk")
	}
	if !referencesTo(ents, "Class:Book") {
		t.Errorf("mikro-orm: expected REFERENCES edge to Class:Book for @OneToMany thunk")
	}
}

func TestIssue4365_Prisma_FieldsAreMembers(t *testing.T) {
	file := loadRepro4365(t, "prisma-schema.prisma.txt", "prisma/schema.prisma", "")
	ents := extract4365(t, "custom_js_prisma", file)
	dumpEnts(t, ents)

	assertFieldsAreMembers(t, ents, map[string]bool{"field": true}, "prisma")

	// Relation fields: User.posts → Post, Post.author → User, etc.
	if !referencesTo(ents, "Class:Post") {
		t.Errorf("prisma: expected REFERENCES edge to Class:Post for User.posts relation field")
	}
	if !referencesTo(ents, "Class:User") {
		t.Errorf("prisma: expected REFERENCES edge to Class:User for Post.author relation field")
	}
	idx := resolve.BuildIndex(ents)
	if _, ok := idx.Lookup("Class:Post"); !ok {
		t.Errorf("prisma: Class:Post stub did not resolve to the Post model node")
	}
}
