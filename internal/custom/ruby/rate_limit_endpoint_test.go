package ruby

import (
	"context"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func runRubyRateLimit(t *testing.T, path, src string) []types.EntityRecord {
	t.Helper()
	ents, err := (&rubyRateLimitExtractor{}).Extract(context.Background(), extractor.FileInput{
		Path: path, Language: "ruby", Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("rate_limit extract: %v", err)
	}
	return ents
}

func rlByHandler(ents []types.EntityRecord) map[string]types.EntityRecord {
	out := map[string]types.EntityRecord{}
	for _, e := range ents {
		if h := e.Properties["controller_action"]; h != "" {
			out[h] = e
		}
	}
	return out
}

func rlByName(ents []types.EntityRecord) map[string]types.EntityRecord {
	out := map[string]types.EntityRecord{}
	for _, e := range ents {
		out[e.Name] = e
	}
	return out
}

// --- Rails 8 ActionController rate_limit --------------------------------------

// TestRailsRateLimitStampsAction — `rate_limit to: 10, within: 1.minute,
// only: [:create]` stamps rate_limited + the resolved 10/60s rate on the
// SessionsController#create endpoint, and ONLY on that action.
func TestRailsRateLimitStampsAction(t *testing.T) {
	src := `class SessionsController < ApplicationController
  rate_limit to: 10, within: 1.minute, only: [:create]

  def new
  end

  def create
  end

  def destroy
  end
end
`
	eps := rlByHandler(runRubyRateLimit(t, "app/controllers/sessions_controller.rb", src))

	e, ok := eps["sessions#create"]
	if !ok {
		t.Fatalf("missing rate-limited op for sessions#create; got %v", eps)
	}
	if e.Properties["rate_limited"] != "true" {
		t.Errorf("rate_limited=%q, want true", e.Properties["rate_limited"])
	}
	if e.Properties["rate_limit"] != "10/60s" {
		t.Errorf("rate_limit=%q, want 10/60s (to:10 within:1.minute)", e.Properties["rate_limit"])
	}
	if e.Properties["rate_limit_scope"] != "route" {
		t.Errorf("rate_limit_scope=%q, want route", e.Properties["rate_limit_scope"])
	}
	if e.Properties["rate_limit_source"] != "rate_limit" {
		t.Errorf("rate_limit_source=%q, want rate_limit", e.Properties["rate_limit_source"])
	}
	// NEGATIVE: only: [:create] must NOT stamp new/destroy.
	if _, leaked := eps["sessions#new"]; leaked {
		t.Errorf("only:[:create] leaked onto sessions#new")
	}
	if _, leaked := eps["sessions#destroy"]; leaked {
		t.Errorf("only:[:create] leaked onto sessions#destroy")
	}
}

// TestRailsRateLimitAllActionsNoScope — no only:/except: → every action stamped.
func TestRailsRateLimitAllActionsNoScope(t *testing.T) {
	src := `class ApiController < ApplicationController
  rate_limit to: 100, within: 1.hour

  def index
  end

  def show
  end
end
`
	eps := rlByHandler(runRubyRateLimit(t, "app/controllers/api_controller.rb", src))
	for _, h := range []string{"api#index", "api#show"} {
		e, ok := eps[h]
		if !ok {
			t.Fatalf("missing %s; got %v", h, eps)
		}
		if e.Properties["rate_limit"] != "100/3600s" {
			t.Errorf("%s rate_limit=%q, want 100/3600s (within:1.hour)", h, e.Properties["rate_limit"])
		}
	}
}

// TestRailsRateLimitExceptScope — except: [:index] protects everything but index.
func TestRailsRateLimitExceptScope(t *testing.T) {
	src := `class WidgetsController < ApplicationController
  rate_limit to: 5, within: 30, except: [:index]

  def index
  end

  def create
  end
end
`
	eps := rlByHandler(runRubyRateLimit(t, "app/controllers/widgets_controller.rb", src))
	if _, leaked := eps["widgets#index"]; leaked {
		t.Errorf("except:[:index] leaked onto widgets#index")
	}
	e, ok := eps["widgets#create"]
	if !ok {
		t.Fatalf("missing widgets#create; got %v", eps)
	}
	// within: 30 is a bare-integer-second literal → 5/30s.
	if e.Properties["rate_limit"] != "5/30s" {
		t.Errorf("rate_limit=%q, want 5/30s (within:30 seconds)", e.Properties["rate_limit"])
	}
}

// TestRailsRateLimitHonestPartialWindow — a config-driven `within:` (non-literal)
// stamps rate_limited but OMITS the numeric rate (never fabricated).
func TestRailsRateLimitHonestPartialWindow(t *testing.T) {
	src := `class UploadsController < ApplicationController
  rate_limit to: 3, within: THROTTLE_WINDOW

  def create
  end
end
`
	eps := rlByHandler(runRubyRateLimit(t, "app/controllers/uploads_controller.rb", src))
	e, ok := eps["uploads#create"]
	if !ok {
		t.Fatalf("missing uploads#create; got %v", eps)
	}
	if e.Properties["rate_limited"] != "true" {
		t.Errorf("rate_limited=%q, want true", e.Properties["rate_limited"])
	}
	if r, present := e.Properties["rate_limit"]; present {
		t.Errorf("rate_limit should be omitted (honest-partial) for config window, got %q", r)
	}
}

// TestRailsRateLimitNegative_PlainBeforeAction — a controller with ONLY a plain
// before_action (no rate_limit) yields no rate-limit ops.
func TestRailsRateLimitNegative_PlainBeforeAction(t *testing.T) {
	src := `class PostsController < ApplicationController
  before_action :set_post

  def index
  end
end
`
	ents := runRubyRateLimit(t, "app/controllers/posts_controller.rb", src)
	if len(ents) != 0 {
		t.Errorf("plain before_action produced %d rate-limit ops, want 0: %v", len(ents), ents)
	}
}

// TestRailsRateLimitNegative_NoToCap — a `rate_limit`-named call without a `to:`
// cap is not the ActionController throttle and must not stamp.
func TestRailsRateLimitNegative_NoToCap(t *testing.T) {
	src := `class ReportsController < ApplicationController
  rate_limit_summary = compute_rate_limit(window: 60)

  def index
  end
end
`
	ents := runRubyRateLimit(t, "app/controllers/reports_controller.rb", src)
	if len(ents) != 0 {
		t.Errorf("rate_limit without to: produced %d ops, want 0: %v", len(ents), ents)
	}
}

// --- rack-attack --------------------------------------------------------------

// TestRackAttackThrottleMarker — `Rack::Attack.throttle("logins/ip", limit: 5,
// period: 1.minute)` emits a marker naming the throttle with limit=5/period=60.
func TestRackAttackThrottleMarker(t *testing.T) {
	src := `class Rack::Attack
  throttle_dummy = 1
end

Rack::Attack.throttle("logins/ip", limit: 5, period: 1.minute) do |req|
  req.ip if req.path == "/login" && req.post?
end
`
	byName := rlByName(runRubyRateLimit(t, "config/initializers/rack_attack.rb", src))
	e, ok := byName["rack_attack_throttle:logins/ip"]
	if !ok {
		t.Fatalf("missing rack-attack throttle marker; got %v", keysOf2(byName))
	}
	if e.Kind != "SCOPE.Pattern" || e.Subtype != "rate_limit" {
		t.Errorf("marker kind/subtype=%s/%s, want SCOPE.Pattern/rate_limit", e.Kind, e.Subtype)
	}
	if e.Properties["rate_limit_name"] != "logins/ip" {
		t.Errorf("rate_limit_name=%q, want logins/ip", e.Properties["rate_limit_name"])
	}
	if e.Properties["limit"] != "5" {
		t.Errorf("limit=%q, want 5", e.Properties["limit"])
	}
	if e.Properties["period"] != "60" {
		t.Errorf("period=%q, want 60 (1.minute)", e.Properties["period"])
	}
	if e.Properties["rate_limit"] != "5/60s" {
		t.Errorf("rate_limit=%q, want 5/60s", e.Properties["rate_limit"])
	}
	if e.Properties["rate_limit_source"] != "rack-attack" {
		t.Errorf("rate_limit_source=%q, want rack-attack", e.Properties["rate_limit_source"])
	}
	if e.Properties["rate_limit_scope"] != "request" {
		t.Errorf("rate_limit_scope=%q, want request (heuristic — honest-partial route binding)", e.Properties["rate_limit_scope"])
	}
}

// TestRackAttackBarePeriodSeconds — `period: 60` (bare integer seconds) resolves.
func TestRackAttackBarePeriodSeconds(t *testing.T) {
	src := `Rack::Attack.throttle("api/ip", limit: 100, period: 60) do |req|
  req.ip
end
`
	byName := rlByName(runRubyRateLimit(t, "config/initializers/rack_attack.rb", src))
	e, ok := byName["rack_attack_throttle:api/ip"]
	if !ok {
		t.Fatalf("missing api/ip throttle; got %v", keysOf2(byName))
	}
	if e.Properties["rate_limit"] != "100/60s" {
		t.Errorf("rate_limit=%q, want 100/60s", e.Properties["rate_limit"])
	}
}

// TestRackAttackNegative_Safelist — a safelist is NOT a throttle: no rate-limit
// entity is emitted (a non-throttle rack-attack rule must not be a limit).
func TestRackAttackNegative_Safelist(t *testing.T) {
	src := `Rack::Attack.safelist("allow-localhost") do |req|
  req.ip == "127.0.0.1"
end

Rack::Attack.blocklist("block-bad-ua") do |req|
  req.user_agent == "BadBot"
end
`
	ents := runRubyRateLimit(t, "config/initializers/rack_attack.rb", src)
	if len(ents) != 0 {
		t.Errorf("safelist/blocklist produced %d rate-limit entities, want 0: %v", len(ents), ents)
	}
}

func keysOf2(m map[string]types.EntityRecord) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
