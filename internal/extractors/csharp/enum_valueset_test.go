package csharp_test

// enum_valueset_test.go — value-asserting tests for the C# enum value-set node
// (data-model, epic #3628). Asserts the SCOPE.Enum node + specific member
// values, not merely a non-empty member list.

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

func findCSharpEnum(recs []types.EntityRecord, name string) *types.EntityRecord {
	for i := range recs {
		if recs[i].Kind == "SCOPE.Enum" && recs[i].Name == name {
			return &recs[i]
		}
	}
	return nil
}

func TestEnumValueSet_CSharpExplicitValues(t *testing.T) {
	src := `
namespace App
{
    public enum Status
    {
        Active = 1,
        Inactive = 2
    }
}
`
	recs := extractCSharpRecords(t, src)
	en := findCSharpEnum(recs, "Status")
	if en == nil {
		t.Fatal("SCOPE.Enum:Status value-set node not found")
	}
	if got := en.Properties["kind_hint"]; got != "csharp_enum" {
		t.Fatalf("kind_hint = %q, want csharp_enum", got)
	}
	if got := en.Properties["values"]; got != "Active=1, Inactive=2" {
		t.Fatalf("values = %q, want %q", got, "Active=1, Inactive=2")
	}
}

func TestEnumValueSet_CSharpImplicitMembers(t *testing.T) {
	src := `
namespace App
{
    public enum Direction { Up, Down, Left, Right }
}
`
	recs := extractCSharpRecords(t, src)
	en := findCSharpEnum(recs, "Direction")
	if en == nil {
		t.Fatal("SCOPE.Enum:Direction value-set node not found")
	}
	if got := en.Properties["members"]; got != "Up, Down, Left, Right" {
		t.Fatalf("members = %q, want %q", got, "Up, Down, Left, Right")
	}
	// No explicit values → no fabricated ordinals.
	if got, ok := en.Properties["values"]; ok {
		t.Fatalf("values should be absent for implicit members, got %q", got)
	}
}
