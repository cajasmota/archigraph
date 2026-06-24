package haskell_test

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// TestPersistent_EntityBlock covers the canonical persistent schema QuasiQuote:
// two entities with fields → two orm_model SCOPE.Components with MAPS_TO edges.
func TestPersistent_EntityBlock(t *testing.T) {
	src := `{-# LANGUAGE QuasiQuotes #-}
module Model where

import Database.Persist.TH

share [mkPersist sqlSettings, mkMigrate "migrateAll"] [persistLowerCase|
User
    name String
    age  Int Maybe
    deriving Show
BlogPost
    title String
    authorId UserId
|]
`
	ents := runHaskell(t, src, "src/Model.hs")

	user := hsFind(ents, "User", "SCOPE.Component")
	if user == nil || user.Subtype != "orm_model" {
		t.Fatalf("expected orm_model entity 'User'; got %+v", user)
	}
	if user.Properties["table_name"] != "user" {
		t.Errorf("User table_name=%q want 'user'", user.Properties["table_name"])
	}
	if user.Properties["orm"] != "persistent" {
		t.Errorf("User orm=%q want 'persistent'", user.Properties["orm"])
	}
	if f := user.Properties["fields"]; !strings.Contains(f, "name") || !strings.Contains(f, "age") {
		t.Errorf("User fields=%q want name,age", f)
	}
	// MAPS_TO edge to the table.
	hasMaps := false
	for _, r := range user.Relationships {
		if r.Kind == "MAPS_TO" && r.ToID == "user" {
			hasMaps = true
		}
	}
	if !hasMaps {
		t.Errorf("expected User --MAPS_TO--> user edge; got %+v", user.Relationships)
	}

	bp := hsFind(ents, "BlogPost", "SCOPE.Component")
	if bp == nil || bp.Subtype != "orm_model" {
		t.Fatalf("expected orm_model entity 'BlogPost'")
	}
	// snake_case table derivation.
	if bp.Properties["table_name"] != "blog_post" {
		t.Errorf("BlogPost table_name=%q want 'blog_post'", bp.Properties["table_name"])
	}
	// authorId foreign-key field is recorded as a plain field (honest partial).
	if f := bp.Properties["fields"]; !strings.Contains(f, "authorId") {
		t.Errorf("BlogPost fields=%q want authorId", f)
	}
}

// TestPersistent_NoBlockNoEmit is the negative guard: a Haskell file with no
// persistent QuasiQuote emits no orm_model entity.
func TestPersistent_NoBlockNoEmit(t *testing.T) {
	src := `module Plain where
data User = User { name :: String }
`
	ents := runHaskell(t, src, "src/Plain.hs")
	for _, e := range ents {
		if e.Subtype == "orm_model" {
			t.Fatalf("non-persistent file must not emit orm_model; got %q", e.Name)
		}
	}
}

// TestHspec_SpecSuite covers hspec describe/it extraction: a *Spec.hs file with
// examples → one test_suite carrying the example count + a stem-affinity TESTS
// edge to the tested module.
func TestHspec_SpecSuite(t *testing.T) {
	src := `module UserSpec (spec) where

import Test.Hspec

spec :: Spec
spec = do
  describe "User.create" $ do
    it "creates a user" $ do
      True ` + "`shouldBe`" + ` True
    it "rejects a blank name" $ do
      True ` + "`shouldBe`" + ` True
  describe "User.delete" $ do
    specify "removes a user" $ do
      True ` + "`shouldBe`" + ` True
`
	ents := runHaskell(t, src, "test/UserSpec.hs")
	suite := hsFind(ents, "spec_suite:UserSpec", "SCOPE.Operation")
	if suite == nil || suite.Subtype != "test_suite" {
		t.Fatalf("expected test_suite 'spec_suite:UserSpec'; got %+v", suite)
	}
	if suite.Properties["framework"] != "hspec" {
		t.Errorf("framework=%q want hspec", suite.Properties["framework"])
	}
	// 2 it + 1 specify = 3 examples.
	if suite.Properties["example_count"] != "3" {
		t.Errorf("example_count=%q want 3", suite.Properties["example_count"])
	}
	// Stem-affinity TESTS edge to the tested module (UserSpec → User).
	hasTests := false
	for _, r := range suite.Relationships {
		if r.Kind == string(types.RelationshipKindTests) && r.ToID == "User" {
			hasTests = true
		}
	}
	if !hasTests {
		t.Errorf("expected TESTS edge from suite to 'User'; got %+v", suite.Relationships)
	}
}

// TestHspec_ExampleLessNoEmit is the honest guard: a spec module with describe
// blocks but no `it`/`specify` examples exercises nothing → no suite.
func TestHspec_ExampleLessNoEmit(t *testing.T) {
	src := `module EmptySpec where
import Test.Hspec
spec = describe "nothing yet" $ return ()
`
	ents := runHaskell(t, src, "test/EmptySpec.hs")
	for _, e := range ents {
		if e.Subtype == "test_suite" {
			t.Fatalf("example-less spec must not emit a suite; got %q", e.Name)
		}
	}
}
