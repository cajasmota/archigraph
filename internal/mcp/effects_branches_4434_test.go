package mcp

// effects_branches_4434_test.go — in-pipeline test for the #4434 `branches`
// facet on JS/TS, Java, and Go (generalizes the Python #4423/#4435 flagship).
//
// Per the standing LIVE-VALIDATE rule: this exercises the REAL effects MCP
// handler (handleEffects) end-to-end — resolver, on-disk source read,
// per-language branch analyzer — over representative branchy functions in three
// brace-delimited languages copied into testdata/branches_4434/:
//
//   - user_service.ts     — Express service with env-gate (process.env), early
//     return guards (400/409), and a try/catch that re-throws HttpException(500);
//   - UserController.java  — Spring controller with env-gate (System.getenv),
//     a guard returning ResponseEntity.status(HttpStatus.BAD_REQUEST), and a
//     try/catch returning a 500 ResponseEntity;
//   - user_handler.go      — net/http handler with env-gate (os.Getenv), the
//     dominant `if err != nil { http.Error(..); return }` guard, and a panic.
//
// It asserts per language:
//   - RED-before / GREEN-after: the DEFAULT call (no include) carries NO
//     `branches` key (default output unchanged — no regression), while
//     include="branches" enumerates each branch with the right kind/outcome;
//   - the env-gate's env_var is surfaced (SIGNUP_ENABLED);
//   - HTTP statuses (503/400/409/500/...) are derived into returns.status;
//   - catch outcome classification (raise vs return_value), Go panic → raise.

import (
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

// Prefixed entity IDs — the fixtures' labels collide case-insensitively
// (createUser / CreateUser), so the tests resolve via the unambiguous
// prefixed id (mirroring grafel_inspect's how_to_choose guidance).
const (
	tsEnt   = "polyglot-svc::op_create_user_ts"
	javaEnt = "polyglot-svc::op_create_user_java"
	goEnt   = "polyglot-svc::op_create_user_go"
)

// branches4434Server builds a server whose single repo's Path points at the
// branches_4434 testdata dir, with one Operation entity per fixture spanning
// the whole file.
func branches4434Server(t *testing.T) *Server {
	t.Helper()
	doc := &graph.Document{
		Repo: "polyglot-svc",
		Entities: []graph.Entity{
			{
				ID: "op_create_user_ts", Name: "createUser",
				Kind: "SCOPE.Function", QualifiedName: "users.user_service.createUser",
				SourceFile: "user_service.ts", StartLine: 9, EndLine: 31,
			},
			{
				ID: "op_create_user_java", Name: "UserController.create",
				Kind: "SCOPE.Operation", QualifiedName: "com.example.api.UserController.create",
				SourceFile: "UserController.java", StartLine: 25, EndLine: 47,
			},
			{
				ID: "op_create_user_go", Name: "CreateUser",
				Kind: "SCOPE.Function", QualifiedName: "api.CreateUser",
				SourceFile: "user_handler.go", StartLine: 13, EndLine: 42,
			},
		},
	}
	srv := newTestServer(t, doc)
	abs, err := filepath.Abs(filepath.Join("testdata", "branches_4434"))
	if err != nil {
		t.Fatalf("abs testdata: %v", err)
	}
	srv.State.mu.Lock()
	srv.State.groups["test"].Repos["polyglot-svc"].Path = abs
	srv.State.mu.Unlock()
	return srv
}

// TestEffectsBranches4434_DefaultUnchanged is the RED-before assertion across
// all three languages: a default effects call (no include) must NOT carry a
// `branches` key. Guards the no-regression contract.
func TestEffectsBranches4434_DefaultUnchanged(t *testing.T) {
	srv := branches4434Server(t)
	for _, ent := range []string{tsEnt, javaEnt, goEnt} {
		out := callEffects(t, srv, ent, "")
		if _, present := out["branches"]; present {
			t.Fatalf("%s: default effects output must NOT contain `branches`", ent)
		}
		if _, present := out["branches_supported"]; present {
			t.Fatalf("%s: default output must not advertise branches_supported", ent)
		}
		if out["entity_id"] == nil {
			t.Fatalf("%s: default output lost entity_id", ent)
		}
	}
}

// TestEffectsBranches4434_JSTS is the GREEN-after assertion for JS/TS.
func TestEffectsBranches4434_JSTS(t *testing.T) {
	srv := branches4434Server(t)
	out := callEffects(t, srv, tsEnt, "branches")
	if sup, _ := out["branches_supported"].(bool); !sup {
		t.Fatalf("branches_supported should be true for jsts; got %v", out["branches_supported"])
	}
	branches := branchList(t, out)

	// env-gate: process.env.SIGNUP_ENABLED → 503
	env := findBranch(branches, "SIGNUP_ENABLED")
	if env == nil {
		t.Fatalf("missing the process.env.SIGNUP_ENABLED env-gate: %v", branches)
	}
	if env["kind"] != "env_gate" {
		t.Errorf("env-gate kind=%v; want env_gate", env["kind"])
	}
	if env["env_var"] != "SIGNUP_ENABLED" {
		t.Errorf("env_var=%v; want SIGNUP_ENABLED", env["env_var"])
	}
	assertStatus(t, env, "503")

	// 400 email-required guard
	g400 := findBranch(branches, "email == null")
	if g400 == nil {
		t.Fatalf("missing the email-required 400 guard: %v", branches)
	}
	if g400["outcome"] != "return_value" {
		t.Errorf("400 guard outcome=%v; want return_value", g400["outcome"])
	}
	assertStatus(t, g400, "400")

	// 409 existing-email guard inside the try
	g409 := findBranch(branches, "existing != null")
	if g409 == nil {
		t.Fatalf("missing the 409 existing-email guard: %v", branches)
	}
	assertStatus(t, g409, "409")

	// catch re-throws HttpException(500) → raise, status 500
	cat := findBranch(branches, "catch")
	if cat == nil {
		t.Fatalf("missing the catch handler: %v", branches)
	}
	if cat["kind"] != "except" {
		t.Errorf("catch kind=%v; want except", cat["kind"])
	}
	if cat["outcome"] != "raise" {
		t.Errorf("catch re-throws → outcome should be raise; got %v", cat["outcome"])
	}
	assertStatus(t, cat, "500")
}

// TestEffectsBranches4434_Java is the GREEN-after assertion for Java.
func TestEffectsBranches4434_Java(t *testing.T) {
	srv := branches4434Server(t)
	out := callEffects(t, srv, javaEnt, "branches")
	if sup, _ := out["branches_supported"].(bool); !sup {
		t.Fatalf("branches_supported should be true for java; got %v", out["branches_supported"])
	}
	branches := branchList(t, out)

	// env-gate: System.getenv("SIGNUP_ENABLED") → 503
	env := findBranch(branches, "SIGNUP_ENABLED")
	if env == nil {
		t.Fatalf("missing the System.getenv env-gate: %v", branches)
	}
	if env["kind"] != "env_gate" || env["env_var"] != "SIGNUP_ENABLED" {
		t.Errorf("env-gate=%v; want env_gate/SIGNUP_ENABLED", env)
	}
	assertStatus(t, env, "503")

	// BAD_REQUEST guard → 400 via HttpStatus enum mapping
	g400 := findBranch(branches, "getEmail() == null")
	if g400 == nil {
		t.Fatalf("missing the BAD_REQUEST guard: %v", branches)
	}
	if g400["outcome"] != "return_value" {
		t.Errorf("400 guard outcome=%v; want return_value", g400["outcome"])
	}
	assertStatus(t, g400, "400")

	// CONFLICT guard inside try → 409
	g409 := findBranch(branches, "existsByEmail")
	if g409 == nil {
		t.Fatalf("missing the CONFLICT guard: %v", branches)
	}
	assertStatus(t, g409, "409")

	// catch returns a 500 ResponseEntity → return_value, status 500
	cat := findBranch(branches, "catch")
	if cat == nil {
		t.Fatalf("missing the catch handler: %v", branches)
	}
	if cat["kind"] != "except" {
		t.Errorf("catch kind=%v; want except", cat["kind"])
	}
	if cat["outcome"] != "return_value" {
		t.Errorf("catch returns 500 ResponseEntity → outcome should be return_value; got %v", cat["outcome"])
	}
	assertStatus(t, cat, "500")
}

// TestEffectsBranches4434_Go is the GREEN-after assertion for Go.
func TestEffectsBranches4434_Go(t *testing.T) {
	srv := branches4434Server(t)
	out := callEffects(t, srv, goEnt, "branches")
	if sup, _ := out["branches_supported"].(bool); !sup {
		t.Fatalf("branches_supported should be true for go; got %v", out["branches_supported"])
	}
	branches := branchList(t, out)

	// env-gate: os.Getenv("SIGNUP_ENABLED") → 503
	env := findBranch(branches, "SIGNUP_ENABLED")
	if env == nil {
		t.Fatalf("missing the os.Getenv env-gate: %v", branches)
	}
	if env["kind"] != "env_gate" || env["env_var"] != "SIGNUP_ENABLED" {
		t.Errorf("env-gate=%v; want env_gate/SIGNUP_ENABLED", env)
	}
	assertStatus(t, env, "503")

	// the dominant `if err != nil { http.Error(.., StatusBadRequest); return }`
	errGuard := findBranch(branches, "err != nil")
	if errGuard == nil {
		t.Fatalf("missing the `if err != nil` guard: %v", branches)
	}
	if errGuard["outcome"] != "return_value" {
		t.Errorf("err-guard outcome=%v; want return_value", errGuard["outcome"])
	}
	assertStatus(t, errGuard, "400")

	// validation guard → 409 (http.StatusConflict)
	g409 := findBranch(branches, "dto.Email ==")
	if g409 == nil {
		t.Fatalf("missing the email-required 409 guard: %v", branches)
	}
	assertStatus(t, g409, "409")

	// panic → raise
	panicGuard := findBranch(branches, "w == nil")
	if panicGuard == nil {
		t.Fatalf("missing the panic guard: %v", branches)
	}
	if panicGuard["outcome"] != "raise" {
		t.Errorf("panic guard outcome=%v; want raise", panicGuard["outcome"])
	}
}
