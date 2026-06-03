package elixir_test

import "testing"

// ---------------------------------------------------------------------------
// custom_elixir_endpoint_deprecation — endpoint deprecation + api_version
// stamping (#4146, child of #3628). Value-asserting: each test pins the SPECIFIC
// deprecated/deprecated_since/deprecated_replacement/deprecation_source/
// api_version resolved on the SPECIFIC Elixir idiom. A "≥1 entity" check is
// NEVER used. Negatives required (non-deprecated → none; versionless → no
// api_version; non-route @deprecated → none).
// ---------------------------------------------------------------------------

const elixirDepKey = "custom_elixir_endpoint_deprecation"

// findElixirDep returns the first deprecation marker matching deprecation_source.
func findElixirDep(ents []entitySummary, source string) (entitySummary, bool) {
	for _, e := range ents {
		if e.Subtype == "deprecation" && e.Props["deprecation_source"] == source {
			return e, true
		}
	}
	return entitySummary{}, false
}

// TestElixirDep_AttrControllerAction: the flagship case from the ticket — a
// @deprecated "use /api/v2/users" attribute above a Phoenix controller action in
// a v1 scope resolves the full contract (deprecated + replacement + api_version=1
// + source). The version lives on the enclosing scope, NOT the message path.
func TestElixirDep_AttrControllerAction(t *testing.T) {
	src := `
defmodule MyAppWeb.UserController do
  use MyAppWeb, :controller

  scope "/api/v1" do
  end

  @deprecated "use /api/v2/users"
  def index(conn, _params) do
    json(conn, %{users: []})
  end
end
`
	ents := extract(t, elixirDepKey, fi("user_controller.ex", "elixir", src))
	e, ok := findElixirDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if e.Props["deprecated_replacement"] != "/api/v2/users" {
		t.Errorf("deprecated_replacement = %q, want /api/v2/users", e.Props["deprecated_replacement"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1 (from enclosing scope, not the v2 message path)", e.Props["api_version"])
	}
	if e.Props["framework"] != "phoenix" {
		t.Errorf("framework = %q, want phoenix", e.Props["framework"])
	}
	if e.Kind != "SCOPE.Pattern" || e.Subtype != "deprecation" {
		t.Errorf("entity = %s/%s, want SCOPE.Pattern/deprecation", e.Kind, e.Subtype)
	}
}

// TestElixirDep_AttrWithSince: an explicit "since X" in the message resolves
// deprecated_since alongside the replacement.
func TestElixirDep_AttrWithSince(t *testing.T) {
	src := `
defmodule MyAppWeb.OrderController do
  scope "/api/v2" do
  end

  @deprecated "since 2.0 use /api/v3/orders instead"
  def index(conn, _params) do
    json(conn, %{})
  end
end
`
	ents := extract(t, elixirDepKey, fi("order_controller.ex", "elixir", src))
	e, ok := findElixirDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["deprecated_since"] != "2.0" {
		t.Errorf("deprecated_since = %q, want 2.0", e.Props["deprecated_since"])
	}
	if e.Props["deprecated_replacement"] != "/api/v3/orders" {
		t.Errorf("deprecated_replacement = %q, want /api/v3/orders", e.Props["deprecated_replacement"])
	}
	if e.Props["api_version"] != "2" {
		t.Errorf("api_version = %q, want 2", e.Props["api_version"])
	}
}

// TestElixirDep_RouterVerbVersion: api_version resolved from a bare /vN segment
// in a Phoenix router verb literal (`get "/v1/users"`).
func TestElixirDep_RouterVerbVersion(t *testing.T) {
	src := `
defmodule MyAppWeb.Router do
  use MyAppWeb, :router

  # DEPRECATED use /v2 instead
  get "/v1/users", UserController, :index
end
`
	ents := extract(t, elixirDepKey, fi("router.ex", "elixir", src))
	e, ok := findElixirDep(ents, "comment # DEPRECATED")
	if !ok {
		t.Fatalf("expected banner-comment marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1", e.Props["api_version"])
	}
}

// TestElixirDep_DocDeprecated: the @doc deprecated: "<msg>" keyword form fires
// with its own deprecation_source and parses the message identically.
func TestElixirDep_DocDeprecated(t *testing.T) {
	src := `
defmodule MyAppWeb.LegacyController do
  scope "/api/v1" do
  end

  @doc deprecated: "replaced by /api/v2/search"
  def search(conn, _params) do
    json(conn, %{})
  end
end
`
	ents := extract(t, elixirDepKey, fi("legacy_controller.ex", "elixir", src))
	e, ok := findElixirDep(ents, "@doc deprecated:")
	if !ok {
		t.Fatalf("expected @doc deprecated: marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if e.Props["deprecated_replacement"] != "/api/v2/search" {
		t.Errorf("deprecated_replacement = %q, want /api/v2/search", e.Props["deprecated_replacement"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1", e.Props["api_version"])
	}
}

// TestElixirDep_SunsetHeader: a put_resp_header(conn, "sunset", ...) write
// (RFC 8594) in an action body fires the Sunset response-header marker.
func TestElixirDep_SunsetHeader(t *testing.T) {
	src := `
defmodule MyAppWeb.ReportController do
  scope "/api/v1" do
  end

  def index(conn, _params) do
    conn
    |> put_resp_header(conn, "sunset", "Wed, 11 Nov 2026 23:59:59 GMT")
    |> json(%{})
  end
end
`
	ents := extract(t, elixirDepKey, fi("report_controller.ex", "elixir", src))
	e, ok := findElixirDep(ents, "Sunset response header")
	if !ok {
		t.Fatalf("expected Sunset header marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1", e.Props["api_version"])
	}
}

// TestElixirDep_DeprecationHeader: a put_resp_header(conn, "deprecation", ...)
// write fires the Deprecation response-header marker (title-cased source).
func TestElixirDep_DeprecationHeader(t *testing.T) {
	src := `
defmodule MyAppWeb.PingController do
  def show(conn, _params) do
    conn
    |> put_resp_header(conn, "deprecation", "true")
    |> json(%{ok: true})
  end
end
`
	ents := extract(t, elixirDepKey, fi("ping_controller.ex", "elixir", src))
	e, ok := findElixirDep(ents, "Deprecation response header")
	if !ok {
		t.Fatalf("expected Deprecation header marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	// Versionless action → no api_version (honest-partial).
	if v, present := e.Props["api_version"]; present {
		t.Errorf("api_version should be ABSENT on a versionless action; got %q", v)
	}
}

// TestElixirDep_PlugFramework: a raw Plug.Router file (no Phoenix) is labelled
// framework=plug.
func TestElixirDep_PlugFramework(t *testing.T) {
	src := `
defmodule MyApp.Router do
  use Plug.Router

  plug :match
  plug :dispatch

  @deprecated "use /v2 endpoint"
  get "/v1/health" do
    send_resp(conn, 200, "ok")
  end
end
`
	ents := extract(t, elixirDepKey, fi("plug_router.ex", "elixir", src))
	e, ok := findElixirDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["framework"] != "plug" {
		t.Errorf("framework = %q, want plug", e.Props["framework"])
	}
	if e.Props["api_version"] != "1" {
		t.Errorf("api_version = %q, want 1", e.Props["api_version"])
	}
}

// --- Negatives -------------------------------------------------------------

// TestElixirDep_NonDeprecatedNone: a plain controller action with NO deprecation
// marker emits no deprecation entity.
func TestElixirDep_NonDeprecatedNone(t *testing.T) {
	src := `
defmodule MyAppWeb.HealthController do
  scope "/api/v1" do
  end

  def index(conn, _params) do
    json(conn, %{status: "ok"})
  end
end
`
	ents := extract(t, elixirDepKey, fi("health_controller.ex", "elixir", src))
	for _, e := range ents {
		if e.Subtype == "deprecation" {
			t.Fatalf("expected NO deprecation marker on a non-deprecated action; got: %+v", e)
		}
	}
}

// TestElixirDep_VersionlessNoApiVersion: a deprecated action with no /vN segment
// nearby carries deprecated=true but NO api_version (honest-partial).
func TestElixirDep_VersionlessNoApiVersion(t *testing.T) {
	src := `
defmodule MyAppWeb.UserController do
  @deprecated "going away soon"
  def index(conn, _params) do
    json(conn, %{})
  end
end
`
	ents := extract(t, elixirDepKey, fi("user_controller.ex", "elixir", src))
	e, ok := findElixirDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["deprecated"] != "true" {
		t.Errorf("deprecated = %q, want true", e.Props["deprecated"])
	}
	if v, present := e.Props["api_version"]; present {
		t.Errorf("api_version should be ABSENT on a versionless route; got %q", v)
	}
}

// TestElixirDep_NonRouteDeprecatedUnaffected: a @deprecated on a non-route helper
// (no `conn` def, no router verb nearby) does NOT emit a route-deprecation marker.
func TestElixirDep_NonRouteDeprecatedUnaffected(t *testing.T) {
	src := `
defmodule MyApp.Calc do
  @deprecated "use new_add/2 instead"
  def old_add(a, b) do
    a + b
  end

  def new_add(a, b), do: a + b
end
`
	ents := extract(t, elixirDepKey, fi("calc.ex", "elixir", src))
	for _, e := range ents {
		if e.Subtype == "deprecation" {
			t.Fatalf("expected NO route-deprecation marker for non-route @deprecated; got: %+v", e)
		}
	}
}

// TestElixirDep_MessagePathNotMistakenForVersion: when the ONLY version segment
// is the replacement path named in the message (`use /api/v2/users`) and the
// route itself is versionless, no api_version is stamped (the message line is
// excluded from version scanning).
func TestElixirDep_MessagePathNotMistakenForVersion(t *testing.T) {
	src := `
defmodule MyAppWeb.UserController do
  @deprecated "use /api/v2/users"
  def index(conn, _params) do
    json(conn, %{})
  end
end
`
	ents := extract(t, elixirDepKey, fi("user_controller.ex", "elixir", src))
	e, ok := findElixirDep(ents, "@deprecated")
	if !ok {
		t.Fatalf("expected @deprecated marker; got: %+v", ents)
	}
	if e.Props["deprecated_replacement"] != "/api/v2/users" {
		t.Errorf("deprecated_replacement = %q, want /api/v2/users", e.Props["deprecated_replacement"])
	}
	if v, present := e.Props["api_version"]; present {
		t.Errorf("api_version must be ABSENT — the only vN is the message replacement path, not the route; got %q", v)
	}
}

// TestElixirDep_NonElixirSkipped: a non-elixir file is skipped entirely.
func TestElixirDep_NonElixirSkipped(t *testing.T) {
	src := `@deprecated "x" def index(conn, _) do get "/v1" end`
	ents := extract(t, elixirDepKey, fi("router.rb", "ruby", src))
	if len(ents) != 0 {
		t.Fatalf("expected no entities for non-elixir file; got: %+v", ents)
	}
}
