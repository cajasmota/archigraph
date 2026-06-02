package baseknowledge

import "testing"

// t9_probe_test.go — VALUE-ASSERTING probe for T9 (#3841), written BEFORE the
// packs to fix the exact resolution the MRO walk performs.
//
// The MRO walk (internal/mcp/mro.go resolveMember path (b)) consults the
// process-global Default() registry via reg.Member(baseName, member). A pack
// "resolves" an inherited member when Default().Member(base, member) returns
// ok=true with a contract whose DefiningClass names the framework base — i.e.
// the member is attributed to the framework class even with no indexed body.
//
// We assert against Default() (every registered pack) so the probe proves the
// new packs are wired into the SAME registry the MRO walk reads.

// probeMember mirrors the walk: resolve (base, member) against the default
// registry and return the contract.
func probeMember(t *testing.T, base, member string) Member {
	t.Helper()
	m, ok := Default().Member(base, member)
	if !ok {
		t.Fatalf("Default().Member(%q, %q): not resolved — pack did not attribute the inherited member", base, member)
	}
	return m
}

// TestT9_SpringData_JpaRepository_InheritedSaveFindResolve asserts a
// `interface UserRepo extends JpaRepository<User,Long> {}` resolves
// UserRepo.save / findById through the Spring Data pack to the SPECIFIC
// JpaRepository contract: save -> db_write effect, findById -> db_read, each
// attributed to the framework base (inherited/external provenance), no
// fabricated HTTP status.
func TestT9_SpringData_JpaRepository_InheritedSaveFindResolve(t *testing.T) {
	save := probeMember(t, "JpaRepository", "save")
	if save.DefiningClass == "" {
		t.Error("save: DefiningClass empty — must name the framework base (inherited provenance)")
	}
	if got := dbEffect(save); got != "db_write" {
		t.Errorf("JpaRepository.save effect = %q, want db_write (behaviour %q)", got, save.Behaviour)
	}
	if save.DefaultStatus != StatusUnknown {
		t.Errorf("save: DefaultStatus = %d, want StatusUnknown (data method, no fabricated HTTP status)", save.DefaultStatus)
	}

	find := probeMember(t, "JpaRepository", "findById")
	if got := dbEffect(find); got != "db_read" {
		t.Errorf("JpaRepository.findById effect = %q, want db_read", got)
	}

	// CrudRepository (the superinterface) must resolve the same finders.
	if got := dbEffect(probeMember(t, "CrudRepository", "findAll")); got != "db_read" {
		t.Errorf("CrudRepository.findAll effect = %q, want db_read", got)
	}
	if got := dbEffect(probeMember(t, "CrudRepository", "deleteById")); got != "db_write" {
		t.Errorf("CrudRepository.deleteById effect = %q, want db_write", got)
	}
}

// TestT9_ActiveRecord_UserFindResolves asserts a Rails
// `class User < ApplicationRecord; end` resolves User.find to the ActiveRecord
// pack contract (db_read), attributed to the framework base.
func TestT9_ActiveRecord_UserFindResolves(t *testing.T) {
	find := probeMember(t, "ApplicationRecord", "find")
	if got := dbEffect(find); got != "db_read" {
		t.Errorf("ApplicationRecord.find effect = %q, want db_read", got)
	}
	if find.DefiningClass == "" {
		t.Error("find: DefiningClass empty — must name ActiveRecord::Base")
	}
	if got := dbEffect(probeMember(t, "ApplicationRecord", "save")); got != "db_write" {
		t.Errorf("ApplicationRecord.save effect = %q, want db_write", got)
	}
	// ActiveRecord::Base (the FQN spelling) resolves identically.
	if got := dbEffect(probeMember(t, "ActiveRecord::Base", "update")); got != "db_write" {
		t.Errorf("ActiveRecord::Base.update effect = %q, want db_write", got)
	}
}

// TestT9_Eloquent_OrderFindResolves asserts an Eloquent
// `class Order extends Model {}` resolves Order::find to the Eloquent pack
// contract (db_read) and save/where attributed to the framework Model.
func TestT9_Eloquent_OrderFindResolves(t *testing.T) {
	if got := dbEffect(probeMember(t, "Model", "find")); got != "db_read" {
		t.Errorf("Eloquent Model.find effect = %q, want db_read", got)
	}
	if got := dbEffect(probeMember(t, "Model", "save")); got != "db_write" {
		t.Errorf("Eloquent Model.save effect = %q, want db_write", got)
	}
	if got := dbEffect(probeMember(t, "Model", "where")); got != "db_read" {
		t.Errorf("Eloquent Model.where effect = %q, want db_read", got)
	}
	// FQN spelling resolves identically.
	if _, ok := Default().Member("Illuminate\\Database\\Eloquent\\Model", "find"); !ok {
		t.Error("Eloquent Model FQN lookup of find failed")
	}
}

// TestT9_Negative_UnknownBaseAndUnknownMethod asserts the honest-no-fabrication
// rule: a base that is NOT a known framework class does not resolve, and a
// method not in a known pack does not resolve.
func TestT9_Negative_UnknownBaseAndUnknownMethod(t *testing.T) {
	// Unknown base with no indexed body -> resolved=false (no fabricated pack).
	if _, ok := Default().Member("com.acme.NotARepository", "save"); ok {
		t.Error("unknown base com.acme.NotARepository.save should NOT resolve via the pack")
	}
	if _, ok := Default().Lookup("TotallyMadeUpBase"); ok {
		t.Error("unknown base TotallyMadeUpBase should NOT resolve")
	}
	// Known base, method NOT in the curated set -> unresolved.
	if _, ok := Default().Member("JpaRepository", "frobnicate"); ok {
		t.Error("JpaRepository.frobnicate (not a real member) should NOT resolve")
	}
	if _, ok := Default().Member("Model", "teleport"); ok {
		t.Error("Eloquent Model.teleport (not a real member) should NOT resolve")
	}
}

// dbEffect extracts the curated db effect from a member's Behaviour. The T9
// data packs encode the read/write effect as a "db_read"/"db_write" token at
// the head of Behaviour (no Effect field exists on Member; we never fabricate
// an HTTP status for a data method, so the effect rides in Behaviour).
func dbEffect(m Member) string {
	b := m.Behaviour
	switch {
	case len(b) >= 8 && b[:8] == "db_write":
		return "db_write"
	case len(b) >= 7 && b[:7] == "db_read":
		return "db_read"
	}
	return ""
}
