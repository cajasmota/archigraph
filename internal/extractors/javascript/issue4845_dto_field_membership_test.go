// Package javascript — issue #4845 NestJS/TS DTO field-membership tests.
//
// LIVE root cause (upvate-v3): a DTO class such as `CreateAlternateAddressBody`
// RESOLVES to a SCOPE.Component/class entity (type_entity_id present), but the
// dashboard shape endpoint returned `{subtype:"class", rows:[]}` — the class had
// ZERO field children. The property fields WERE extracted (as SCOPE.Schema/field
// entities, #679) but the class→field CONTAINS edges were NOT emitted: the
// CONTAINS loop in handleClassDeclaration only linked SCOPE.Operation methods,
// so classHasFieldChildren returned false and the Params/Response panels showed
// no expand glyph.
//
// After #4845 every plain/decorated class property field gets a CONTAINS edge
// (mirroring the Java/Python/Kotlin fixes), and NestJS mapped-type DTOs
// (`extends PartialType(X)` / PickType / OmitType / IntersectionType) emit an
// EXTENDS edge to the field-bearing base DTO so the shape walker can recurse.
package javascript_test

import (
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

// hasContainsField reports whether the class named `class` carries a CONTAINS
// edge to the SCOPE.Schema/field structural-ref for `<class>.<field>`.
func hasContainsField(ents []types.EntityRecord, path, className, field string) bool {
	want := extreg.BuildSchemaFieldStructuralRef("typescript", path, className+"."+field)
	for _, e := range ents {
		if e.Name != className || e.Kind != "SCOPE.Component" {
			continue
		}
		for _, r := range e.Relationships {
			if r.Kind == "CONTAINS" && r.ToID == want {
				return true
			}
		}
	}
	return false
}

// fieldEntityExists reports whether a SCOPE.Schema/field entity named
// "<class>.<field>" was emitted.
func fieldEntityExists(ents []types.EntityRecord, className, field string) bool {
	want := className + "." + field
	for _, e := range ents {
		if e.Kind == "SCOPE.Schema" && e.Subtype == "field" && e.Name == want {
			return true
		}
	}
	return false
}

// TestDTO_PlainAndDecoratedFieldsAreContained proves a NestJS DTO with plain,
// readonly, optional, and @ApiProperty-decorated properties emits one
// SCOPE.Schema/field entity per property AND a class→field CONTAINS edge for
// each — the exact gap that left shape rows empty on upvate-v3.
func TestDTO_PlainAndDecoratedFieldsAreContained(t *testing.T) {
	path := "src/address/dto/create-alternate-address-body.dto.ts"
	src := []byte(`
import { ApiProperty } from '@nestjs/swagger';

export class CreateAlternateAddressBody {
  @ApiProperty()
  name: string;

  readonly line1: string;

  street?: string;

  count = 0;
}
`)
	ents := extractHeritageTS(t, path, src)

	for _, f := range []string{"name", "line1", "street", "count"} {
		if !fieldEntityExists(ents, "CreateAlternateAddressBody", f) {
			t.Errorf("expected SCOPE.Schema/field entity CreateAlternateAddressBody.%s", f)
		}
		if !hasContainsField(ents, path, "CreateAlternateAddressBody", f) {
			t.Errorf("expected CONTAINS edge from class to field %q", f)
		}
	}
}

// TestDTO_MappedTypeExtendsBase proves a mapped-type DTO
// (`extends PartialType(CreateThingBody)`) emits an EXTENDS edge to the
// field-bearing base DTO (and NOT to the PartialType helper itself), so the
// shape walker can recurse into the base's fields even though the mapped DTO
// owns none of its own.
func TestDTO_MappedTypeExtendsBase(t *testing.T) {
	path := "src/thing/dto/update-thing-body.dto.ts"
	src := []byte(`
import { PartialType, PickType, OmitType, IntersectionType } from '@nestjs/mapped-types';
import { CreateThingBody } from './create-thing-body.dto';
import { OtherBody } from './other-body.dto';

export class CreateThingBody {
  name: string;
  size: number;
}

export class UpdateThingBody extends PartialType(CreateThingBody) {}

export class PickThingBody extends PickType(CreateThingBody, ['name'] as const) {}

export class OmitThingBody extends OmitType(CreateThingBody, ['size'] as const) {}

export class MergedBody extends IntersectionType(CreateThingBody, OtherBody) {}
`)
	ents := extractHeritageTS(t, path, src)

	for _, mapped := range []string{"UpdateThingBody", "PickThingBody", "OmitThingBody"} {
		if !hasHeritageEdge(ents, mapped, "EXTENDS", "CreateThingBody") {
			t.Errorf("%s should EXTENDS the base DTO CreateThingBody", mapped)
		}
		// The helper itself must NOT be emitted as a base.
		if hasHeritageEdge(ents, mapped, "EXTENDS", "PartialType") ||
			hasHeritageEdge(ents, mapped, "EXTENDS", "PickType") ||
			hasHeritageEdge(ents, mapped, "EXTENDS", "OmitType") {
			t.Errorf("%s must not EXTENDS the mapped-type helper", mapped)
		}
	}
	// IntersectionType yields an EXTENDS edge to BOTH argument DTOs.
	if !hasHeritageEdge(ents, "MergedBody", "EXTENDS", "CreateThingBody") {
		t.Error("MergedBody should EXTENDS CreateThingBody (IntersectionType arg 1)")
	}
	if !hasHeritageEdge(ents, "MergedBody", "EXTENDS", "OtherBody") {
		t.Error("MergedBody should EXTENDS OtherBody (IntersectionType arg 2)")
	}
}

// TestDTO_PlainExtendsBaseStillContainsOwnFields guards the general case: a DTO
// that `extends BaseDto` and ALSO declares its own fields keeps CONTAINS edges
// for its own fields (the EXTENDS-recursion is additive, not a replacement).
func TestDTO_PlainExtendsBaseStillContainsOwnFields(t *testing.T) {
	path := "src/user/dto/admin-user-body.dto.ts"
	src := []byte(`
export class BaseUserBody {
  email: string;
}

export class AdminUserBody extends BaseUserBody {
  role: string;
}
`)
	ents := extractHeritageTS(t, path, src)

	if !hasContainsField(ents, path, "AdminUserBody", "role") {
		t.Error("AdminUserBody should CONTAINS its own field role")
	}
	if !hasContainsField(ents, path, "BaseUserBody", "email") {
		t.Error("BaseUserBody should CONTAINS its field email")
	}
	if !hasHeritageEdge(ents, "AdminUserBody", "EXTENDS", "BaseUserBody") {
		t.Error("AdminUserBody should EXTENDS BaseUserBody")
	}
}
