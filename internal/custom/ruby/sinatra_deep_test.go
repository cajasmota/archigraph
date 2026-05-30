package ruby_test

// sinatra_deep_test.go — value-asserting tests for ruby_sinatra_deep extractor.
// Part of issue #3344.

import (
	"testing"
)

func sinatraDeepExtract(t *testing.T, path, src string) []entitySummary {
	t.Helper()
	return extract(t, "ruby_sinatra_deep", fi(path, "ruby", src))
}

// ---------------------------------------------------------------------------
// Routing — class-based Sinatra::Base
// ---------------------------------------------------------------------------

func TestSinatraDeep_ClassBasedRoutes(t *testing.T) {
	src := `
require 'sinatra/base'

class MyApp < Sinatra::Base
  get '/hello' do
    "Hello!"
  end

  post '/users' do
    User.create(params)
  end

  put '/users/:id' do
    User.find(params[:id]).update(params)
  end

  patch '/posts/:id/publish' do
    Post.find(params[:id]).publish!
  end

  delete '/users/:id' do
    User.find(params[:id]).destroy
  end

  options '/cors' do
    headers 'Access-Control-Allow-Origin' => '*'
  end

  head '/ping' do
  end
end
`
	ents := sinatraDeepExtract(t, "app.rb", src)

	routes := []string{
		"GET /hello",
		"POST /users",
		"PUT /users/:id",
		"PATCH /posts/:id/publish",
		"DELETE /users/:id",
		"OPTIONS /cors",
		"HEAD /ping",
	}
	for _, r := range routes {
		if !containsEntity(ents, "SCOPE.Operation", r) {
			t.Errorf("expected Sinatra route entity %q", r)
		}
	}
}

// ---------------------------------------------------------------------------
// Routing — standalone Sinatra app (no class, require 'sinatra')
// ---------------------------------------------------------------------------

func TestSinatraDeep_StandaloneApp(t *testing.T) {
	src := `
require 'sinatra'

get '/' do
  'Hello World!'
end

post '/items' do
  Item.create(params)
end

get '/items/:id' do
  Item.find(params[:id]).to_json
end
`
	ents := sinatraDeepExtract(t, "app.rb", src)

	routes := []string{
		"GET /",
		"POST /items",
		"GET /items/:id",
	}
	for _, r := range routes {
		if !containsEntity(ents, "SCOPE.Operation", r) {
			t.Errorf("expected standalone Sinatra route %q", r)
		}
	}
}

// ---------------------------------------------------------------------------
// Routing — named params and splat routes
// ---------------------------------------------------------------------------

func TestSinatraDeep_NamedParamRoute(t *testing.T) {
	src := `
require 'sinatra'

get '/users/:id/posts/:post_id' do
  User.find(params[:id]).posts.find(params[:post_id]).to_json
end
`
	ents := sinatraDeepExtract(t, "app.rb", src)
	if !containsEntity(ents, "SCOPE.Operation", "GET /users/:id/posts/:post_id") {
		t.Error("expected GET /users/:id/posts/:post_id route with named params")
	}
}

func TestSinatraDeep_SplatRoute(t *testing.T) {
	src := `
require 'sinatra'

get '/files/*path' do
  send_file params[:path]
end

get '/catch/*' do
  params[:splat].inspect
end
`
	ents := sinatraDeepExtract(t, "app.rb", src)
	if !containsEntity(ents, "SCOPE.Operation", "GET /files/*path") {
		t.Error("expected GET /files/*path splat route")
	}
	if !containsEntity(ents, "SCOPE.Operation", "GET /catch/*") {
		t.Error("expected GET /catch/* splat route")
	}
}

// ---------------------------------------------------------------------------
// Middleware — use Rack::X
// ---------------------------------------------------------------------------

func TestSinatraDeep_RackUse(t *testing.T) {
	src := `
class MyApp < Sinatra::Base
  use Rack::Session::Cookie, secret: 'abc'
  use Rack::Cors
  use Rack::Logger
end
`
	ents := sinatraDeepExtract(t, "app.rb", src)
	if !containsEntity(ents, "SCOPE.Pattern", "sinatra_rack_use:Rack::Session::Cookie") {
		t.Error("expected sinatra_rack_use:Rack::Session::Cookie middleware entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "sinatra_rack_use:Rack::Cors") {
		t.Error("expected sinatra_rack_use:Rack::Cors middleware entity")
	}
}

// ---------------------------------------------------------------------------
// Middleware — helpers do block
// ---------------------------------------------------------------------------

func TestSinatraDeep_HelpersBlock(t *testing.T) {
	src := `
class MyApp < Sinatra::Base
  helpers do
    def current_user
      @current_user ||= User.find(session[:user_id])
    end

    def logged_in?
      !current_user.nil?
    end
  end
end
`
	ents := sinatraDeepExtract(t, "app.rb", src)
	if !containsEntity(ents, "SCOPE.Pattern", "sinatra_helpers_block") {
		t.Error("expected sinatra_helpers_block middleware entity")
	}
}

// ---------------------------------------------------------------------------
// Auth — before { halt 401 unless ... } guard
// ---------------------------------------------------------------------------

func TestSinatraDeep_BeforeHaltGuard(t *testing.T) {
	src := `
class MyApp < Sinatra::Base
  before do
    halt 401 unless logged_in?
  end

  get '/dashboard' do
    "Welcome #{current_user.name}"
  end
end
`
	ents := sinatraDeepExtract(t, "app.rb", src)
	if !containsEntity(ents, "SCOPE.Pattern", "sinatra_auth_guard:halt") {
		t.Error("expected sinatra_auth_guard:halt auth guard entity")
	}
}

// ---------------------------------------------------------------------------
// Auth — protected! helper
// ---------------------------------------------------------------------------

func TestSinatraDeep_ProtectedHelper(t *testing.T) {
	src := `
class MyApp < Sinatra::Base
  helpers do
    def protected!
      return if authorized?
      halt 401
    end

    def authorized?
      @auth ||= Rack::Auth::Basic::Request.new(request.env)
      @auth.provided? && @auth.basic? && @auth.credentials && @auth.credentials == ['user', 'secret']
    end
  end

  get '/admin' do
    protected!
    "Admin area"
  end
end
`
	ents := sinatraDeepExtract(t, "app.rb", src)
	if !containsEntity(ents, "SCOPE.Pattern", "protected!") {
		t.Error("expected protected! auth_guard entity")
	}
}

// ---------------------------------------------------------------------------
// Auth — halt status codes
// ---------------------------------------------------------------------------

func TestSinatraDeep_HaltStatus(t *testing.T) {
	src := `
require 'sinatra'

get '/secret' do
  halt 403 unless current_user.admin?
  "Secret data"
end

post '/items' do
  halt 422, "Invalid params" unless params[:name]
  Item.create(params)
end
`
	ents := sinatraDeepExtract(t, "app.rb", src)
	if !containsEntity(ents, "SCOPE.Pattern", "sinatra_halt:403") {
		t.Error("expected sinatra_halt:403 auth guard entity")
	}
	if !containsEntity(ents, "SCOPE.Pattern", "sinatra_halt:422") {
		t.Error("expected sinatra_halt:422 auth guard entity")
	}
}

// ---------------------------------------------------------------------------
// Validation — sinatra-param gem
// ---------------------------------------------------------------------------

func TestSinatraDeep_SinatraParam(t *testing.T) {
	src := `
require 'sinatra'
require 'sinatra/param'

post '/users' do
  param :name,  String, required: true
  param :email, String, required: true
  param :age,   Integer, min: 18
  User.create(params)
end
`
	ents := sinatraDeepExtract(t, "app.rb", src)
	if !containsEntitySubtype(ents, "SCOPE.Schema", "dto_field", "sinatra_param:name") {
		t.Error("expected sinatra_param:name dto_field entity")
	}
	if !containsEntitySubtype(ents, "SCOPE.Schema", "dto_field", "sinatra_param:email") {
		t.Error("expected sinatra_param:email dto_field entity")
	}
	if !containsEntitySubtype(ents, "SCOPE.Schema", "dto_field", "sinatra_param:age") {
		t.Error("expected sinatra_param:age dto_field entity")
	}
}

// ---------------------------------------------------------------------------
// Testing — rack-test spec linkage
// ---------------------------------------------------------------------------

func TestSinatraDeep_RackTestInclude(t *testing.T) {
	src := `
require 'spec_helper'
require 'rack/test'

RSpec.describe MyApp do
  include Rack::Test::Methods

  def app
    MyApp
  end

  it 'returns hello' do
    get '/hello'
    expect(last_response).to be_ok
    expect(last_response.body).to eq('Hello!')
  end

  it 'creates a user' do
    post '/users', name: 'Alice', email: 'alice@example.com'
    expect(last_response.status).to eq(201)
  end
end
`
	ents := sinatraDeepExtract(t, "spec/app_spec.rb", src)

	// Top-level rack-test inclusion signal
	if !containsEntitySubtype(ents, "SCOPE.Pattern", "test_framework", "rack_test:Rack::Test::Methods") {
		t.Error("expected rack_test:Rack::Test::Methods test_framework entity")
	}

	// App definition inside spec
	if !containsEntitySubtype(ents, "SCOPE.Pattern", "test_framework", "rack_test:app_def") {
		t.Error("expected rack_test:app_def test_framework entity")
	}

	// HTTP call-site entities
	if !containsEntitySubtype(ents, "SCOPE.Operation", "test_call", "rack_test_call:GET /hello") {
		t.Error("expected rack_test_call:GET /hello test_call entity")
	}
	if !containsEntitySubtype(ents, "SCOPE.Operation", "test_call", "rack_test_call:POST /users") {
		t.Error("expected rack_test_call:POST /users test_call entity")
	}
}

func TestSinatraDeep_RackTestMinitest(t *testing.T) {
	src := `
require 'minitest/autorun'
require 'rack/test'

class AppTest < Minitest::Test
  include Rack::Test::Methods

  def app
    Sinatra::Application
  end

  def test_get_index
    get '/'
    assert last_response.ok?
  end

  def test_post_items
    post '/items', name: 'Widget'
    assert_equal 201, last_response.status
  end
end
`
	ents := sinatraDeepExtract(t, "test/app_test.rb", src)

	if !containsEntitySubtype(ents, "SCOPE.Pattern", "test_framework", "rack_test:Rack::Test::Methods") {
		t.Error("expected rack_test:Rack::Test::Methods test_framework entity for Minitest")
	}
	if !containsEntitySubtype(ents, "SCOPE.Operation", "test_call", "rack_test_call:GET /") {
		t.Error("expected rack_test_call:GET / test_call entity")
	}
	if !containsEntitySubtype(ents, "SCOPE.Operation", "test_call", "rack_test_call:POST /items") {
		t.Error("expected rack_test_call:POST /items test_call entity")
	}
}

// ---------------------------------------------------------------------------
// No-match guard
// ---------------------------------------------------------------------------

func TestSinatraDeep_NonSinatraFile(t *testing.T) {
	src := `class User < ApplicationRecord; end`
	ents := sinatraDeepExtract(t, "app/models/user.rb", src)
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for non-Sinatra file, got %d", len(ents))
	}
}

func TestSinatraDeep_EmptyFile(t *testing.T) {
	ents := sinatraDeepExtract(t, "app.rb", "")
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for empty file, got %d", len(ents))
	}
}
