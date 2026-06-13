package patterns

import (
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

// #5114 — F# property-test (FsCheck / Hedgehog) and assertion-library
// (Unquote / FsUnit) records, the non-db tail of #4941.

// --- property-test records (FsCheck / Hedgehog) -----------------------------

func TestPropertyTest_FSharpFsCheck_AttributeAndDriver(t *testing.T) {
	d := &propertyTestDetector{}
	src := `module RevTests
open FsCheck.Xunit

[<Property>]
let reverseOfReverseIsOriginal (xs: int list) =
    List.rev (List.rev xs) = xs

let quickRun () = Check.Quick prop
`
	if !d.AppliesTo(src) {
		t.Fatal("AppliesTo should fire on FsCheck source")
	}
	results := d.Detect("RevTests.fs", "fsharp", src)
	if !hasLibrary(results, "property_test", "fscheck") {
		t.Fatalf("expected fscheck property_test record, got %v", libsOf(results))
	}
}

func TestPropertyTest_FSharpHedgehog(t *testing.T) {
	d := &propertyTestDetector{}
	src := `module RevTests
open Hedgehog

let revProp =
    property {
        let! xs = Gen.list (Range.linear 0 100) Gen.alpha
        return List.rev (List.rev xs) = xs
    }
`
	results := d.Detect("RevTests.fs", "fsharp", src)
	if !hasLibrary(results, "property_test", "hedgehog") {
		t.Fatalf("expected hedgehog property_test record, got %v", libsOf(results))
	}
}

// Wrong-language no-op: the F# detector must NOT fire when language != fsharp,
// even if the F#-shaped token is present.
func TestPropertyTest_FSharp_WrongLanguage_NoOp(t *testing.T) {
	d := &propertyTestDetector{}
	src := "[<Property>]\nlet x = Check.Quick prop\n"
	results := d.Detect("thing.cs", "csharp", src)
	if hasLibrary(results, "property_test", "fscheck") {
		t.Fatal("fscheck record must not be emitted for non-fsharp language")
	}
}

// No-match no-op: a plain F# file with no property-test marker emits nothing.
func TestPropertyTest_FSharp_NoMatch_NoOp(t *testing.T) {
	d := &propertyTestDetector{}
	src := "module M\nlet add a b = a + b\n"
	results := d.Detect("M.fs", "fsharp", src)
	if hasLibrary(results, "property_test", "fscheck") || hasLibrary(results, "property_test", "hedgehog") {
		t.Fatalf("expected no property_test record, got %v", libsOf(results))
	}
}

// --- assertion-library records (Unquote / FsUnit) ---------------------------

func TestAssertionLib_FSharpUnquote(t *testing.T) {
	d := &assertionLibDetector{}
	src := `module MathTests
open Swensen.Unquote

[<Fact>]
let addWorks () =
    test <@ add 2 2 = 4 @>
`
	if !d.AppliesTo(src) {
		t.Fatal("AppliesTo should fire on Unquote source")
	}
	results := d.Detect("MathTests.fs", "fsharp", src)
	if !hasLibrary(results, "assertion_lib", "unquote") {
		t.Fatalf("expected unquote assertion_lib record, got %v", libsOf(results))
	}
}

func TestAssertionLib_FSharpFsUnit(t *testing.T) {
	d := &assertionLibDetector{}
	src := `module MathTests
open FsUnit.Xunit

[<Fact>]
let addWorks () =
    add 2 2 |> should equal 4
`
	results := d.Detect("MathTests.fs", "fsharp", src)
	if !hasLibrary(results, "assertion_lib", "fsunit") {
		t.Fatalf("expected fsunit assertion_lib record, got %v", libsOf(results))
	}
}

// Wrong-language no-op for assertion records.
func TestAssertionLib_FSharp_WrongLanguage_NoOp(t *testing.T) {
	d := &assertionLibDetector{}
	src := "x |> should equal 4\ntest <@ y = 1 @>\n"
	results := d.Detect("thing.scala", "scala", src)
	if len(results) != 0 {
		t.Fatalf("assertion_lib records must not be emitted for non-fsharp language, got %v", libsOf(results))
	}
}

// No-match no-op for assertion records.
func TestAssertionLib_FSharp_NoMatch_NoOp(t *testing.T) {
	d := &assertionLibDetector{}
	src := "module M\nlet add a b = a + b\n"
	if d.AppliesTo(src) {
		t.Fatal("AppliesTo should not fire on a plain F# file")
	}
	results := d.Detect("M.fs", "fsharp", src)
	if len(results) != 0 {
		t.Fatalf("expected no assertion_lib record, got %v", libsOf(results))
	}
}

// --- helpers ----------------------------------------------------------------

func hasLibrary(results []types.EntityRecord, subtype, library string) bool {
	for _, r := range results {
		if r.Subtype == subtype && r.Properties["library"] == library {
			return true
		}
	}
	return false
}

func libsOf(results []types.EntityRecord) []string {
	var out []string
	for _, r := range results {
		out = append(out, r.Subtype+":"+r.Properties["library"])
	}
	return out
}
