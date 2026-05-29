package golang_test

import (
	"testing"
)

// Tests for the middleware/auth/request_validation passes added to the three
// Go codegen framework extractors (kratos, go-zero, huma) — issue #3255.
// Reuses fullEntity/extractFull (middleware_auth_test.go), fi (extractors_test.go),
// and fixtureInput (gorm_test.go).

// findPattern returns the first SCOPE.Pattern entity matching name, or nil.
func findPattern(ents []fullEntity, name string) *fullEntity {
	for i := range ents {
		if ents[i].Kind == "SCOPE.Pattern" && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

// hasPatternKind reports whether any SCOPE.Pattern carries pattern_kind=kind.
func hasPatternKind(ents []fullEntity, kind string) bool {
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Props["pattern_kind"] == kind {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// kratos
// ---------------------------------------------------------------------------

func TestKratosMiddlewareChain(t *testing.T) {
	src := `
import "github.com/go-kratos/kratos/v2/transport/http"
func newServer() {
	http.Middleware(recovery.Recovery(), logging.Server())
}`
	ents := extractFull(t, "custom_go_kratos", fi("server.go", "go", src))
	for name, order := range map[string]string{
		"recovery.Recovery()": "0",
		"logging.Server()":    "1",
	} {
		mw := findPattern(ents, name)
		if mw == nil {
			t.Fatalf("missing middleware %q", name)
		}
		if mw.Props["pattern_kind"] != "middleware" {
			t.Errorf("%s: pattern_kind=%q", name, mw.Props["pattern_kind"])
		}
		if mw.Props["mw_order"] != order {
			t.Errorf("%s: mw_order=%q want %q", name, mw.Props["mw_order"], order)
		}
	}
}

func TestKratosAuthDetection(t *testing.T) {
	src := `
import "github.com/go-kratos/kratos/v2/transport/http"
func newServer() {
	http.Middleware(jwt.Server(keyFunc))
}`
	ents := extractFull(t, "custom_go_kratos", fi("server.go", "go", src))
	mw := findPattern(ents, "jwt.Server(keyFunc)")
	if mw == nil {
		t.Fatal("missing jwt middleware")
	}
	if mw.Props["is_auth"] != "true" || mw.Props["auth_kind"] != "jwt" {
		t.Errorf("jwt mw: is_auth=%q auth_kind=%q", mw.Props["is_auth"], mw.Props["auth_kind"])
	}
	if findPattern(ents, "auth:jwt.Server") == nil {
		t.Error("expected dedicated auth:jwt.Server pattern")
	}
}

func TestKratosRequestValidation(t *testing.T) {
	src := `
package v1
func (m *HelloRequest) Validate() error { return nil }
import "github.com/go-kratos/kratos/v2/middleware/validate"
func mw() { validate.Validator() }`
	ents := extractFull(t, "custom_go_kratos", fi("x.pb.validate.go", "go", src))
	if findPattern(ents, "validation:rule:HelloRequest") == nil {
		t.Error("expected pgv validation rule for HelloRequest")
	}
	if findPattern(ents, "validation:binding:validate_call") == nil {
		t.Error("expected validate.Validator() binding pattern")
	}
}

func TestKratosWiringFixture(t *testing.T) {
	ents := extractFull(t, "custom_go_kratos", fixtureInput(t, "kratos_server.go", "go"))
	if !hasPatternKind(ents, "middleware") {
		t.Error("fixture: expected middleware patterns")
	}
	if !hasPatternKind(ents, "auth") {
		t.Error("fixture: expected auth pattern (jwt via selector)")
	}
	if findPattern(ents, "validate.Validator()") == nil {
		t.Error("fixture: expected validate.Validator() middleware")
	}
}

func TestKratosPGVFixture(t *testing.T) {
	ents := extractFull(t, "custom_go_kratos", fixtureInput(t, "kratos_request.pb.validate.go", "go"))
	for _, msg := range []string{"validation:rule:HelloRequest", "validation:rule:CreateGreetingRequest"} {
		if findPattern(ents, msg) == nil {
			t.Errorf("fixture: expected %q", msg)
		}
	}
}

func TestKratosWiringNoMatch(t *testing.T) {
	// Plain Go file with no kratos marker emits nothing.
	ents := extractFull(t, "custom_go_kratos", fi("util.go", "go", `package util
func Add(a, b int) int { return a + b }`))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// go-zero
// ---------------------------------------------------------------------------

func TestGoZeroMiddlewareChain(t *testing.T) {
	src := `
import "github.com/zeromicro/go-zero/rest"
func s() {
	rest.WithMiddleware(corsMiddleware, traceMiddleware)
}`
	ents := extractFull(t, "custom_go_go_zero", fi("server.go", "go", src))
	for name, order := range map[string]string{
		"corsMiddleware":  "0",
		"traceMiddleware": "1",
	} {
		mw := findPattern(ents, name)
		if mw == nil {
			t.Fatalf("missing middleware %q", name)
		}
		if mw.Props["mw_order"] != order {
			t.Errorf("%s: mw_order=%q want %q", name, mw.Props["mw_order"], order)
		}
	}
}

func TestGoZeroWithJwtAuth(t *testing.T) {
	src := `
import "github.com/zeromicro/go-zero/rest"
func s() { rest.WithJwt("secret") }`
	ents := extractFull(t, "custom_go_go_zero", fi("server.go", "go", src))
	mw := findPattern(ents, "rest.WithJwt")
	if mw == nil {
		t.Fatal("missing rest.WithJwt pattern")
	}
	if mw.Props["is_auth"] != "true" || mw.Props["auth_kind"] != "jwt" {
		t.Errorf("WithJwt: is_auth=%q auth_kind=%q", mw.Props["is_auth"], mw.Props["auth_kind"])
	}
	if findPattern(ents, "auth:rest.WithJwt") == nil {
		t.Error("expected dedicated auth:rest.WithJwt pattern")
	}
}

func TestGoZeroRequestValidation(t *testing.T) {
	src := `
import "github.com/zeromicro/go-zero/rest/httpx"
type LoginRequest struct {
	Username string ` + "`json:\"username\" validate:\"required,min=3\"`" + `
}
func h(r *http.Request) { var req LoginRequest; httpx.Parse(r, &req) }`
	ents := extractFull(t, "custom_go_go_zero", fi("login.go", "go", src))
	rule := findPattern(ents, "validation:rule:LoginRequest.Username")
	if rule == nil {
		t.Fatal("expected validate-tag rule for LoginRequest.Username")
	}
	if rule.Props["rules"] != "required,min=3" {
		t.Errorf("rules=%q", rule.Props["rules"])
	}
	if findPattern(ents, "validation:binding:bind_call") == nil {
		t.Error("expected httpx.Parse binding pattern")
	}
}

func TestGoZeroWiringFixture(t *testing.T) {
	ents := extractFull(t, "custom_go_go_zero", fixtureInput(t, "go_zero_server.go", "go"))
	if !hasPatternKind(ents, "middleware") {
		t.Error("fixture: expected middleware patterns")
	}
	if !hasPatternKind(ents, "auth") {
		t.Error("fixture: expected auth pattern (WithJwt)")
	}
	for _, w := range []string{
		"validation:rule:LoginRequest.Username",
		"validation:rule:LoginRequest.Password",
		"validation:binding:bind_call",
	} {
		if findPattern(ents, w) == nil {
			t.Errorf("fixture: expected %q", w)
		}
	}
}

func TestGoZeroWiringNoMatch(t *testing.T) {
	ents := extractFull(t, "custom_go_go_zero", fi("util.go", "go", `package util
func Add(a, b int) int { return a + b }`))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// huma
// ---------------------------------------------------------------------------

func TestHumaMiddlewareChain(t *testing.T) {
	src := `
import "github.com/danielgtaylor/huma/v2"
func r(api huma.API) { api.UseMiddleware(authMw, logMw) }`
	ents := extractFull(t, "custom_go_huma", fi("api.go", "go", src))
	for name, order := range map[string]string{"authMw": "0", "logMw": "1"} {
		mw := findPattern(ents, name)
		if mw == nil {
			t.Fatalf("missing middleware %q", name)
		}
		if mw.Props["mw_order"] != order {
			t.Errorf("%s: mw_order=%q want %q", name, mw.Props["mw_order"], order)
		}
	}
}

func TestHumaSecurityRequirement(t *testing.T) {
	src := `
import "github.com/danielgtaylor/huma/v2"
func r(api huma.API) {
	huma.Register(api, huma.Operation{
		Method: "GET", Path: "/me",
		Security: []map[string][]string{{"bearer": {}}},
	}, handler)
}`
	ents := extractFull(t, "custom_go_huma", fi("api.go", "go", src))
	au := findPattern(ents, "auth:requirement:bearer")
	if au == nil {
		t.Fatal("expected auth:requirement:bearer")
	}
	if au.Props["auth_kind"] != "jwt" || au.Props["auth_source"] != "operation_security" {
		t.Errorf("auth_kind=%q auth_source=%q", au.Props["auth_kind"], au.Props["auth_source"])
	}
}

func TestHumaSecurityScheme(t *testing.T) {
	src := `
import "github.com/danielgtaylor/huma/v2"
func r(api huma.API) {
	api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {Type: "http", Scheme: "bearer"},
	}
}`
	ents := extractFull(t, "custom_go_huma", fi("api.go", "go", src))
	au := findPattern(ents, "auth:scheme:bearer")
	if au == nil {
		t.Fatal("expected auth:scheme:bearer")
	}
	if au.Props["auth_source"] != "security_scheme" {
		t.Errorf("auth_source=%q", au.Props["auth_source"])
	}
}

func TestHumaRequestValidation(t *testing.T) {
	src := `
import "github.com/danielgtaylor/huma/v2"
type CreateUserInput struct {
	Body struct {
		Name string ` + "`json:\"name\" required:\"true\" minLength:\"2\"`" + `
	}
}`
	ents := extractFull(t, "custom_go_huma", fi("api.go", "go", src))
	if findPattern(ents, "validation:rule:CreateUserInput.Name") == nil {
		t.Error("expected huma validation rule for CreateUserInput.Name")
	}
}

func TestHumaWiringFixture(t *testing.T) {
	ents := extractFull(t, "custom_go_huma", fixtureInput(t, "huma_security.go", "go"))
	if !hasPatternKind(ents, "middleware") {
		t.Error("fixture: expected middleware patterns")
	}
	if findPattern(ents, "auth:requirement:bearer") == nil {
		t.Error("fixture: expected operation security requirement")
	}
	if findPattern(ents, "auth:scheme:bearer") == nil {
		t.Error("fixture: expected security scheme")
	}
	for _, w := range []string{
		"validation:rule:CreateUserInput.Name",
		"validation:rule:CreateUserInput.Email",
		"validation:rule:CreateUserInput.Age",
	} {
		if findPattern(ents, w) == nil {
			t.Errorf("fixture: expected %q", w)
		}
	}
}

func TestHumaWiringNoMatch(t *testing.T) {
	ents := extractFull(t, "custom_go_huma", fi("util.go", "go", `package util
func Add(a, b int) int { return a + b }`))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}
