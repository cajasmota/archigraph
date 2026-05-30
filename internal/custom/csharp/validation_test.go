package csharp_test

// ---------------------------------------------------------------------------
// Validation — request_validation + dto_extraction
// ---------------------------------------------------------------------------

import "testing"

// FluentValidation: AbstractValidator<T> subclass
func TestValidationFluentAbstractValidator(t *testing.T) {
	src := `
using FluentValidation;

public class CreateOrderValidator : AbstractValidator<CreateOrderDto>
{
    public CreateOrderValidator()
    {
        RuleFor(x => x.CustomerId).NotEmpty();
        RuleFor(x => x.Amount).GreaterThan(0);
    }
}
`
	ents := extract(t, "custom_csharp_validation", fi("CreateOrderValidator.cs", "csharp", src))

	validationFound := false
	dtoFound := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "request_validation" && e.Name == "validation:fluent:CreateOrderValidator" {
			validationFound = true
		}
		if e.Kind == "SCOPE.Schema" && e.Subtype == "dto" && e.Name == "CreateOrderDto" {
			dtoFound = true
		}
	}
	if !validationFound {
		t.Error("expected validation:fluent:CreateOrderValidator SCOPE.Pattern entity")
	}
	if !dtoFound {
		t.Error("expected CreateOrderDto SCOPE.Schema dto entity from AbstractValidator")
	}
}

// FluentValidation: qualified namespace (FluentValidation.AbstractValidator<T>)
func TestValidationFluentQualifiedNamespace(t *testing.T) {
	src := `
public class UserValidator : FluentValidation.AbstractValidator<UserRequest>
{
    public UserValidator()
    {
        RuleFor(u => u.Email).NotEmpty().EmailAddress();
    }
}
`
	ents := extract(t, "custom_csharp_validation", fi("UserValidator.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "request_validation" && e.Name == "validation:fluent:UserValidator" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation:fluent:UserValidator from qualified FluentValidation.AbstractValidator<T>")
	}
}

// DataAnnotations: [Required] on model properties
func TestValidationDataAnnotationsRequired(t *testing.T) {
	src := `
using System.ComponentModel.DataAnnotations;

public class RegisterRequest
{
    [Required]
    public string Username { get; set; }

    [Required]
    [StringLength(100, MinimumLength = 6)]
    public string Password { get; set; }

    [EmailAddress]
    public string Email { get; set; }
}
`
	ents := extract(t, "custom_csharp_validation", fi("RegisterRequest.cs", "csharp", src))

	validationCount := 0
	dtoFound := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "request_validation" {
			validationCount++
		}
		if e.Kind == "SCOPE.Schema" && e.Subtype == "dto" && e.Name == "RegisterRequest" {
			dtoFound = true
		}
	}
	if validationCount == 0 {
		t.Error("expected at least one request_validation SCOPE.Pattern from DataAnnotations")
	}
	if !dtoFound {
		t.Error("expected RegisterRequest SCOPE.Schema dto from DataAnnotation model")
	}
}

// DataAnnotations: [Range] and [RegularExpression]
func TestValidationDataAnnotationsRangeRegex(t *testing.T) {
	src := `
public class ProductDto
{
    [Range(0.01, 9999.99)]
    public decimal Price { get; set; }

    [RegularExpression(@"^[A-Z]{2,4}$")]
    public string Code { get; set; }
}
`
	ents := extract(t, "custom_csharp_validation", fi("ProductDto.cs", "csharp", src))
	rangeFound := false
	regexFound := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "request_validation" {
			if containsEntity([]entitySummary{{Kind: e.Kind, Subtype: e.Subtype, Name: e.Name}}, "SCOPE.Pattern", e.Name) {
				// Just check they exist
				if e.Name != "" {
					rangeFound = rangeFound || (len(e.Name) > 0)
					regexFound = regexFound || (len(e.Name) > 0)
				}
			}
		}
	}
	count := 0
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "request_validation" {
			count++
		}
	}
	if count < 2 {
		t.Errorf("expected at least 2 DataAnnotation validation patterns, got %d", count)
	}
	_ = rangeFound
	_ = regexFound
}

// [ApiController] auto-validation
func TestValidationApiControllerAutoValidation(t *testing.T) {
	src := `
[ApiController]
[Route("api/[controller]")]
public class OrdersController : ControllerBase
{
    [HttpPost]
    public IActionResult Create([FromBody] CreateOrderDto dto) => Ok();
}
`
	ents := extract(t, "custom_csharp_validation", fi("OrdersController.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "request_validation" && e.Name == "validation:ApiController:auto:OrdersController.cs" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation:ApiController:auto entity from [ApiController]")
	}
}

// Wrong language should produce no entities.
func TestValidationWrongLanguageSkipped(t *testing.T) {
	src := `[Required] public string Name { get; set; }`
	ents := extract(t, "custom_csharp_validation", fi("file.java", "java", src))
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for non-csharp language, got %d", len(ents))
	}
}

// No match — plain file with no validation patterns.
func TestValidationNoMatch(t *testing.T) {
	src := `namespace App { class Helper { public void DoWork() {} } }`
	ents := extract(t, "custom_csharp_validation", fi("Helper.cs", "csharp", src))
	if len(ents) != 0 {
		t.Errorf("expected 0 validation entities, got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// csVal: ModelState.IsValid (issue #3380)
// ---------------------------------------------------------------------------

func TestValidationModelStateIsValid(t *testing.T) {
	src := `
[HttpPost]
public IActionResult Create([FromBody] CreateOrderDto dto)
{
    if (!ModelState.IsValid)
    {
        return BadRequest(ModelState);
    }
    return Ok();
}
`
	ents := extract(t, "custom_csharp_validation", fi("OrdersController.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "request_validation" {
			// Name pattern: "validation:ModelState.IsValid:OrdersController.cs:<line>"
			if len(e.Name) > len("validation:ModelState.IsValid:") &&
				e.Name[:len("validation:ModelState.IsValid:")] == "validation:ModelState.IsValid:" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected validation:ModelState.IsValid entity from ModelState.IsValid check")
	}
}

func TestValidationModelStateIsValidMultiple(t *testing.T) {
	src := `
public IActionResult Create([FromBody] CreateDto dto)
{
    if (!ModelState.IsValid) return BadRequest(ModelState);
    return Ok();
}
public IActionResult Update([FromBody] UpdateDto dto)
{
    if (!ModelState.IsValid) return BadRequest(ModelState);
    return Ok();
}
`
	ents := extract(t, "custom_csharp_validation", fi("Controller.cs", "csharp", src))
	count := 0
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "request_validation" &&
			len(e.Name) > len("validation:ModelState.IsValid:") &&
			e.Name[:len("validation:ModelState.IsValid:")] == "validation:ModelState.IsValid:" {
			count++
		}
	}
	if count < 2 {
		t.Errorf("expected at least 2 ModelState.IsValid entities, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// csVal: [FromBody] DTO binding → dto_extraction (issue #3380)
// ---------------------------------------------------------------------------

func TestValidationFromBodyDtoExtraction(t *testing.T) {
	src := `
[HttpPost]
public IActionResult Create([FromBody] CreateOrderRequest dto) => Ok();

[HttpPut("{id}")]
public IActionResult Update([FromBody] UpdateOrderRequest request) => Ok();
`
	ents := extract(t, "custom_csharp_validation", fi("OrdersController.cs", "csharp", src))
	createFound := false
	updateFound := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Schema" && e.Subtype == "dto" {
			if e.Name == "CreateOrderRequest" {
				createFound = true
			}
			if e.Name == "UpdateOrderRequest" {
				updateFound = true
			}
		}
	}
	if !createFound {
		t.Error("expected SCOPE.Schema dto CreateOrderRequest from [FromBody] binding")
	}
	if !updateFound {
		t.Error("expected SCOPE.Schema dto UpdateOrderRequest from [FromBody] binding")
	}
}

func TestValidationFromBodyPrimitivesSkipped(t *testing.T) {
	src := `
public IActionResult Delete([FromBody] int id) => Ok();
public IActionResult Toggle([FromBody] bool enabled) => Ok();
`
	ents := extract(t, "custom_csharp_validation", fi("Controller.cs", "csharp", src))
	for _, e := range ents {
		if e.Kind == "SCOPE.Schema" && e.Subtype == "dto" && (e.Name == "int" || e.Name == "bool") {
			t.Errorf("primitive type %q should not emit a dto entity", e.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// csVal: Per-property RuleFor property name extraction (issue #3380)
// ---------------------------------------------------------------------------

func TestValidationFluentRuleForProperties(t *testing.T) {
	src := `
public class CreateUserValidator : AbstractValidator<CreateUserDto>
{
    public CreateUserValidator()
    {
        RuleFor(x => x.Email).NotEmpty().EmailAddress();
        RuleFor(x => x.Password).NotEmpty().MinimumLength(8);
        RuleFor(x => x.Age).InclusiveBetween(18, 120);
    }
}
`
	ents := extract(t, "custom_csharp_validation", fi("CreateUserValidator.cs", "csharp", src))

	props := make(map[string]bool)
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "request_validation" &&
			len(e.Name) > len("validation:fluent:rule_for_prop:") &&
			e.Name[:len("validation:fluent:rule_for_prop:")] == "validation:fluent:rule_for_prop:" {
			// extract property name from: validation:fluent:rule_for_prop:PropName:file:line
			rest := e.Name[len("validation:fluent:rule_for_prop:"):]
			colonIdx := 0
			for i, c := range rest {
				if c == ':' {
					colonIdx = i
					break
				}
			}
			if colonIdx > 0 {
				props[rest[:colonIdx]] = true
			}
		}
	}

	for _, expected := range []string{"Email", "Password", "Age"} {
		if !props[expected] {
			t.Errorf("expected RuleFor property %q to be extracted", expected)
		}
	}
}

// ---------------------------------------------------------------------------
// csVal: DataAnnotation args capture (issue #3380)
// ---------------------------------------------------------------------------

func TestValidationAnnotationArgsStringLength(t *testing.T) {
	src := `
public class RegisterRequest
{
    [StringLength(100, MinimumLength = 6)]
    public string Username { get; set; }

    [Range(1, 150)]
    public int Age { get; set; }
}
`
	ents := extract(t, "custom_csharp_validation", fi("RegisterRequest.cs", "csharp", src))

	stringLenFound := false
	rangeFound := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "request_validation" &&
			len(e.Name) > len("validation:annotation_args:") &&
			e.Name[:len("validation:annotation_args:")] == "validation:annotation_args:" {
			if len(e.Name) > len("validation:annotation_args:StringLength:") &&
				e.Name[:len("validation:annotation_args:StringLength:")] == "validation:annotation_args:StringLength:" {
				stringLenFound = true
			}
			if len(e.Name) > len("validation:annotation_args:Range:") &&
				e.Name[:len("validation:annotation_args:Range:")] == "validation:annotation_args:Range:" {
				rangeFound = true
			}
		}
	}
	if !stringLenFound {
		t.Error("expected validation:annotation_args:StringLength entity with captured args")
	}
	if !rangeFound {
		t.Error("expected validation:annotation_args:Range entity with captured args")
	}
}
