package elixir_test

// ---------------------------------------------------------------------------
// Plug extractor tests
// ---------------------------------------------------------------------------

import "testing"

func TestPlugRouter(t *testing.T) {
	src := `
defmodule MyApp.Router do
  use Plug.Router

  plug :match
  plug :dispatch

  get "/hello", do: send_resp(conn, 200, "Hello!")
  post "/users", do: send_resp(conn, 201, "Created")
  match _ , do: send_resp(conn, 404, "Not found")
end
`
	ents := extract(t, "custom_elixir_plug", fi("router.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Component", "MyApp.Router") {
		t.Error("expected MyApp.Router router component")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "plug::match") {
		t.Error("expected plug::match middleware pattern")
	}
	if !containsEntity(ents, "SCOPE.Operation", "GET /hello") {
		t.Error("expected GET /hello route")
	}
	if !containsEntity(ents, "SCOPE.Operation", "POST /users") {
		t.Error("expected POST /users route")
	}
}

func TestPlugBuilder(t *testing.T) {
	src := `
defmodule MyApp.Pipeline do
  use Plug.Builder

  plug Plug.Logger
  plug :authenticate
  plug MyApp.AuthPlug
end
`
	ents := extract(t, "custom_elixir_plug", fi("pipeline.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Component", "MyApp.Pipeline") {
		t.Error("expected MyApp.Pipeline builder component")
	}
}

// TestPlugBuilderAuthChain asserts the ordered Plug.Builder plug chain is
// captured with per-plug order indices and that the Guardian auth plug is
// classified with the right provider + method.
func TestPlugBuilderAuthChain(t *testing.T) {
	src := `
defmodule MyApp.AuthPipeline do
  use Plug.Builder

  plug Plug.Logger
  plug Guardian.Plug.VerifyHeader
  plug Guardian.Plug.EnsureAuthenticated
  plug MyApp.LoadCurrentUser
end
`
	ents := extract(t, "custom_elixir_plug", fi("auth_pipeline.ex", "elixir", src))

	logger := findEntity(ents, "SCOPE.Pattern", "plug:Plug.Logger")
	if logger == nil {
		t.Fatal("expected plug:Plug.Logger middleware")
	}
	if got := logger.Props["plug_order"]; got != "0" {
		t.Errorf("expected Plug.Logger order 0, got %q", got)
	}
	if logger.Props["auth"] == "true" {
		t.Error("Plug.Logger must not be classified as auth")
	}

	guard := findEntity(ents, "SCOPE.Pattern", "plug:Guardian.Plug.EnsureAuthenticated")
	if guard == nil {
		t.Fatal("expected Guardian.Plug.EnsureAuthenticated middleware")
	}
	if got := guard.Props["plug_order"]; got != "2" {
		t.Errorf("expected EnsureAuthenticated order 2, got %q", got)
	}
	if guard.Props["auth"] != "true" {
		t.Error("expected auth=true on Guardian.Plug.EnsureAuthenticated")
	}
	if got := guard.Props["auth_provider"]; got != "guardian" {
		t.Errorf("expected auth_provider guardian, got %q", got)
	}
	if got := guard.Props["auth_method"]; got != "jwt" {
		t.Errorf("expected auth_method jwt, got %q", got)
	}
}

func TestPlugCallImpl(t *testing.T) {
	src := `
defmodule MyApp.AuthPlug do
  @behaviour Plug

  def init(opts), do: opts

  def call(conn, opts) do
    case get_session(conn, :user_id) do
      nil -> conn |> send_resp(401, "Unauthorized") |> halt()
      _   -> conn
    end
  end
end
`
	ents := extract(t, "custom_elixir_plug", fi("auth_plug.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Operation", "call") {
		t.Error("expected Plug.call/2 operation")
	}
}

func TestPlugForward(t *testing.T) {
	src := `
defmodule MyApp.Router do
  use Plug.Router

  forward "/api", to: MyApp.API.Router
  forward "/admin", to: MyApp.Admin.Router
end
`
	ents := extract(t, "custom_elixir_plug", fi("router.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Operation", "forward:/api") {
		t.Error("expected forward:/api entity")
	}
	if !containsEntity(ents, "SCOPE.Operation", "forward:/admin") {
		t.Error("expected forward:/admin entity")
	}
}

func TestPlugNoMatch(t *testing.T) {
	src := `defmodule MyApp.Helper do\n  def add(a, b), do: a + b\nend`
	ents := extract(t, "custom_elixir_plug", fi("helper.ex", "elixir", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities from plain module, got %d", len(ents))
	}
}
