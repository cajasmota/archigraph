package elixir_test

import (
	"context"
	"strings"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"

	_ "github.com/cajasmota/grafel/internal/custom/elixir"
)

func fi(path, lang, src string) extreg.FileInput {
	return extreg.FileInput{Path: path, Language: lang, Content: []byte(src)}
}

func extract(t *testing.T, name string, file extreg.FileInput) []entitySummary {
	t.Helper()
	e, ok := extreg.Get(name)
	if !ok {
		t.Fatalf("extractor %q not registered", name)
	}
	ents, err := e.Extract(context.Background(), file)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	var out []entitySummary
	for _, ent := range ents {
		out = append(out, entitySummary{Kind: ent.Kind, Subtype: ent.Subtype, Name: ent.Name, Props: ent.Properties})
	}
	return out
}

type entitySummary struct {
	Kind, Subtype, Name string
	Props               map[string]string
}

func containsEntity(ents []entitySummary, kind, name string) bool {
	for _, e := range ents {
		if e.Kind == kind && e.Name == name {
			return true
		}
	}
	return false
}

// findEntity returns the first entity matching kind+name, or nil.
func findEntity(ents []entitySummary, kind, name string) *entitySummary {
	for i := range ents {
		if ents[i].Kind == kind && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Phoenix
// ---------------------------------------------------------------------------

func TestPhoenixRoute(t *testing.T) {
	src := `
scope "/api", MyAppWeb do
  get "/users", UserController, :index
  post "/users", UserController, :create
  delete "/users/:id", UserController, :delete
end
`
	ents := extract(t, "custom_elixir_phoenix", fi("router.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Operation", "GET /users") {
		t.Error("expected GET /users route")
	}
	if !containsEntity(ents, "SCOPE.Operation", "POST /users") {
		t.Error("expected POST /users route")
	}
}

func TestPhoenixResources(t *testing.T) {
	src := `
scope "/", MyAppWeb do
  resources "/articles", ArticleController
end
`
	ents := extract(t, "custom_elixir_phoenix", fi("router.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Operation", "GET /articles") {
		t.Error("expected GET /articles from resources")
	}
	if !containsEntity(ents, "SCOPE.Operation", "POST /articles") {
		t.Error("expected POST /articles from resources")
	}
}

func TestPhoenixLiveView(t *testing.T) {
	// Phoenix LiveView extractor looks for "use Phoenix.LiveView"
	src := `
defmodule MyAppWeb.CounterLive do
  use Phoenix.LiveView

  def mount(_params, _session, socket), do: {:ok, socket}
  def handle_event("increment", _, socket), do: {:noreply, socket}
end
`
	ents := extract(t, "custom_elixir_phoenix", fi("counter_live.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.UIComponent", "MyAppWeb.CounterLive") {
		t.Error("expected CounterLive UIComponent")
	}
	if !containsEntity(ents, "SCOPE.Operation", "handle_event") {
		t.Error("expected handle_event operation")
	}
}

func TestPhoenixPipeline(t *testing.T) {
	src := `
pipeline :api do
  plug :accepts, ["json"]
  plug :fetch_session
end
`
	ents := extract(t, "custom_elixir_phoenix", fi("router.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Pattern", "pipeline:api") {
		t.Error("expected api pipeline pattern")
	}
}

// TestPhoenixPipelineOrderedPlugs asserts the exact ordered plug chain is
// captured on the pipeline entity (not just the pipeline name).
func TestPhoenixPipelineOrderedPlugs(t *testing.T) {
	src := `
pipeline :browser do
  plug :accepts, ["html"]
  plug :fetch_session
  plug :fetch_live_flash
  plug :protect_from_forgery
  plug :put_secure_browser_headers
end
`
	ents := extract(t, "custom_elixir_phoenix", fi("router.ex", "elixir", src))
	pl := findEntity(ents, "SCOPE.Pattern", "pipeline:browser")
	if pl == nil {
		t.Fatal("expected pipeline:browser pattern")
	}
	if pl.Subtype != "pipeline" {
		t.Errorf("expected subtype pipeline, got %q", pl.Subtype)
	}
	wantChain := ":accepts -> :fetch_session -> :fetch_live_flash -> :protect_from_forgery -> :put_secure_browser_headers"
	if got := pl.Props["plug_chain"]; got != wantChain {
		t.Errorf("plug_chain mismatch:\n got  %q\n want %q", got, wantChain)
	}
	if got := pl.Props["plug_count"]; got != "5" {
		t.Errorf("expected plug_count 5, got %q", got)
	}
	if pl.Props["auth"] == "true" {
		t.Error("browser pipeline (no auth plug) must not be flagged auth=true")
	}
	// Individual middleware entities carry their order index within the pipeline.
	psf := findEntity(ents, "SCOPE.Pattern", "plug::protect_from_forgery")
	if psf == nil {
		t.Fatal("expected plug::protect_from_forgery middleware")
	}
	if got := psf.Props["plug_order"]; got != "3" {
		t.Errorf("expected protect_from_forgery plug_order 3, got %q", got)
	}
	if got := psf.Props["pipeline_name"]; got != "browser" {
		t.Errorf("expected pipeline_name browser, got %q", got)
	}
}

// TestPhoenixGuardianAuthPipeline asserts a Guardian JWT pipeline is detected
// with the correct provider + auth method, and that pipe_through binding
// propagates the auth classification.
func TestPhoenixGuardianAuthPipeline(t *testing.T) {
	src := `
pipeline :auth do
  plug Guardian.Plug.VerifyHeader
  plug Guardian.Plug.EnsureAuthenticated
  plug Guardian.Plug.LoadResource
end

scope "/api", MyAppWeb do
  pipe_through [:api, :auth]
  get "/me", UserController, :show
end
`
	ents := extract(t, "custom_elixir_phoenix", fi("router.ex", "elixir", src))
	pl := findEntity(ents, "SCOPE.Pattern", "pipeline:auth")
	if pl == nil {
		t.Fatal("expected pipeline:auth pattern")
	}
	if pl.Props["auth"] != "true" {
		t.Error("expected auth=true on :auth pipeline")
	}
	if got := pl.Props["auth_provider"]; got != "guardian" {
		t.Errorf("expected auth_provider guardian, got %q", got)
	}
	if got := pl.Props["auth_method"]; got != "jwt" {
		t.Errorf("expected auth_method jwt, got %q", got)
	}
	if got := pl.Props["auth_plug"]; got != "Guardian.Plug.VerifyHeader" {
		t.Errorf("expected auth_plug Guardian.Plug.VerifyHeader, got %q", got)
	}
	wantChain := "Guardian.Plug.VerifyHeader -> Guardian.Plug.EnsureAuthenticated -> Guardian.Plug.LoadResource"
	if got := pl.Props["plug_chain"]; got != wantChain {
		t.Errorf("plug_chain mismatch:\n got  %q\n want %q", got, wantChain)
	}
	// pipe_through binding records ordered pipelines + propagates auth.
	pt := findEntity(ents, "SCOPE.Pattern", "pipe_through:api,auth")
	if pt == nil {
		t.Fatal("expected pipe_through:api,auth pattern")
	}
	if got := pt.Props["pipelines"]; got != "api -> auth" {
		t.Errorf("expected pipelines 'api -> auth', got %q", got)
	}
	if pt.Props["auth"] != "true" || pt.Props["auth_method"] != "jwt" {
		t.Errorf("expected pipe_through to inherit jwt auth, got auth=%q method=%q",
			pt.Props["auth"], pt.Props["auth_method"])
	}
}

// TestGuardianImplModule asserts that a `use Guardian` implementation module
// is recorded as an auth component (method=jwt) carrying its implemented
// Guardian behaviour callbacks. (#3511)
func TestGuardianImplModule(t *testing.T) {
	src := `
defmodule MyApp.Guardian do
  use Guardian, otp_app: :my_app

  def subject_for_token(resource, _claims) do
    {:ok, to_string(resource.id)}
  end

  def resource_from_claims(%{"sub" => id}) do
    {:ok, MyApp.Accounts.get_user!(id)}
  end
end
`
	ents := extract(t, "custom_elixir_phoenix", fi("guardian.ex", "elixir", src))
	g := findEntity(ents, "SCOPE.Component", "MyApp.Guardian")
	if g == nil {
		t.Fatal("expected MyApp.Guardian auth component")
	}
	if got := g.Props["auth_provider"]; got != "guardian" {
		t.Errorf("expected auth_provider guardian, got %q", got)
	}
	if got := g.Props["auth_method"]; got != "jwt" {
		t.Errorf("expected auth_method jwt, got %q", got)
	}
	if got := g.Props["auth"]; got != "true" {
		t.Errorf("expected auth=true, got %q", got)
	}
	cbs := g.Props["guardian_callbacks"]
	if !strings.Contains(cbs, "subject_for_token") || !strings.Contains(cbs, "resource_from_claims") {
		t.Errorf("expected guardian_callbacks to include subject_for_token and resource_from_claims, got %q", cbs)
	}
	if got := g.Props["guardian_callback_count"]; got != "2" {
		t.Errorf("expected guardian_callback_count 2, got %q", got)
	}
}

func TestPhoenixNoMatch(t *testing.T) {
	src := `defmodule MyApp.Helper do\n  def greet, do: "hello"\nend`
	ents := extract(t, "custom_elixir_phoenix", fi("helper.ex", "elixir", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// Ecto
// ---------------------------------------------------------------------------

func TestEctoSchema(t *testing.T) {
	src := `
defmodule MyApp.Post do
  use Ecto.Schema

  schema "posts" do
    field :title, :string
    field :body, :text
    belongs_to :user, MyApp.User
    has_many :comments, MyApp.Comment
  end
end
`
	ents := extract(t, "custom_elixir_ecto", fi("post.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Schema", "posts") {
		t.Error("expected posts schema")
	}
	if !containsEntity(ents, "SCOPE.Component", "belongs_to:user") {
		t.Error("expected belongs_to:user component")
	}
	if !containsEntity(ents, "SCOPE.Component", "has_many:comments") {
		t.Error("expected has_many:comments component")
	}
}

func TestEctoChangeset(t *testing.T) {
	src := `
defmodule MyApp.Post do
  def changeset(post, attrs) do
    post |> cast(attrs, [:title, :body]) |> validate_required([:title])
  end
end
`
	ents := extract(t, "custom_elixir_ecto", fi("post.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Operation", "changeset") {
		t.Error("expected changeset operation")
	}
}

func TestEctoRepo(t *testing.T) {
	src := `
defmodule MyApp.Repo do
  use Ecto.Repo, otp_app: :my_app, adapter: Ecto.Adapters.Postgres
end
`
	ents := extract(t, "custom_elixir_ecto", fi("repo.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Service", "MyApp.Repo") {
		t.Error("expected MyApp.Repo SCOPE.Service")
	}
}

func TestEctoRepoQuery(t *testing.T) {
	src := `
Repo.get(Post, id)
Repo.all(Post)
Repo.insert(%Post{title: "Hello"})
`
	ents := extract(t, "custom_elixir_ecto", fi("controller.ex", "elixir", src))
	if !containsEntity(ents, "SCOPE.Operation", "Repo.get") {
		t.Error("expected Repo.get operation")
	}
	if !containsEntity(ents, "SCOPE.Operation", "Repo.insert") {
		t.Error("expected Repo.insert operation")
	}
}

func TestEctoNoMatch(t *testing.T) {
	src := `defmodule MyApp.Helper do\n  def add(a, b), do: a + b\nend`
	ents := extract(t, "custom_elixir_ecto", fi("helper.ex", "elixir", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}
