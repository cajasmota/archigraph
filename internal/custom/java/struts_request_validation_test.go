package java

import (
	"testing"
)

// ============================================================================
// Issue #3256: Struts request_validation extractor
// ============================================================================
//
// Struts 2 request validation is implemented via:
//   1. public void validate() override on ActionSupport — programmatic
//      validation called by the workflow interceptor before execute().
//   2. @Validations / @Validation class-level annotations — declarative
//      validation enabled via the Struts 2 annotation framework.
//   3. Per-field validator annotations: @RequiredStringValidator,
//      @IntRangeFieldValidator, @EmailValidator, @RegexFieldValidator, etc.
//
// Struts 1 validation:
//   4. ActionForm.validate(ActionMapping, HttpServletRequest) override.
//
// XML-based:
//   5. <field name="..."> and <validator type="..."> in validation.xml.
//
// Registry target: lang.java.framework.struts Validation/request_validation → partial
// Cite: internal/custom/java/struts_routes.go

// ── Struts 2: validate() method override ─────────────────────────────────────

// strutsValidateMethodFixture is a Struts 2 action class that overrides
// validate() to enforce programmatic validation constraints.
const strutsValidateMethodFixture = `package com.example.actions;

import com.opensymphony.xwork2.ActionSupport;

public class RegistrationAction extends ActionSupport {

    private String username;
    private String email;
    private int age;

    @Override
    public void validate() {
        if (username == null || username.trim().isEmpty()) {
            addFieldError("username", "Username is required");
        }
        if (email == null || !email.contains("@")) {
            addFieldError("email", "Valid email required");
        }
        if (age < 18) {
            addFieldError("age", "Must be at least 18");
        }
    }

    @Override
    public String execute() {
        return SUCCESS;
    }

    public void setUsername(String username) { this.username = username; }
    public void setEmail(String email) { this.email = email; }
    public void setAge(int age) { this.age = age; }
}
`

// TestStruts_RequestValidation_ValidateMethod_Issue3256 proves that a
// public void validate() override is extracted as a SCOPE.Operation
// validation entity.
//
// Registry target: lang.java.framework.struts Validation/request_validation → partial
func TestStruts_RequestValidation_ValidateMethod_Issue3256(t *testing.T) {
	r := ExtractStruts(PatternContext{
		Source:    strutsValidateMethodFixture,
		Language:  "java",
		Framework: "struts2",
		FilePath:  "RegistrationAction.java",
	})

	found := false
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_STRUTS_VALIDATE_METHOD" {
			if e.Kind != "SCOPE.Operation" {
				t.Errorf("[#3256 request_validation] expected SCOPE.Operation, got %s", e.Kind)
			}
			if e.Subtype != "validation" {
				t.Errorf("[#3256 request_validation] expected subtype=validation, got %s", e.Subtype)
			}
			if e.Properties["framework"] != "struts" {
				t.Errorf("[#3256 request_validation] expected framework=struts, got %v", e.Properties["framework"])
			}
			if e.Properties["validation_kind"] != "validate_override" {
				t.Errorf("[#3256 request_validation] expected validation_kind=validate_override, got %v", e.Properties["validation_kind"])
			}
			if cls, _ := e.Properties["action_class"].(string); cls != "RegistrationAction" {
				t.Errorf("[#3256 request_validation] expected action_class=RegistrationAction, got %v", cls)
			}
			found = true
		}
	}
	if !found {
		t.Errorf("[#3256 request_validation] expected validation entity for validate() override")
	}
}

// ── Struts 2: annotation-based validation ────────────────────────────────────

// strutsAnnotationValidationFixture uses @Validations with nested field
// validator annotations.
const strutsAnnotationValidationFixture = `package com.example.actions;

import com.opensymphony.xwork2.ActionSupport;
import com.opensymphony.xwork2.validator.annotations.Validations;
import com.opensymphony.xwork2.validator.annotations.RequiredStringValidator;
import com.opensymphony.xwork2.validator.annotations.EmailValidator;
import com.opensymphony.xwork2.validator.annotations.IntRangeFieldValidator;

@Validations(
    requiredStrings = {
        @RequiredStringValidator(fieldName = "username", message = "Username is required"),
        @RequiredStringValidator(fieldName = "password", message = "Password is required")
    },
    emails = {
        @EmailValidator(fieldName = "email", message = "Invalid email")
    },
    intRangeFields = {
        @IntRangeFieldValidator(fieldName = "age", min = "18", max = "120", message = "Age out of range")
    }
)
public class SignupAction extends ActionSupport {
    private String username;
    private String password;
    private String email;
    private int age;

    @Override
    public String execute() {
        return SUCCESS;
    }

    public void setUsername(String u) { this.username = u; }
    public void setPassword(String p) { this.password = p; }
    public void setEmail(String e) { this.email = e; }
    public void setAge(int a) { this.age = a; }
}
`

// TestStruts_RequestValidation_ValidationsAnnotation_Issue3256 proves that
// @Validations is extracted as a validation entity.
func TestStruts_RequestValidation_ValidationsAnnotation_Issue3256(t *testing.T) {
	r := ExtractStruts(PatternContext{
		Source:    strutsAnnotationValidationFixture,
		Language:  "java",
		Framework: "struts2",
		FilePath:  "SignupAction.java",
	})

	foundValidations := false
	fieldValidators := make(map[string]bool)
	for _, e := range r.Entities {
		switch e.Provenance {
		case "INFERRED_FROM_STRUTS_VALIDATIONS_ANNOTATION":
			foundValidations = true
			if e.Kind != "SCOPE.Operation" || e.Subtype != "validation" {
				t.Errorf("[#3256 request_validation] @Validations: expected SCOPE.Operation/validation, got %s/%s",
					e.Kind, e.Subtype)
			}
		case "INFERRED_FROM_STRUTS_FIELD_VALIDATOR_ANNOTATION":
			if vt, ok := e.Properties["validator_type"].(string); ok {
				fieldValidators[vt] = true
			}
		}
	}

	if !foundValidations {
		t.Errorf("[#3256 request_validation] expected @Validations annotation entity")
	}
	for _, want := range []string{"@RequiredStringValidator", "@EmailValidator", "@IntRangeFieldValidator"} {
		if !fieldValidators[want] {
			t.Errorf("[#3256 request_validation] expected field validator %q, got %v", want, fieldValidators)
		}
	}
}

// TestStruts_RequestValidation_ValidationAnnotation_Issue3256 proves that the
// class-level @Validation annotation (enable flag) is extracted.
func TestStruts_RequestValidation_ValidationAnnotation_Issue3256(t *testing.T) {
	src := `package com.example;

import com.opensymphony.xwork2.ActionSupport;
import com.opensymphony.xwork2.validator.annotations.Validation;

@Validation
public class UpdateAction extends ActionSupport {
    @Override
    public String execute() { return SUCCESS; }
}
`
	r := ExtractStruts(PatternContext{
		Source:    src,
		Language:  "java",
		Framework: "struts",
		FilePath:  "UpdateAction.java",
	})

	found := false
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_STRUTS_VALIDATION_ANNOTATION" {
			found = true
			if e.Properties["validation_kind"] != "validation_annotation" {
				t.Errorf("[#3256 request_validation] expected validation_kind=validation_annotation, got %v",
					e.Properties["validation_kind"])
			}
		}
	}
	if !found {
		t.Errorf("[#3256 request_validation] expected @Validation annotation entity")
	}
}

// ── Struts 1: ActionForm.validate() ──────────────────────────────────────────

// strutsActionFormValidateFixture is a Struts 1 ActionForm that overrides the
// validate() callback.
const strutsActionFormValidateFixture = `package com.example.forms;

import org.apache.struts.action.ActionErrors;
import org.apache.struts.action.ActionForm;
import org.apache.struts.action.ActionMapping;
import org.apache.struts.action.ActionMessage;
import javax.servlet.http.HttpServletRequest;

public class LoginForm extends ActionForm {

    private String username;
    private String password;

    public ActionErrors validate(ActionMapping mapping, HttpServletRequest request) {
        ActionErrors errors = new ActionErrors();
        if (username == null || username.trim().isEmpty()) {
            errors.add("username", new ActionMessage("error.username.required"));
        }
        if (password == null || password.length() < 6) {
            errors.add("password", new ActionMessage("error.password.short"));
        }
        return errors;
    }

    public String getUsername() { return username; }
    public void setUsername(String u) { this.username = u; }
    public String getPassword() { return password; }
    public void setPassword(String p) { this.password = p; }
}
`

// TestStruts_RequestValidation_ActionFormValidate_Issue3256 proves that
// ActionForm.validate(ActionMapping, HttpServletRequest) overrides are
// extracted as validation entities.
func TestStruts_RequestValidation_ActionFormValidate_Issue3256(t *testing.T) {
	r := ExtractStruts(PatternContext{
		Source:    strutsActionFormValidateFixture,
		Language:  "java",
		Framework: "struts",
		FilePath:  "LoginForm.java",
	})

	found := false
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_STRUTS_ACTIONFORM_VALIDATE" {
			if e.Kind != "SCOPE.Operation" || e.Subtype != "validation" {
				t.Errorf("[#3256 request_validation] ActionForm.validate: expected SCOPE.Operation/validation, got %s/%s",
					e.Kind, e.Subtype)
			}
			if e.Properties["validation_kind"] != "actionform_validate" {
				t.Errorf("[#3256 request_validation] expected validation_kind=actionform_validate, got %v",
					e.Properties["validation_kind"])
			}
			found = true
		}
	}
	if !found {
		t.Errorf("[#3256 request_validation] expected ActionForm.validate() entity")
	}
}

// ── XML validation descriptor ─────────────────────────────────────────────────

// strutsValidationXMLFixture is a representative Struts 2 validation.xml file.
const strutsValidationXMLFixture = `<!DOCTYPE validators PUBLIC "-//Apache Struts//XWork Validator 1.0.3//EN"
    "http://struts.apache.org/dtds/xwork-validator-1.0.3.dtd">
<validators>
    <field name="username">
        <field-validator type="requiredstring">
            <message>Username is required</message>
        </field-validator>
        <field-validator type="stringlength">
            <param name="maxLength">50</param>
            <message>Username too long</message>
        </field-validator>
    </field>
    <field name="email">
        <field-validator type="email">
            <message>Invalid email address</message>
        </field-validator>
    </field>
    <validator type="expression">
        <param name="expression">password.equals(confirmPassword)</param>
        <message>Passwords do not match</message>
    </validator>
</validators>
`

// TestStruts_RequestValidation_XMLDescriptor_Issue3256 proves that
// <field name="..."> and <validator type="..."> in a validation XML file
// are extracted as validation entities.
func TestStruts_RequestValidation_XMLDescriptor_Issue3256(t *testing.T) {
	r := ExtractStruts(PatternContext{
		Source:    strutsValidationXMLFixture,
		Language:  "java",
		Framework: "struts2",
		FilePath:  "RegistrationAction-validation.xml",
	})

	xmlFields := make(map[string]bool)
	xmlValidators := make(map[string]bool)
	for _, e := range r.Entities {
		switch e.Provenance {
		case "INFERRED_FROM_STRUTS_VALIDATION_XML_FIELD":
			if fn, ok := e.Properties["field_name"].(string); ok {
				xmlFields[fn] = true
			}
			if e.Kind != "SCOPE.Operation" || e.Subtype != "validation" {
				t.Errorf("[#3256 xml_validation] expected SCOPE.Operation/validation, got %s/%s",
					e.Kind, e.Subtype)
			}
		case "INFERRED_FROM_STRUTS_VALIDATION_XML_VALIDATOR":
			if vt, ok := e.Properties["validator_type"].(string); ok {
				xmlValidators[vt] = true
			}
		}
	}

	for _, want := range []string{"username", "email"} {
		if !xmlFields[want] {
			t.Errorf("[#3256 xml_validation] expected XML field %q, got %v", want, xmlFields)
		}
	}
	if !xmlValidators["expression"] {
		t.Errorf("[#3256 xml_validation] expected XML validator type=expression, got %v", xmlValidators)
	}
}

// ── Gating tests ──────────────────────────────────────────────────────────────

// TestStruts_RequestValidation_WrongFramework_Issue3256 proves the extractor
// does not fire for non-struts frameworks.
func TestStruts_RequestValidation_WrongFramework_Issue3256(t *testing.T) {
	r := ExtractStruts(PatternContext{
		Source:    strutsValidateMethodFixture,
		Language:  "java",
		Framework: "spring_boot",
		FilePath:  "RegistrationAction.java",
	})
	for _, e := range r.Entities {
		if e.Subtype == "validation" {
			t.Errorf("[#3256 request_validation] expected no validation entities for framework=spring_boot, got %v", e)
		}
	}
}

// TestStruts_RequestValidation_AllFrameworkKeys_Issue3256 confirms that all
// Struts framework key aliases activate the validation extractor.
func TestStruts_RequestValidation_AllFrameworkKeys_Issue3256(t *testing.T) {
	for _, fw := range []string{"struts", "struts2", "struts-2", "apache_struts", "apache-struts"} {
		r := ExtractStruts(PatternContext{
			Source:    strutsValidateMethodFixture,
			Language:  "java",
			Framework: fw,
			FilePath:  "RegistrationAction.java",
		})
		found := false
		for _, e := range r.Entities {
			if e.Provenance == "INFERRED_FROM_STRUTS_VALIDATE_METHOD" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("[#3256 request_validation] expected validate() entity for framework=%s", fw)
		}
	}
}

// TestStruts_RequestValidation_FullAnnotatedAction_Issue3256 is a
// comprehensive proving test: a Struts 2 action with @Validation, @Validations,
// field validators, validate() override, and DTO setters — all in one file.
func TestStruts_RequestValidation_FullAnnotatedAction_Issue3256(t *testing.T) {
	src := `package com.example;

import com.opensymphony.xwork2.ActionSupport;
import com.opensymphony.xwork2.validator.annotations.Validation;
import com.opensymphony.xwork2.validator.annotations.Validations;
import com.opensymphony.xwork2.validator.annotations.RequiredStringValidator;
import com.opensymphony.xwork2.validator.annotations.EmailValidator;

@Validation
@Validations(
    requiredStrings = {
        @RequiredStringValidator(fieldName = "name", message = "Name required")
    },
    emails = {
        @EmailValidator(fieldName = "email", message = "Bad email")
    }
)
public class ContactAction extends ActionSupport {

    private String name;
    private String email;

    @Override
    public void validate() {
        if (name != null && name.length() > 100) {
            addFieldError("name", "Too long");
        }
    }

    @Override
    public String execute() { return SUCCESS; }

    public void setName(String n) { this.name = n; }
    public void setEmail(String e) { this.email = e; }
}
`
	r := ExtractStruts(PatternContext{
		Source:    src,
		Language:  "java",
		Framework: "struts2",
		FilePath:  "ContactAction.java",
	})

	provenances := make(map[string]bool)
	for _, e := range r.Entities {
		if e.Subtype == "validation" {
			provenances[e.Provenance] = true
		}
	}

	for _, want := range []string{
		"INFERRED_FROM_STRUTS_VALIDATION_ANNOTATION",
		"INFERRED_FROM_STRUTS_VALIDATIONS_ANNOTATION",
		"INFERRED_FROM_STRUTS_FIELD_VALIDATOR_ANNOTATION",
		"INFERRED_FROM_STRUTS_VALIDATE_METHOD",
	} {
		if !provenances[want] {
			t.Errorf("[#3256 request_validation full] expected provenance %q, got %v", want, provenances)
		}
	}
}
