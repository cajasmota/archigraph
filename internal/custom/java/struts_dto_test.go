package java

import (
	"os"
	"testing"
)

// ============================================================================
// Issue #3191: Struts dto_extraction extractor (ActionForm + Struts 2 binding)
//
// Registry target: lang.java.framework.struts Validation/dto_extraction → partial
// Cite: internal/custom/java/struts_routes.go
//       internal/custom/java/struts_dto_test.go
//       testdata/fixtures/sources/java/struts/StrutsDtoFixture.java
// ============================================================================

func strutsDTOCtx(source, file string) PatternContext {
	return PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "struts",
		FilePath:  file,
	}
}

// schemaNames returns the names of SCOPE.Schema entities with the given
// provenance.
func strutsSchemaNamesByProvenance(r PatternResult, provenance string) []string {
	var out []string
	for _, e := range r.Entities {
		if e.Kind == "SCOPE.Schema" && e.Provenance == provenance {
			out = append(out, e.Name)
		}
	}
	return out
}

func strutsHasEntity(r PatternResult, kind, name string) bool {
	for _, e := range r.Entities {
		if e.Kind == kind && e.Name == name {
			return true
		}
	}
	return false
}

func strutsHasRel(r PatternResult, src, tgt, typ string) bool {
	for _, rel := range r.Relationships {
		if rel.RelationshipType == typ && rel.SourceRef == src && rel.TargetRef == tgt {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------------
// Struts 1: ActionForm subclass detection
// ----------------------------------------------------------------------------

// TestStruts_DTO_ActionForm_Issue3191 proves that `extends ActionForm`
// subclasses are emitted as SCOPE.Schema DTO entities with their bound fields.
func TestStruts_DTO_ActionForm_Issue3191(t *testing.T) {
	source := `
package com.example;

import org.apache.struts.action.ActionForm;

public class RegisterForm extends ActionForm {
    private String username;
    public String getUsername() { return username; }
    public void setUsername(String username) { this.username = username; }
    public void setEmail(String email) { }
}
`
	r := ExtractStruts(strutsDTOCtx(source, "RegisterForm.java"))

	if !strutsHasEntity(r, "SCOPE.Schema", "RegisterForm") {
		t.Fatalf("[#3191 dto_extraction] expected SCOPE.Schema DTO for RegisterForm; got %+v", r.Entities)
	}

	// Bound fields: username + email.
	gotFields := map[string]bool{}
	for _, e := range r.Entities {
		if e.Kind == "SCOPE.Field" {
			gotFields[e.Properties["field_name"].(string)] = true
		}
	}
	for _, want := range []string{"username", "email"} {
		if !gotFields[want] {
			t.Errorf("[#3191 dto_extraction] expected bound field %q, got %v", want, gotFields)
		}
	}

	// BINDS_INPUT relationships present.
	var binds int
	for _, rel := range r.Relationships {
		if rel.RelationshipType == "BINDS_INPUT" {
			binds++
		}
	}
	if binds < 2 {
		t.Errorf("[#3191 dto_extraction] expected >=2 BINDS_INPUT rels; got %d", binds)
	}
}

// ----------------------------------------------------------------------------
// Struts 2: ActionSupport field-binding (OGNL setters)
// ----------------------------------------------------------------------------

// TestStruts_DTO_ActionSupportBinding_Issue3191 proves that an ActionSupport
// subclass with public setters is treated as a request-binding DTO.
func TestStruts_DTO_ActionSupportBinding_Issue3191(t *testing.T) {
	source := `
package com.example;

import com.opensymphony.xwork2.ActionSupport;

public class SearchAction extends ActionSupport {
    private String query;
    public void setQuery(String query) { this.query = query; }
    public void setLimit(int limit) { }
}
`
	r := ExtractStruts(strutsDTOCtx(source, "SearchAction.java"))

	if !strutsHasEntity(r, "SCOPE.Schema", "SearchAction") {
		t.Fatalf("[#3191 dto_extraction] expected SCOPE.Schema DTO for SearchAction; got %+v", r.Entities)
	}
	form := ""
	for _, e := range r.Entities {
		if e.Name == "SearchAction" && e.Kind == "SCOPE.Schema" {
			form = e.Properties["form_kind"].(string)
		}
	}
	if form != "ActionSupport" {
		t.Errorf("[#3191 dto_extraction] expected form_kind=ActionSupport; got %q", form)
	}

	gotFields := map[string]bool{}
	for _, e := range r.Entities {
		if e.Kind == "SCOPE.Field" {
			gotFields[e.Properties["field_name"].(string)] = true
		}
	}
	for _, want := range []string{"query", "limit"} {
		if !gotFields[want] {
			t.Errorf("[#3191 dto_extraction] expected bound field %q, got %v", want, gotFields)
		}
	}
}

// ----------------------------------------------------------------------------
// Struts 2: framework-plumbing setters are not bound DTO fields
// ----------------------------------------------------------------------------

// TestStruts_DTO_SkipPlumbingSetters_Issue3191 proves that
// setServletRequest/setSession-style plumbing setters are not surfaced as
// bound DTO fields.
func TestStruts_DTO_SkipPlumbingSetters_Issue3191(t *testing.T) {
	source := `
import com.opensymphony.xwork2.ActionSupport;

public class CartAction extends ActionSupport {
    public void setItem(String item) { }
    public void setServletRequest(Object r) { }
    public void setSession(java.util.Map s) { }
}
`
	r := ExtractStruts(strutsDTOCtx(source, "CartAction.java"))
	for _, e := range r.Entities {
		if e.Kind == "SCOPE.Field" {
			name := e.Properties["field_name"].(string)
			if name == "servletRequest" || name == "session" {
				t.Errorf("[#3191 dto_extraction] plumbing setter %q must not be a bound field", name)
			}
		}
	}
	if !strutsHasEntity(r, "SCOPE.Field", "CartAction.item") {
		t.Errorf("[#3191 dto_extraction] expected real field CartAction.item bound")
	}
}

// ----------------------------------------------------------------------------
// Struts 2: ModelDriven<T> exposes a separate domain model
// ----------------------------------------------------------------------------

// TestStruts_DTO_ModelDriven_Issue3191 proves that ModelDriven<T> surfaces the
// model type T as its own DTO and links the action via BINDS_MODEL.
func TestStruts_DTO_ModelDriven_Issue3191(t *testing.T) {
	source := `
import com.opensymphony.xwork2.ActionSupport;
import com.opensymphony.xwork2.ModelDriven;

public class AccountAction extends ActionSupport implements ModelDriven<Account> {
    private final Account account = new Account();
    public Account getModel() { return account; }
}
`
	r := ExtractStruts(strutsDTOCtx(source, "AccountAction.java"))

	if !strutsHasEntity(r, "SCOPE.Schema", "Account") {
		t.Fatalf("[#3191 dto_extraction] expected SCOPE.Schema DTO for ModelDriven model Account; got %+v", r.Entities)
	}
	actionRef := "scope:schema:struts_dto:AccountAction.java:AccountAction"
	modelRef := "scope:schema:struts_dto:AccountAction.java:Account"
	if !strutsHasRel(r, actionRef, modelRef, "BINDS_MODEL") {
		t.Errorf("[#3191 dto_extraction] expected BINDS_MODEL %s -> %s; rels=%+v", actionRef, modelRef, r.Relationships)
	}
}

// ----------------------------------------------------------------------------
// Gating: extractor must not fire for non-struts frameworks
// ----------------------------------------------------------------------------

func TestStruts_DTO_Gating_Issue3191(t *testing.T) {
	source := `public class FooForm extends ActionForm { public void setX(String x) {} }`
	r := ExtractStruts(PatternContext{
		Source: source, Language: "java", Framework: "spring_boot", FilePath: "FooForm.java",
	})
	if len(r.Entities) != 0 {
		t.Errorf("[#3191 gating] extractor must not fire for framework=spring_boot; got %+v", r.Entities)
	}
}

// ----------------------------------------------------------------------------
// Golden fixture integration test
// ----------------------------------------------------------------------------

// TestStruts_DTO_FixtureFile_Issue3191 loads the committed golden fixture and
// asserts the full set of DTO extractions, proving dto_extraction → partial.
func TestStruts_DTO_FixtureFile_Issue3191(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/fixtures/sources/java/struts/StrutsDtoFixture.java")
	if err != nil {
		t.Fatalf("[#3191 fixture] cannot read fixture: %v", err)
	}
	r := ExtractStruts(strutsDTOCtx(string(data), "StrutsDtoFixture.java"))

	// 1. Struts 1 ActionForm subclasses.
	actionForms := strutsSchemaNamesByProvenance(r, "INFERRED_FROM_STRUTS_ACTIONFORM")
	for _, want := range []string{"UserForm", "LoginForm"} {
		found := false
		for _, n := range actionForms {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("[#3191 fixture] missing ActionForm DTO %q; got %v", want, actionForms)
		}
	}

	// 2. Struts 2 action binding DTOs.
	if !strutsHasEntity(r, "SCOPE.Schema", "OrderAction") {
		t.Errorf("[#3191 fixture] expected ActionSupport DTO OrderAction; got %v", actionForms)
	}

	// 3. ModelDriven model surfaced + linked.
	modelDriven := strutsSchemaNamesByProvenance(r, "INFERRED_FROM_STRUTS_MODELDRIVEN")
	foundProduct := false
	for _, n := range modelDriven {
		if n == "Product" {
			foundProduct = true
		}
	}
	if !foundProduct {
		t.Errorf("[#3191 fixture] expected ModelDriven model Product; got %v", modelDriven)
	}
	productActionRef := "scope:schema:struts_dto:StrutsDtoFixture.java:ProductAction"
	productRef := "scope:schema:struts_dto:StrutsDtoFixture.java:Product"
	if !strutsHasRel(r, productActionRef, productRef, "BINDS_MODEL") {
		t.Errorf("[#3191 fixture] expected BINDS_MODEL ProductAction -> Product")
	}

	// 4. Bound fields from UserForm (incl. type capture) but NOT plumbing.
	wantFields := map[string]string{"username": "String", "email": "String", "age": "int"}
	gotTypes := map[string]string{}
	for _, e := range r.Entities {
		if e.Kind == "SCOPE.Field" && e.Properties["dto"] == "UserForm" {
			gotTypes[e.Properties["field_name"].(string)] = e.Properties["field_type"].(string)
		}
	}
	for f, typ := range wantFields {
		if gotTypes[f] != typ {
			t.Errorf("[#3191 fixture] UserForm.%s expected type %q; got %q (all=%v)", f, typ, gotTypes[f], gotTypes)
		}
	}
	if _, bad := gotTypes["servletRequest"]; bad {
		t.Errorf("[#3191 fixture] UserForm must not bind plumbing setServletRequest")
	}

	// 5. BINDS_INPUT relationships present overall.
	var binds int
	for _, rel := range r.Relationships {
		if rel.RelationshipType == "BINDS_INPUT" {
			binds++
		}
	}
	if binds == 0 {
		t.Errorf("[#3191 fixture] expected >=1 BINDS_INPUT relationship; got 0")
	}
}
