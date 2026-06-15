package ruby_test

// rails_routes_test.go — value-asserting tests for the deep Rails routes
// extractor (custom_ruby_rails_routes). These assert the EXACT synthesized
// route set (path + method + controller#action handler) — not "≥1 route
// exists" — for resources, singular resource, nested resources, namespace,
// scope, member/collection, only/except, root, match, mount, and concerns.

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

// railsRoute is a (method, path[, handler]) expectation.
type railsRoute struct {
	method  string
	path    string
	handler string // "" → don't assert handler
}

// extractRailsRoutes runs the deep routes extractor and returns full records
// so handler properties + CALLS edges can be asserted.
func extractRailsRoutes(t *testing.T, path, src string) []types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get("custom_ruby_rails_routes")
	if !ok {
		t.Fatal("custom_ruby_rails_routes not registered")
	}
	ents, err := e.Extract(context.Background(), extreg.FileInput{
		Path: path, Language: "ruby", Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return ents
}

// assertRoute fails unless an endpoint with the exact name "METHOD path" exists
// and (when r.handler != "") carries that controller#action handler property.
func assertRoute(t *testing.T, ents []types.EntityRecord, r railsRoute) {
	t.Helper()
	want := r.method + " " + r.path
	for _, e := range ents {
		if e.Kind == "SCOPE.Operation" && e.Subtype == "endpoint" && e.Name == want {
			if r.handler != "" {
				got := e.Properties["handler"]
				if got != r.handler {
					t.Errorf("route %q: handler = %q, want %q", want, got, r.handler)
				}
			}
			return
		}
	}
	t.Errorf("missing route %q (handler=%q)", want, r.handler)
}

// assertNoRoute fails if a route with the exact name exists.
func assertNoRoute(t *testing.T, ents []types.EntityRecord, method, path string) {
	t.Helper()
	want := method + " " + path
	for _, e := range ents {
		if e.Kind == "SCOPE.Operation" && e.Subtype == "endpoint" && e.Name == want {
			t.Errorf("unexpected route %q present", want)
		}
	}
}

// countEndpoints returns the number of endpoint entities.
func countEndpoints(ents []types.EntityRecord) int {
	n := 0
	for _, e := range ents {
		if e.Kind == "SCOPE.Operation" && e.Subtype == "endpoint" {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// resources → 7 RESTful routes, correct paths + methods + handlers
// ---------------------------------------------------------------------------

func TestRailsRoutes_ResourcesSevenRESTful(t *testing.T) {
	src := `
Rails.application.routes.draw do
  resources :photos
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	want := []railsRoute{
		{"GET", "/photos", "photos#index"},
		{"POST", "/photos", "photos#create"},
		{"GET", "/photos/new", "photos#new"},
		{"GET", "/photos/:id", "photos#show"},
		{"GET", "/photos/:id/edit", "photos#edit"},
		{"PATCH", "/photos/:id", "photos#update"},
		{"PUT", "/photos/:id", "photos#update"},
		{"DELETE", "/photos/:id", "photos#destroy"},
	}
	for _, r := range want {
		assertRoute(t, ents, r)
	}
	if n := countEndpoints(ents); n != 8 {
		t.Errorf("resources :photos emitted %d endpoints, want 8 (7 actions, update twice for PUT+PATCH)", n)
	}
}

func TestRailsRoutes_ResourcesHandlerCallsEdge(t *testing.T) {
	src := `
Rails.application.routes.draw do
  resources :photos
end
`
	ents := extractRailsRoutes(t, "myapp/config/routes.rb", src)
	wantRef := "scope:operation:method:ruby:myapp/app/controllers/photos_controller.rb:show"
	for _, e := range ents {
		if e.Name == "GET /photos/:id" {
			found := false
			for _, rel := range e.Relationships {
				if rel.Kind == "CALLS" && rel.ToID == wantRef {
					found = true
				}
			}
			if !found {
				t.Errorf("GET /photos/:id: missing CALLS edge to %q; got %+v", wantRef, e.Relationships)
			}
			if got := e.Properties["handler_file"]; got != "myapp/app/controllers/photos_controller.rb" {
				t.Errorf("handler_file = %q", got)
			}
			return
		}
	}
	t.Fatal("GET /photos/:id not found")
}

// ---------------------------------------------------------------------------
// singular resource → 6 routes, no index, no :id
// ---------------------------------------------------------------------------

func TestRailsRoutes_SingularResource(t *testing.T) {
	src := `
Rails.application.routes.draw do
  resource :profile
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	for _, r := range []railsRoute{
		{"GET", "/profile/new", "profile#new"},
		{"POST", "/profile", "profile#create"},
		{"GET", "/profile", "profile#show"},
		{"GET", "/profile/edit", "profile#edit"},
		{"PATCH", "/profile", "profile#update"},
		{"PUT", "/profile", "profile#update"},
		{"DELETE", "/profile", "profile#destroy"},
	} {
		assertRoute(t, ents, r)
	}
	// No index route, no :id member segment.
	assertNoRoute(t, ents, "GET", "/profile/:id")
}

// ---------------------------------------------------------------------------
// nested resources → /photos/:photo_id/comments… path composition
// ---------------------------------------------------------------------------

func TestRailsRoutes_NestedResources(t *testing.T) {
	src := `
Rails.application.routes.draw do
  resources :photos do
    resources :comments
  end
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	// Parent routes still present.
	assertRoute(t, ents, railsRoute{"GET", "/photos", "photos#index"})
	// Nested comments composed under /photos/:photo_id.
	for _, r := range []railsRoute{
		{"GET", "/photos/:photo_id/comments", "comments#index"},
		{"POST", "/photos/:photo_id/comments", "comments#create"},
		{"GET", "/photos/:photo_id/comments/:id", "comments#show"},
		{"DELETE", "/photos/:photo_id/comments/:id", "comments#destroy"},
	} {
		assertRoute(t, ents, r)
	}
}

func TestRailsRoutes_NestedResourcesSingularize(t *testing.T) {
	// categories → category_id (ies→y), addresses → address_id (es→drop -es).
	src := `
Rails.application.routes.draw do
  resources :categories do
    resources :addresses
  end
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/categories/:category_id/addresses", "addresses#index"})
	assertRoute(t, ents, railsRoute{"GET", "/categories/:category_id/addresses/:id", "addresses#show"})
}

// ---------------------------------------------------------------------------
// namespace → /admin prefix + Admin:: module on handler
// ---------------------------------------------------------------------------

func TestRailsRoutes_Namespace(t *testing.T) {
	src := `
Rails.application.routes.draw do
  namespace :admin do
    resources :users
  end
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/admin/users", "admin/users#index"})
	assertRoute(t, ents, railsRoute{"GET", "/admin/users/:id", "admin/users#show"})
	assertRoute(t, ents, railsRoute{"DELETE", "/admin/users/:id", "admin/users#destroy"})
}

func TestRailsRoutes_NestedNamespace(t *testing.T) {
	src := `
Rails.application.routes.draw do
  namespace :api do
    namespace :v1 do
      resources :tokens, only: [:create]
    end
  end
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"POST", "/api/v1/tokens", "api/v1/tokens#create"})
	assertNoRoute(t, ents, "GET", "/api/v1/tokens")
}

// ---------------------------------------------------------------------------
// scope (path-only) and scope module:
// ---------------------------------------------------------------------------

func TestRailsRoutes_ScopePath(t *testing.T) {
	src := `
Rails.application.routes.draw do
  scope '/v1' do
    get '/health', to: 'health#index'
    resources :items, only: [:index]
  end
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/v1/health", "health#index"})
	assertRoute(t, ents, railsRoute{"GET", "/v1/items", "items#index"})
}

func TestRailsRoutes_ScopeModule(t *testing.T) {
	// scope module: prefixes the controller module but NOT the URL path.
	src := `
Rails.application.routes.draw do
  scope module: :internal do
    resources :reports, only: [:index]
  end
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/reports", "internal/reports#index"})
}

func TestRailsRoutes_ScopePathAndModule(t *testing.T) {
	src := `
Rails.application.routes.draw do
  scope path: '/v2', module: :v2 do
    resources :widgets, only: [:show]
  end
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/v2/widgets/:id", "v2/widgets#show"})
}

// ---------------------------------------------------------------------------
// member / collection blocks
// ---------------------------------------------------------------------------

func TestRailsRoutes_MemberCollection(t *testing.T) {
	src := `
Rails.application.routes.draw do
  resources :photos do
    member do
      get :preview
    end
    collection do
      get :search
    end
  end
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	// member → /photos/:id/preview
	assertRoute(t, ents, railsRoute{"GET", "/photos/:id/preview", "photos#preview"})
	// collection → /photos/search (no :id)
	assertRoute(t, ents, railsRoute{"GET", "/photos/search", "photos#search"})
}

func TestRailsRoutes_MemberCollectionInlineOn(t *testing.T) {
	src := `
Rails.application.routes.draw do
  resources :photos do
    get :preview, on: :member
    get :search, on: :collection
  end
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/photos/:id/preview", "photos#preview"})
	assertRoute(t, ents, railsRoute{"GET", "/photos/search", "photos#search"})
}

// ---------------------------------------------------------------------------
// only: / except: filtering
// ---------------------------------------------------------------------------

func TestRailsRoutes_OnlyFilter(t *testing.T) {
	src := `
Rails.application.routes.draw do
  resources :photos, only: [:index, :show]
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/photos", "photos#index"})
	assertRoute(t, ents, railsRoute{"GET", "/photos/:id", "photos#show"})
	assertNoRoute(t, ents, "POST", "/photos")
	assertNoRoute(t, ents, "DELETE", "/photos/:id")
	if n := countEndpoints(ents); n != 2 {
		t.Errorf("only:[index,show] emitted %d endpoints, want 2", n)
	}
}

func TestRailsRoutes_ExceptFilter(t *testing.T) {
	src := `
Rails.application.routes.draw do
  resources :photos, except: [:destroy, :new, :edit]
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/photos", "photos#index"})
	assertRoute(t, ents, railsRoute{"PATCH", "/photos/:id", "photos#update"})
	assertNoRoute(t, ents, "DELETE", "/photos/:id")
	assertNoRoute(t, ents, "GET", "/photos/new")
	assertNoRoute(t, ents, "GET", "/photos/:id/edit")
}

// ---------------------------------------------------------------------------
// root
// ---------------------------------------------------------------------------

func TestRailsRoutes_Root(t *testing.T) {
	src := `
Rails.application.routes.draw do
  root 'home#index'
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/", "home#index"})
}

func TestRailsRoutes_RootToForm(t *testing.T) {
	src := `
Rails.application.routes.draw do
  root to: 'dashboard#show'
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/", "dashboard#show"})
}

// ---------------------------------------------------------------------------
// explicit verb routes
// ---------------------------------------------------------------------------

func TestRailsRoutes_VerbRoutes(t *testing.T) {
	src := `
Rails.application.routes.draw do
  get '/login', to: 'sessions#new'
  post '/login', to: 'sessions#create'
  delete '/logout', to: 'sessions#destroy'
  patch '/profile', to: 'profiles#update'
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	for _, r := range []railsRoute{
		{"GET", "/login", "sessions#new"},
		{"POST", "/login", "sessions#create"},
		{"DELETE", "/logout", "sessions#destroy"},
		{"PATCH", "/profile", "profiles#update"},
	} {
		assertRoute(t, ents, r)
	}
}

// ---------------------------------------------------------------------------
// match … via:
// ---------------------------------------------------------------------------

func TestRailsRoutes_MatchVia(t *testing.T) {
	src := `
Rails.application.routes.draw do
  match '/search', to: 'search#run', via: [:get, :post]
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"GET", "/search", "search#run"})
	assertRoute(t, ents, railsRoute{"POST", "/search", "search#run"})
}

func TestRailsRoutes_MatchViaAll(t *testing.T) {
	src := `
Rails.application.routes.draw do
  match '/webhook', to: 'hooks#receive', via: :all
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		assertRoute(t, ents, railsRoute{m, "/webhook", "hooks#receive"})
	}
}

// ---------------------------------------------------------------------------
// mount engine
// ---------------------------------------------------------------------------

func TestRailsRoutes_Mount(t *testing.T) {
	src := `
Rails.application.routes.draw do
  mount Sidekiq::Web => '/sidekiq'
  mount Blog::Engine, at: '/blog'
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	assertRoute(t, ents, railsRoute{"MOUNT", "/sidekiq", ""})
	assertRoute(t, ents, railsRoute{"MOUNT", "/blog", ""})
}

// ---------------------------------------------------------------------------
// concern / concerns:
// ---------------------------------------------------------------------------

func TestRailsRoutes_Concerns(t *testing.T) {
	src := `
Rails.application.routes.draw do
  concern :commentable do
    resources :comments, only: [:index, :create]
  end

  resources :photos, concerns: [:commentable]
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	// photos still gets its own RESTful routes.
	assertRoute(t, ents, railsRoute{"GET", "/photos", "photos#index"})
	// concern expands comments under /photos/:photo_id.
	assertRoute(t, ents, railsRoute{"GET", "/photos/:photo_id/comments", "comments#index"})
	assertRoute(t, ents, railsRoute{"POST", "/photos/:photo_id/comments", "comments#create"})
}

// ---------------------------------------------------------------------------
// combined realistic routes.rb
// ---------------------------------------------------------------------------

func TestRailsRoutes_RealisticCombined(t *testing.T) {
	src := `
Rails.application.routes.draw do
  root 'home#index'

  get '/about', to: 'pages#about'

  resources :articles do
    resources :comments, only: [:create, :destroy]
    member do
      post :publish
    end
  end

  namespace :admin do
    resources :users, only: [:index, :show]
  end
end
`
	ents := extractRailsRoutes(t, "config/routes.rb", src)
	for _, r := range []railsRoute{
		{"GET", "/", "home#index"},
		{"GET", "/about", "pages#about"},
		{"GET", "/articles", "articles#index"},
		{"GET", "/articles/:id", "articles#show"},
		{"POST", "/articles/:article_id/comments", "comments#create"},
		{"DELETE", "/articles/:article_id/comments/:id", "comments#destroy"},
		{"POST", "/articles/:id/publish", "articles#publish"},
		{"GET", "/admin/users", "admin/users#index"},
		{"GET", "/admin/users/:id", "admin/users#show"},
	} {
		assertRoute(t, ents, r)
	}
	// comments only:[create,destroy] → no index.
	assertNoRoute(t, ents, "GET", "/articles/:article_id/comments")
}

// ---------------------------------------------------------------------------
// gate: non-routes file emits nothing
// ---------------------------------------------------------------------------

func TestRailsRoutes_GateRejectsNonRoutes(t *testing.T) {
	src := `
class PhotosController < ApplicationController
  def index; end
  resources :photos  # not inside a routes.draw block
end
`
	ents := extractRailsRoutes(t, "app/controllers/photos_controller.rb", src)
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for non-routes file, got %d", len(ents))
	}
}

func TestRailsRoutes_EmptyFile(t *testing.T) {
	ents := extractRailsRoutes(t, "config/routes.rb", "")
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for empty file, got %d", len(ents))
	}
}
