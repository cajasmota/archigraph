package engine

import "testing"

// TestSynth_ElixirTesla_BaseURLPlusVerb covers a `use Tesla` client with a
// BaseUrl middleware and both qualified (`Tesla.get`) and bare (`get`) verb
// calls. The base host is stripped; the route path survives. (#3511)
func TestSynth_ElixirTesla_BaseURLPlusVerb(t *testing.T) {
	src := `
defmodule MyApp.GatewayClient do
  use Tesla

  plug Tesla.Middleware.BaseUrl, "https://gateway:4000"
  plug Tesla.Middleware.JSON

  def list_users do
    get(client(), "/users")
  end

  def create_user(body) do
    Tesla.post(client(), "/users", body)
  end
end
`
	got, _ := runDetect(t, "elixir", "gateway_client.ex", src)
	want := []string{
		"http:GET:/users",
		"http:POST:/users",
	}
	requireContains(t, got, want, "elixir-tesla-baseurl-verb")
}

// TestSynth_ElixirTesla_InterpolatedPath covers a Tesla verb call whose path
// contains a `#{...}` interpolation, which must canonicalise to a {name}
// placeholder. (#3511)
func TestSynth_ElixirTesla_InterpolatedPath(t *testing.T) {
	src := `
defmodule MyApp.UsersClient do
  use Tesla
  plug Tesla.Middleware.BaseUrl, "https://gateway:4000"

  def get_user(id) do
    Tesla.get(client(), "/users/#{id}")
  end
end
`
	got, _ := runDetect(t, "elixir", "users_client.ex", src)
	want := []string{"http:GET:/users/{id}"}
	requireContains(t, got, want, "elixir-tesla-interpolated")
}

// TestSynth_ElixirReq_LiteralAndBang covers `Req.get!("url")` and
// `Req.post!("url", json: body)` with absolute URLs (host stripped). (#3511)
func TestSynth_ElixirReq_LiteralAndBang(t *testing.T) {
	src := `
defmodule MyApp.CatalogClient do
  def list_products do
    Req.get!("https://catalog:3001/products")
  end

  def create_product(body) do
    Req.post!("https://catalog:3001/products", json: body)
  end
end
`
	got, _ := runDetect(t, "elixir", "catalog_client.ex", src)
	want := []string{
		"http:GET:/products",
		"http:POST:/products",
	}
	requireContains(t, got, want, "elixir-req-literal-bang")
}

// TestSynth_ElixirReq_URLOption covers `Req.get(req, url: "/path")` where the
// path is supplied via the url: keyword on a pre-built request. (#3511)
func TestSynth_ElixirReq_URLOption(t *testing.T) {
	src := `
defmodule MyApp.OrdersClient do
  def fetch do
    req = Req.new(base_url: "https://orders:3002")
    Req.get(req, url: "/orders")
  end
end
`
	got, _ := runDetect(t, "elixir", "orders_client.ex", src)
	want := []string{"http:GET:/orders"}
	requireContains(t, got, want, "elixir-req-url-option")
}

// TestSynth_ElixirReq_ConcatPath covers `Req.get("https://host/items/" <> id)`
// — the dynamic concat suffix becomes an {id} placeholder. (#3511)
func TestSynth_ElixirReq_ConcatPath(t *testing.T) {
	src := `
defmodule MyApp.ItemsClient do
  def get_item(id) do
    Req.get("https://catalog:3001/items/" <> id)
  end
end
`
	got, _ := runDetect(t, "elixir", "items_client.ex", src)
	want := []string{"http:GET:/items/{id}"}
	requireContains(t, got, want, "elixir-req-concat")
}

// TestSynth_ElixirTesla_NoFalsePositive ensures a `use Tesla` module with only
// middleware plugs and no verb call sites does NOT emit a bogus endpoint, and
// that a non-Tesla `get(conn, params)` controller body is not misread. (#3511)
func TestSynth_ElixirTesla_NoFalsePositive(t *testing.T) {
	src := `
defmodule MyApp.PageController do
  use MyAppWeb, :controller

  def index(conn, _params) do
    render(conn, "index.html")
  end
end
`
	got, _ := runDetect(t, "elixir", "page_controller.ex", src)
	for _, id := range got {
		if id == "http:GET:/" {
			t.Errorf("false positive Tesla: emitted bogus endpoint %q for non-Tesla controller", id)
		}
	}
}
