package golang_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"

	_ "github.com/cajasmota/grafel/internal/custom/golang"
)

// Tests for the shared middleware + auth detector (issue #3213), covering the
// four well-templated Go frameworks: gin, echo, fiber, chi.

// fullEntity is the test-local view including properties, since middleware
// ordering + auth classification live in the property bag.
type fullEntity struct {
	Kind, Name string
	Props      map[string]string
}

func extractFull(t *testing.T, name string, file extreg.FileInput) []fullEntity {
	t.Helper()
	e, ok := extreg.Get(name)
	if !ok {
		t.Fatalf("extractor %q not registered", name)
	}
	ents, err := e.Extract(context.Background(), file)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	out := make([]fullEntity, 0, len(ents))
	for _, ent := range ents {
		out = append(out, fullEntity{Kind: ent.Kind, Name: ent.Name, Props: ent.Properties})
	}
	return out
}

func findMW(ents []fullEntity, name string) *fullEntity {
	for i := range ents {
		if ents[i].Kind == "SCOPE.Pattern" && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

func fixtureFile(t *testing.T, name string) extreg.FileInput {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return extreg.FileInput{Path: filepath.Join("testdata", name), Language: "go", Content: content}
}

// ---------------------------------------------------------------------------
// Ordering: a multi-arg .Use(...) chain yields one entity per middleware,
// in registration order, with mw_order 0,1,2,…
// ---------------------------------------------------------------------------

func TestGinMiddlewareOrdering(t *testing.T) {
	src := `r.Use(gin.Logger(), gin.Recovery(), cors.Default())`
	ents := extractFull(t, "custom_go_gin", fi("main.go", "go", src))
	want := []struct {
		name, order string
	}{
		{"gin.Logger()", "0"},
		{"gin.Recovery()", "1"},
		{"cors.Default()", "2"},
	}
	for _, w := range want {
		mw := findMW(ents, w.name)
		if mw == nil {
			t.Fatalf("missing middleware %q", w.name)
		}
		if mw.Props["pattern_kind"] != "middleware" {
			t.Errorf("%s: pattern_kind=%q want middleware", w.name, mw.Props["pattern_kind"])
		}
		if mw.Props["mw_order"] != w.order {
			t.Errorf("%s: mw_order=%q want %q", w.name, mw.Props["mw_order"], w.order)
		}
	}
}

func TestChiMiddlewareOrdering(t *testing.T) {
	src := `r.Use(middleware.RequestID, middleware.Logger, middleware.Recoverer)`
	ents := extractFull(t, "custom_go_chi", fi("main.go", "go", src))
	for name, order := range map[string]string{
		"middleware.RequestID": "0",
		"middleware.Logger":    "1",
		"middleware.Recoverer": "2",
	} {
		mw := findMW(ents, name)
		if mw == nil {
			t.Fatalf("missing middleware %q", name)
		}
		if mw.Props["mw_order"] != order {
			t.Errorf("%s: mw_order=%q want %q", name, mw.Props["mw_order"], order)
		}
		if mw.Props["middleware_name"] != name {
			t.Errorf("%s: middleware_name=%q", name, mw.Props["middleware_name"])
		}
	}
}

// ---------------------------------------------------------------------------
// Auth classification: an auth middleware is flagged is_auth=true with an
// auth_kind, and a dedicated auth:<name> SCOPE.Pattern (pattern_kind=auth)
// is also emitted.
// ---------------------------------------------------------------------------

func TestGinAuthDetection(t *testing.T) {
	src := `r.Use(authMw.MiddlewareFunc())`
	ents := extractFull(t, "custom_go_gin", fi("main.go", "go", src))
	mw := findMW(ents, "authMw.MiddlewareFunc()")
	if mw == nil {
		t.Fatal("missing auth middleware entity")
	}
	if mw.Props["is_auth"] != "true" || mw.Props["auth_kind"] != "auth" {
		t.Errorf("expected is_auth/auth_kind, got %+v", mw.Props)
	}
	if findMW(ents, "auth:authMw.MiddlewareFunc") == nil {
		t.Error("expected dedicated auth:authMw.MiddlewareFunc pattern")
	}
}

func TestEchoJWTAuthKind(t *testing.T) {
	src := `e.Use(middleware.JWT([]byte("secret")))`
	ents := extractFull(t, "custom_go_echo", fi("main.go", "go", src))
	mw := findMW(ents, `middleware.JWT([]byte("secret"))`)
	if mw == nil {
		t.Fatal("missing JWT middleware entity")
	}
	if mw.Props["auth_kind"] != "jwt" {
		t.Errorf("auth_kind=%q want jwt", mw.Props["auth_kind"])
	}
}

func TestFiberPathMountSkipsStringLiteral(t *testing.T) {
	// app.Use("/api", jwtware.New(...)) — the "/api" mount prefix must NOT
	// become a middleware entity, and the jwtware value must be order 0.
	src := `app.Use("/api", jwtware.New(jwtware.Config{}))`
	ents := extractFull(t, "custom_go_fiber", fi("main.go", "go", src))
	if findMW(ents, `"/api"`) != nil {
		t.Error("string-literal mount prefix should not be a middleware entity")
	}
	mw := findMW(ents, "jwtware.New(jwtware.Config{})")
	if mw == nil {
		t.Fatal("missing jwtware middleware entity")
	}
	if mw.Props["mw_order"] != "0" {
		t.Errorf("mw_order=%q want 0 (prefix skipped)", mw.Props["mw_order"])
	}
	if mw.Props["auth_kind"] != "jwt" {
		t.Errorf("auth_kind=%q want jwt", mw.Props["auth_kind"])
	}
}

func TestChiAuthDetection(t *testing.T) {
	src := `r.Use(jwtauth.Authenticator)`
	ents := extractFull(t, "custom_go_chi", fi("main.go", "go", src))
	mw := findMW(ents, "jwtauth.Authenticator")
	if mw == nil {
		t.Fatal("missing jwtauth.Authenticator entity")
	}
	// "jwtauth" classifies as jwt (more specific than the generic "auth").
	if mw.Props["auth_kind"] != "jwt" {
		t.Errorf("auth_kind=%q want jwt", mw.Props["auth_kind"])
	}
	if findMW(ents, "auth:jwtauth.Authenticator") == nil {
		t.Error("expected dedicated auth:jwtauth.Authenticator pattern")
	}
}

// Non-auth middleware must NOT be flagged.
func TestNonAuthMiddlewareNotFlagged(t *testing.T) {
	src := `r.Use(gin.Logger())`
	ents := extractFull(t, "custom_go_gin", fi("main.go", "go", src))
	mw := findMW(ents, "gin.Logger()")
	if mw == nil {
		t.Fatal("missing gin.Logger() entity")
	}
	if mw.Props["is_auth"] == "true" {
		t.Error("logger middleware wrongly flagged as auth")
	}
}

// ---------------------------------------------------------------------------
// Fixture-driven end-to-end checks.
// ---------------------------------------------------------------------------

func TestGinFixtureMiddlewareAuth(t *testing.T) {
	ents := extractFull(t, "custom_go_gin", fixtureFile(t, "gin_middleware_auth.go"))
	for _, n := range []string{"gin.Logger()", "gin.Recovery()", "cors.Default()"} {
		if findMW(ents, n) == nil {
			t.Errorf("fixture: missing middleware %q", n)
		}
	}
	if findMW(ents, "RequireAuth()") == nil {
		t.Error("fixture: missing RequireAuth() middleware")
	}
}

func TestEchoFixtureAuth(t *testing.T) {
	ents := extractFull(t, "custom_go_echo", fixtureFile(t, "echo_middleware_auth.go"))
	jwt := findMW(ents, `middleware.JWT([]byte("secret"))`)
	if jwt == nil || jwt.Props["auth_kind"] != "jwt" {
		t.Errorf("fixture: expected echo JWT auth, got %+v", jwt)
	}
	basic := findMW(ents, "middleware.BasicAuth(validate)")
	if basic == nil || basic.Props["auth_kind"] != "basic" {
		t.Errorf("fixture: expected basic auth, got %+v", basic)
	}
}

func TestFiberFixtureAuth(t *testing.T) {
	ents := extractFull(t, "custom_go_fiber", fixtureFile(t, "fiber_middleware_auth.go"))
	if findMW(ents, "jwtware.New(jwtware.Config{})") == nil {
		t.Error("fixture: missing fiber jwtware middleware")
	}
}

func TestChiFixtureAuth(t *testing.T) {
	ents := extractFull(t, "custom_go_chi", fixtureFile(t, "chi_middleware_auth.go"))
	if findMW(ents, "jwtauth.Verifier(ta)") == nil {
		t.Error("fixture: missing jwtauth.Verifier")
	}
	auth := findMW(ents, "jwtauth.Authenticator")
	if auth == nil || auth.Props["auth_kind"] == "" {
		t.Errorf("fixture: expected jwtauth.Authenticator flagged auth, got %+v", auth)
	}
}
