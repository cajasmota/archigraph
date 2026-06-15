package javascript_test

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/types"

	_ "github.com/cajasmota/grafel/internal/custom/javascript"
)

// Issue #4976 — fold zod/joi/yup chain constraints into the per-field
// `validations` chip list so the ShapeTree renders schema constraints
// (`MaxLength:120`, `Email`, `Min:0`, `Required`, …) with the same chip format
// as class-validator (#4858). Builds on #4925 (zod/joi/yup schema + field
// members).

// fieldChips returns the comma-split Properties["validations"] chips for the
// SCOPE.Schema/field sub-entity named "<Schema>.<field>".
func fieldChips(ents []types.EntityRecord, qualified string) []string {
	e := fieldChild(ents, qualified)
	if e == nil || e.Properties == nil {
		return nil
	}
	raw := strings.TrimSpace(e.Properties["validations"])
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hasChip(chips []string, want string) bool {
	for _, c := range chips {
		if c == want {
			return true
		}
	}
	return false
}

func wantChips(t *testing.T, ents []types.EntityRecord, field string, want ...string) {
	t.Helper()
	chips := fieldChips(ents, field)
	for _, w := range want {
		if !hasChip(chips, w) {
			t.Errorf("field %q: missing chip %q (got %v)", field, w, chips)
		}
	}
}

func TestZodChainConstraints_StampedAsChips(t *testing.T) {
	src := `import { z } from 'zod';
const CreateUser = z.object({
  name: z.string().min(2).max(120),
  email: z.string().email(),
  age: z.number().int().min(0).max(150),
  bio: z.string().max(500).optional(),
  website: z.string().url(),
});`
	ents := extractFull(t, "custom_js_validation_schema", fi("users.ts", "typescript", src))

	// string min/max → MinLength/MaxLength with the scalar bound folded in.
	wantChips(t, ents, "CreateUser.name", "MinLength:2", "MaxLength:120")
	// .email() → flag chip.
	wantChips(t, ents, "CreateUser.email", "Email")
	// numeric min/max stay Min/Max; .int() → Int.
	wantChips(t, ents, "CreateUser.age", "Int", "Min:0", "Max:150")
	// string .max() on an optional field still folds the bound.
	wantChips(t, ents, "CreateUser.bio", "MaxLength:500")
	wantChips(t, ents, "CreateUser.website", "Url")

	// .optional() marks the field optional.
	bio := fieldChild(ents, "CreateUser.bio")
	if bio == nil || bio.Properties["optional"] != "true" {
		t.Errorf("CreateUser.bio expected optional=true (props=%v)", bio.Properties)
	}
}

func TestJoiChainConstraints_StampedAsChips(t *testing.T) {
	src := `const CreateUser = Joi.object({
  name: Joi.string().min(2).max(120).required(),
  email: Joi.string().email().required(),
  age: Joi.number().integer().min(0).max(10),
  nickname: Joi.string().pattern(/^[a-z]+$/),
});`
	ents := extractFull(t, "custom_js_validation_schema", fi("users.ts", "typescript", src))

	wantChips(t, ents, "CreateUser.name", "MinLength:2", "MaxLength:120", "Required")
	wantChips(t, ents, "CreateUser.email", "Email", "Required")
	wantChips(t, ents, "CreateUser.age", "Int", "Min:0", "Max:10")
	// regex literal arg is compound → bare Pattern chip (no folded arg).
	wantChips(t, ents, "CreateUser.nickname", "Pattern")

	// joi defaults to optional unless .required() is present.
	nick := fieldChild(ents, "CreateUser.nickname")
	if nick == nil || nick.Properties["optional"] != "true" {
		t.Errorf("CreateUser.nickname expected optional=true (props=%v)", nick.Properties)
	}
	name := fieldChild(ents, "CreateUser.name")
	if name != nil && name.Properties["optional"] == "true" {
		t.Errorf("CreateUser.name has .required() → expected not optional (props=%v)", name.Properties)
	}
}

func TestYupChainConstraints_StampedAsChips(t *testing.T) {
	src := `const CreateUser = yup.object().shape({
  name: yup.string().required().min(2).max(120),
  email: yup.string().email(),
  age: yup.number().integer().min(0).max(150),
  handle: yup.string().matches(/^@\w+$/),
});`
	ents := extractFull(t, "custom_js_validation_schema", fi("users.ts", "typescript", src))

	wantChips(t, ents, "CreateUser.name", "Required", "MinLength:2", "MaxLength:120")
	wantChips(t, ents, "CreateUser.email", "Email")
	wantChips(t, ents, "CreateUser.age", "Int", "Min:0", "Max:150")
	wantChips(t, ents, "CreateUser.handle", "Pattern")
}

// A schema field with no recognised chain constraints must carry no
// `validations` property (honest — no empty chip bag).
func TestSchemaField_NoChainConstraints_NoValidationsProp(t *testing.T) {
	src := `import { z } from 'zod';
const Plain = z.object({ id: z.string(), flag: z.boolean() });`
	ents := extractFull(t, "custom_js_validation_schema", fi("plain.ts", "typescript", src))

	for _, f := range []string{"Plain.id", "Plain.flag"} {
		e := fieldChild(ents, f)
		if e == nil {
			t.Fatalf("expected field member %q", f)
		}
		if _, ok := e.Properties["validations"]; ok {
			t.Errorf("field %q should carry no validations prop (got %q)", f, e.Properties["validations"])
		}
	}
}
