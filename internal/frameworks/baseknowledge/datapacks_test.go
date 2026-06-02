package baseknowledge

import "testing"

// datapacks_test.go — value-asserting tests for the T9 (#3841) data-layer
// framework packs (Spring Data, ActiveRecord, Eloquent, TypeORM, NestJS Crud),
// using ISOLATED registries (one pack each) so the assertions are independent
// of process-global registration order — mirroring drf_test.go's reg().

func mustMember(t *testing.T, r *Registry, base, member string) Member {
	t.Helper()
	m, ok := r.Member(base, member)
	if !ok {
		t.Fatalf("%s.%s did not resolve through the pack", base, member)
	}
	return m
}

// ---- Spring Data ----------------------------------------------------------

func TestSpringData_JpaRepositoryEffectiveSurface(t *testing.T) {
	r := NewRegistry(springDataPack{})

	// save -> db_write attributed to CrudRepository (where it is defined).
	save := mustMember(t, r, "JpaRepository", "save")
	if dbEffect(save) != "db_write" {
		t.Errorf("JpaRepository.save effect = %q, want db_write", dbEffect(save))
	}
	if save.DefiningClass != springDataRoot+"CrudRepository" {
		t.Errorf("save DefiningClass = %q, want CrudRepository", save.DefiningClass)
	}
	if save.DefaultStatus != StatusUnknown || save.HTTPVerb != "" {
		t.Errorf("save must carry no HTTP status/verb (data method); got status=%d verb=%q", save.DefaultStatus, save.HTTPVerb)
	}

	// findById -> db_read.
	if dbEffect(mustMember(t, r, "JpaRepository", "findById")) != "db_read" {
		t.Error("JpaRepository.findById should be db_read")
	}
	// JPA-specific method attributed to JpaRepository itself.
	saf := mustMember(t, r, "JpaRepository", "saveAndFlush")
	if dbEffect(saf) != "db_write" || saf.DefiningClass != springJpaRoot+"JpaRepository" {
		t.Errorf("saveAndFlush: effect=%q defining=%q, want db_write / JpaRepository", dbEffect(saf), saf.DefiningClass)
	}
	// getReferenceById -> db_read.
	if dbEffect(mustMember(t, r, "JpaRepository", "getReferenceById")) != "db_read" {
		t.Error("JpaRepository.getReferenceById should be db_read")
	}
}

func TestSpringData_CrudRepositoryIsCrudOnly(t *testing.T) {
	r := NewRegistry(springDataPack{})
	// CrudRepository must NOT carry JpaRepository-only methods (no over-claim).
	if _, ok := r.Member("CrudRepository", "saveAndFlush"); ok {
		t.Error("CrudRepository should not expose saveAndFlush (JpaRepository-only)")
	}
	if dbEffect(mustMember(t, r, "CrudRepository", "deleteById")) != "db_write" {
		t.Error("CrudRepository.deleteById should be db_write")
	}
}

func TestSpringData_FQNLookup(t *testing.T) {
	r := NewRegistry(springDataPack{})
	if _, ok := r.Lookup(springJpaRoot + "JpaRepository"); !ok {
		t.Error("JpaRepository FQN lookup failed")
	}
	if _, ok := r.Member("JpaRepository", "frobnicate"); ok {
		t.Error("non-member frobnicate must not resolve (no fabrication)")
	}
}

// ---- ActiveRecord ---------------------------------------------------------

func TestActiveRecord_FindersAndPersistence(t *testing.T) {
	r := NewRegistry(activeRecordPack{})
	for _, n := range []string{"find", "find_by", "where", "all", "first", "exists?", "count"} {
		if dbEffect(mustMember(t, r, "ApplicationRecord", n)) != "db_read" {
			t.Errorf("ApplicationRecord.%s should be db_read", n)
		}
	}
	for _, n := range []string{"save", "save!", "update", "create", "destroy", "delete", "update_all"} {
		if dbEffect(mustMember(t, r, "ApplicationRecord", n)) != "db_write" {
			t.Errorf("ApplicationRecord.%s should be db_write", n)
		}
	}
	// Both spellings resolve identically and attribute ActiveRecord::Base.
	if m := mustMember(t, r, "ActiveRecord::Base", "find"); m.DefiningClass != activeRecordBase {
		t.Errorf("ActiveRecord::Base.find DefiningClass = %q, want %q", m.DefiningClass, activeRecordBase)
	}
	if _, ok := r.Member("ApplicationRecord", "teleport"); ok {
		t.Error("non-member teleport must not resolve")
	}
}

// ---- Eloquent -------------------------------------------------------------

func TestEloquent_ModelFindersAndPersistence(t *testing.T) {
	r := NewRegistry(eloquentPack{})
	for _, n := range []string{"find", "findOrFail", "first", "all", "where", "get", "count"} {
		if dbEffect(mustMember(t, r, "Model", n)) != "db_read" {
			t.Errorf("Eloquent Model.%s should be db_read", n)
		}
	}
	for _, n := range []string{"save", "update", "create", "delete", "destroy", "updateOrCreate"} {
		if dbEffect(mustMember(t, r, "Model", n)) != "db_write" {
			t.Errorf("Eloquent Model.%s should be db_write", n)
		}
	}
	// FQN + leaf spellings both resolve.
	if _, ok := r.Lookup(eloquentModelFQN); !ok {
		t.Error("Eloquent Model FQN lookup failed")
	}
	if m := mustMember(t, r, "Model", "save"); m.DefiningClass != eloquentModelFQN {
		t.Errorf("Eloquent Model.save DefiningClass = %q, want %q", m.DefiningClass, eloquentModelFQN)
	}
}

// ---- TypeORM / NestJS Crud ------------------------------------------------

func TestTypeORM_RepositoryEffects(t *testing.T) {
	r := NewRegistry(nestjsPack{})
	if dbEffect(mustMember(t, r, "Repository", "findOne")) != "db_read" {
		t.Error("TypeORM Repository.findOne should be db_read")
	}
	for _, n := range []string{"save", "insert", "update", "remove", "delete", "softDelete"} {
		if dbEffect(mustMember(t, r, "Repository", n)) != "db_write" {
			t.Errorf("TypeORM Repository.%s should be db_write", n)
		}
	}
}

func TestNestjsxCrud_ServiceEffects(t *testing.T) {
	r := NewRegistry(nestjsPack{})
	if dbEffect(mustMember(t, r, "TypeOrmCrudService", "getMany")) != "db_read" {
		t.Error("TypeOrmCrudService.getMany should be db_read")
	}
	for _, n := range []string{"createOne", "updateOne", "replaceOne", "deleteOne"} {
		if dbEffect(mustMember(t, r, "TypeOrmCrudService", n)) != "db_write" {
			t.Errorf("TypeOrmCrudService.%s should be db_write", n)
		}
	}
	if _, ok := r.Member("TypeOrmCrudService", "warpDrive"); ok {
		t.Error("non-member warpDrive must not resolve")
	}
}

// TestT9_NoFabricatedStatusAcrossDataPacks asserts the honest rule across every
// data pack: NO data member carries a fabricated HTTP status or verb.
func TestT9_NoFabricatedStatusAcrossDataPacks(t *testing.T) {
	r := NewRegistry(springDataPack{}, activeRecordPack{}, eloquentPack{}, nestjsPack{})
	for _, p := range r.Packs() {
		for _, c := range p.Contracts() {
			for name, m := range c.Members {
				if m.DefaultStatus != StatusUnknown {
					t.Errorf("%s.%s fabricated DefaultStatus=%d (data method must be StatusUnknown)", c.Leaf(), name, m.DefaultStatus)
				}
				if m.HTTPVerb != "" {
					t.Errorf("%s.%s fabricated HTTPVerb=%q (data method is not a route handler)", c.Leaf(), name, m.HTTPVerb)
				}
				if dbEffect(m) == "" {
					t.Errorf("%s.%s missing db_read/db_write effect token in Behaviour", c.Leaf(), name)
				}
			}
		}
	}
}
