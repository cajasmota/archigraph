// Package testmap — value-asserting tests for the F# Expecto + xUnit/NUnit
// detector (#4906). Mirrors the Nim std/unittest tests; reuses the shared
// off-side-rule block-body extractor.
//
// HONEST SCOPE: the shared resolver's directCallRE captures PAREN-style call
// sites (`UserService()`, `Account.create()`) and the describe-subject from a
// `testList "Subject"` container. F#'s SPACE-APPLIED application (`createUser
// "ada"`) is the dominant functional call idiom and is NOT paren-captured — it
// is a documented follow-up (see the coverage-doc note + #4906 follow-up). These
// tests assert only the genuinely-working signals.
package testmap

import (
	"testing"
)

// TestFSharpExpecto_SubjectLinkage proves an Expecto `testList "User"` container
// seeds a describe-subject TESTS edge for each leaf `testCase`, attributed to the
// fsharp-expecto framework.
func TestFSharpExpecto_SubjectLinkage(t *testing.T) {
	src := `
module UserTests

open Expecto
open User

let tests =
    testList "User" [
        testCase "creates a user" <| fun _ ->
            let u = makeUser "ada"
            Expect.equal u.name "ada" "name"
    ]
`
	recs := runExtract(t, "tests/UserTests.fs", "fsharp", src)
	if len(recs) == 0 {
		t.Fatalf("expected >=1 testmap entity for fsharp-expecto")
	}
	rec := findByTested(t, recs, "creates_a_user", "User")
	if rec.Properties["test_framework"] != "fsharp-expecto" {
		t.Errorf("framework=%q, want fsharp-expecto", rec.Properties["test_framework"])
	}
	if !hasEdge(recs, "creates_a_user", "User") {
		t.Errorf("missing TESTS edge creates_a_user -> User (testList subject)")
	}
}

// TestFSharpExpecto_ParenCallHighConfidence proves a PAREN-style .NET-interop
// call inside a `testCase` body is captured as a direct (high-confidence) TESTS
// edge — the F# constructor/interop call idiom.
func TestFSharpExpecto_ParenCallHighConfidence(t *testing.T) {
	src := `
module SvcTests

open Expecto
open Svc

let tests =
    testList "Svc" [
        testCase "alpha" <| fun _ ->
            let svc = OrderService()
            let r = svc.PlaceOrder(42)
            Expect.isTrue r "ok"
        testCase "beta" <| fun _ ->
            let q = QueryRunner()
            Expect.isTrue (q.RunBeta()) "b"
    ]
`
	recs := runExtract(t, "tests/SvcTests.fs", "fsharp", src)
	if !hasEdgeAny(recs, "alpha", "OrderService") {
		t.Errorf("expected alpha -> OrderService paren-call edge")
	}
	// Block-body scoping: QueryRunner (from beta) must not leak into alpha.
	if hasEdgeAny(recs, "alpha", "QueryRunner") {
		t.Errorf("QueryRunner leaked into the alpha case body — block-body scoping failed")
	}
	if !hasEdgeAny(recs, "beta", "QueryRunner") {
		t.Errorf("expected beta -> QueryRunner paren-call edge")
	}
}

// TestFSharpXUnit_AttributedLetBinding proves an xUnit [<Fact>]-decorated `let`
// binding is detected as a test case and its paren-call body is scanned.
func TestFSharpXUnit_AttributedLetBinding(t *testing.T) {
	src := `
module AccountTests

open Xunit
open Account

[<Fact>]
let ` + "``creates an account``" + ` () =
    let svc = AccountService()
    let acc = svc.Open("savings")
    Assert.NotNull acc
`
	recs := runExtract(t, "tests/AccountTests.fs", "fsharp", src)
	if len(recs) == 0 {
		t.Fatalf("expected >=1 testmap entity for fsharp xUnit")
	}
	if !hasEdgeAny(recs, "creates_an_account", "AccountService") {
		t.Errorf("expected creates_an_account -> AccountService edge")
	}
}

// TestFSharp_NonTestFileNoop proves a plain production .fs file (no test
// attribute, no Expecto case) is not detected as a test file.
func TestFSharp_NonTestFileNoop(t *testing.T) {
	src := `
module User

let createUser (name: string) : User =
    { name = name }
`
	recs := runExtract(t, "src/User.fs", "fsharp", src)
	if len(recs) != 0 {
		t.Fatalf("expected no testmap entities for a production file, got %d", len(recs))
	}
}

// TestFSharp_TestFilenameButNoCasesNoop proves a test-named .fs file that
// contains no Expecto case and no test attribute yields zero entities (the
// self-confirming detector, like rust_test).
func TestFSharp_TestFilenameButNoCasesNoop(t *testing.T) {
	src := `
module Helpers

let buildFixture () = ()
`
	recs := runExtract(t, "tests/HelpersTests.fs", "fsharp", src)
	if len(recs) != 0 {
		t.Fatalf("expected no entities for a test-named file with no cases, got %d", len(recs))
	}
}
