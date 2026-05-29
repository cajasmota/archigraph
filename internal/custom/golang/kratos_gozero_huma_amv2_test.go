package golang_test

import "testing"

// Tests for the auth / middleware / request_validation surfaces added to the
// kratos, go-zero and huma custom extractors (#3255). These complement the
// routing tests in kratos_gozero_test.go / hertz_huma_test.go.

// findPattern returns the first SCOPE.Pattern entity with the given name.
func findPattern(ents []fullEntity, name string) *fullEntity {
	for i := range ents {
		if ents[i].Kind == "SCOPE.Pattern" && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

// hasPatternKind reports whether any SCOPE.Pattern carries pattern_kind==kind.
func hasPatternKind(ents []fullEntity, kind string) bool {
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Props["pattern_kind"] == kind {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// kratos: middleware + auth (jwt) + request_validation (protoc-gen-validate)
// ---------------------------------------------------------------------------

func TestKratosMiddlewareChain(t *testing.T) {
	src := `package server
import "github.com/go-kratos/kratos/v2/transport/http"
func New() *http.Server {
	return http.NewServer(http.Middleware(recovery.Recovery(), jwt.Server(kf)))
}`
	ents := extractFull(t, "custom_go_kratos", fi("server.go", "go", src))
	rec := findPattern(ents, "recovery.Recovery()")
	if rec == nil || rec.Props["pattern_kind"] != "middleware" {
		t.Fatalf("expected recovery middleware pattern, got %+v", rec)
	}
	if rec.Props["mw_order"] != "0" {
		t.Errorf("expected recovery at order 0, got %q", rec.Props["mw_order"])
	}
	jw := findPattern(ents, "jwt.Server(kf)")
	if jw == nil || jw.Props["is_auth"] != "true" || jw.Props["auth_kind"] != "jwt" {
		t.Fatalf("expected jwt auth middleware, got %+v", jw)
	}
}

func TestKratosAuthPattern(t *testing.T) {
	src := `package server
import "github.com/go-kratos/kratos/v2/transport/http"
func New() *http.Server {
	return http.NewServer(http.Middleware(jwt.Server(kf)))
}`
	ents := extractFull(t, "custom_go_kratos", fi("server.go", "go", src))
	au := findPattern(ents, "auth:jwt.Server")
	if au == nil || au.Props["pattern_kind"] != "auth" || au.Props["auth_kind"] != "jwt" {
		t.Fatalf("expected dedicated jwt auth pattern, got %+v", au)
	}
}

func TestKratosSelectorServerMiddleware(t *testing.T) {
	src := `package server
import "github.com/go-kratos/kratos/v2/middleware/selector"
func wire(srv *http.Server) {
	srv.Use(selector.Server(jwt.Server(kf)).Match(m).Build())
}`
	ents := extractFull(t, "custom_go_kratos", fi("server.go", "go", src))
	jw := findPattern(ents, "jwt.Server(kf)")
	if jw == nil || jw.Props["middleware_form"] != "selector_server" {
		t.Fatalf("expected selector_server middleware, got %+v", jw)
	}
}

func TestKratosRequestValidation(t *testing.T) {
	src := `package server
import "github.com/go-kratos/kratos/v2/transport/http"
func (m *CreateUserRequest) Validate() error { return nil }
func h(in *CreateUserRequest) error {
	if err := in.Validate(); err != nil { return err }
	return nil
}`
	ents := extractFull(t, "custom_go_kratos", fi("svc.go", "go", src))
	rule := findPattern(ents, "validation:rule:CreateUserRequest.Validate")
	if rule == nil || rule.Props["validation_kind"] != "rule" ||
		rule.Props["validation_subtype"] != "protoc_gen_validate" {
		t.Fatalf("expected PGV validation rule, got %+v", rule)
	}
	if !hasBinding(ents, "validation") {
		t.Error("expected a validation binding call entity")
	}
}

func TestKratosAMVFixture(t *testing.T) {
	ents := extractFull(t, "custom_go_kratos", fixtureFile(t, "kratos_server.go"))
	if !hasPatternKind(ents, "middleware") {
		t.Error("fixture: expected middleware patterns")
	}
	if !hasPatternKind(ents, "auth") {
		t.Error("fixture: expected auth patterns")
	}
	if findPattern(ents, "validation:rule:CreateUserRequest.Validate") == nil {
		t.Error("fixture: expected CreateUserRequest.Validate PGV rule")
	}
}

// ---------------------------------------------------------------------------
// go-zero: middleware + auth (rest.WithJwt) + request_validation (httpx.Parse)
// ---------------------------------------------------------------------------

func TestGoZeroMiddleware(t *testing.T) {
	src := `package handler
import "github.com/zeromicro/go-zero/rest"
func wire(server *rest.Server, mw rest.Middleware) {
	server.Use(mw)
	server.AddRoutes([]rest.Route{{Method: "GET", Path: "/x", Handler: h}}, rest.WithMiddleware(mw))
}`
	ents := extractFull(t, "custom_go_go_zero", fi("routes.go", "go", src))
	if !hasPatternKind(ents, "middleware") {
		t.Fatal("expected middleware patterns from .Use / rest.WithMiddleware")
	}
}

func TestGoZeroJwtAuth(t *testing.T) {
	src := `package handler
import "github.com/zeromicro/go-zero/rest"
func wire(server *rest.Server) {
	server.AddRoutes([]rest.Route{{Method: "GET", Path: "/x", Handler: h}}, rest.WithJwt("secret"))
}`
	ents := extractFull(t, "custom_go_go_zero", fi("routes.go", "go", src))
	au := findPattern(ents, "auth:rest.WithJwt")
	if au == nil || au.Props["auth_kind"] != "jwt" || au.Props["auth_form"] != "with_jwt" {
		t.Fatalf("expected rest.WithJwt auth pattern, got %+v", au)
	}
}

func TestGoZeroRequestValidation(t *testing.T) {
	src := `package logic
import "github.com/zeromicro/go-zero/rest/httpx"
type CreateReq struct {
	Name string ` + "`json:\"name\" validate:\"required,min=2\"`" + `
}
func handle(w http.ResponseWriter, r *http.Request) {
	var req CreateReq
	if err := httpx.Parse(r, &req); err != nil { return }
}`
	ents := extractFull(t, "custom_go_go_zero", fi("logic.go", "go", src))
	rule := findPattern(ents, "validation:rule:CreateReq.Name")
	if rule == nil || rule.Props["rules"] != "required,min=2" {
		t.Fatalf("expected CreateReq.Name validate rule, got %+v", rule)
	}
	bind := findPattern(ents, "validation:binding:httpx_parse:req")
	if bind == nil || bind.Props["validation_subtype"] != "httpx_parse" {
		t.Fatalf("expected httpx.Parse binding, got %+v", bind)
	}
}

func TestGoZeroAMVFixture(t *testing.T) {
	ents := extractFull(t, "custom_go_go_zero", fixtureFile(t, "go_zero_server.go"))
	if !hasPatternKind(ents, "middleware") {
		t.Error("fixture: expected middleware patterns")
	}
	if findPattern(ents, "auth:rest.WithJwt") == nil {
		t.Error("fixture: expected rest.WithJwt auth pattern")
	}
	if findPattern(ents, "validation:rule:CreateUserReq.Email") == nil {
		t.Error("fixture: expected CreateUserReq.Email validate rule")
	}
	if findPattern(ents, "validation:binding:httpx_parse:req") == nil {
		t.Error("fixture: expected httpx.Parse binding")
	}
}

// ---------------------------------------------------------------------------
// huma: middleware + auth (Security/SecuritySchemes) + request_validation
// ---------------------------------------------------------------------------

func TestHumaUseMiddleware(t *testing.T) {
	src := `package h
import "github.com/danielgtaylor/huma/v2"
func r(api huma.API) { api.UseMiddleware(logMW, authMW) }`
	ents := extractFull(t, "custom_go_huma", fi("h.go", "go", src))
	first := findPattern(ents, "logMW")
	if first == nil || first.Props["mw_order"] != "0" ||
		first.Props["middleware_form"] != "use_middleware" {
		t.Fatalf("expected logMW middleware at order 0, got %+v", first)
	}
}

func TestHumaOperationSecurity(t *testing.T) {
	src := `package h
import "github.com/danielgtaylor/huma/v2"
func r(api huma.API) {
	huma.Register(api, huma.Operation{
		Method: "POST", Path: "/users",
		Security: []map[string][]string{{"bearer": {"write"}}},
	}, createUser)
}`
	ents := extractFull(t, "custom_go_huma", fi("h.go", "go", src))
	au := findPattern(ents, "auth:POST /users")
	if au == nil || au.Props["auth_kind"] != "jwt" || au.Props["security_scheme"] != "bearer" {
		t.Fatalf("expected per-operation Security auth pattern, got %+v", au)
	}
}

func TestHumaSecuritySchemes(t *testing.T) {
	src := `package h
import "github.com/danielgtaylor/huma/v2"
func cfg(config huma.Config) {
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {Type: "http", Scheme: "bearer"},
		"apiKey": {Type: "apiKey"},
	}
}`
	ents := extractFull(t, "custom_go_huma", fi("h.go", "go", src))
	if findPattern(ents, "auth:scheme:bearer") == nil {
		t.Error("expected bearer security scheme auth pattern")
	}
	ak := findPattern(ents, "auth:scheme:apiKey")
	if ak == nil || ak.Props["auth_kind"] != "api_key" {
		t.Fatalf("expected apiKey scheme with api_key kind, got %+v", ak)
	}
}

func TestHumaRequestValidation(t *testing.T) {
	src := `package h
import "github.com/danielgtaylor/huma/v2"
type CreateInput struct {
	Body struct {
		Name string ` + "`json:\"name\" minLength:\"2\" maxLength:\"50\"`" + `
		Age  int    ` + "`json:\"age\" minimum:\"0\"`" + `
	}
}
func r(api huma.API) {}`
	ents := extractFull(t, "custom_go_huma", fi("h.go", "go", src))
	rule := findPattern(ents, "validation:rule:CreateInput.Name:minLength")
	if rule == nil || rule.Props["constraint"] != "minLength" ||
		rule.Props["constraint_value"] != "2" {
		t.Fatalf("expected minLength schema-tag rule, got %+v", rule)
	}
	if findPattern(ents, "validation:rule:CreateInput.Age:minimum") == nil {
		t.Error("expected Age minimum schema-tag rule")
	}
}

func TestHumaAMVFixture(t *testing.T) {
	ents := extractFull(t, "custom_go_huma", fixtureFile(t, "huma_secured.go"))
	if !hasPatternKind(ents, "middleware") {
		t.Error("fixture: expected middleware patterns")
	}
	if findPattern(ents, "auth:scheme:bearer") == nil {
		t.Error("fixture: expected bearer security scheme")
	}
	if findPattern(ents, "auth:POST /users") == nil {
		t.Error("fixture: expected per-operation Security on POST /users")
	}
	if findPattern(ents, "validation:rule:CreateUserInput.Email:format") == nil {
		t.Error("fixture: expected Email format schema-tag rule")
	}
}

// hasBinding reports whether any validation SCOPE.Pattern is a binding kind.
func hasBinding(ents []fullEntity, _ string) bool {
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Props["validation_kind"] == "binding" {
			return true
		}
	}
	return false
}
