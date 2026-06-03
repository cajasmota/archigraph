package engine

import "testing"

// phpRLProps runs full synthesis over a Laravel routes file and returns the
// endpoint at "<VERB> <path>".
func phpRLProps(t *testing.T, content, key string) map[string]string {
	t.Helper()
	eps := authProps(t, "php", "routes/api.php", content)
	e, ok := eps[key]
	if !ok {
		keys := make([]string, 0, len(eps))
		for k := range eps {
			keys = append(keys, k)
		}
		t.Fatalf("endpoint %q not synthesised (got: %v)", key, keys)
	}
	return e.Properties
}

// TestLaravelRateLimit_PerRouteLiteralThrottle — the canonical spec case:
// `Route::get('/api/x', …)->middleware('throttle:60,1')` → that endpoint is
// rate_limited=true with the resolved literal rate 60/1min at route scope; a
// sibling plain route is NOT stamped (negative).
func TestLaravelRateLimit_PerRouteLiteralThrottle(t *testing.T) {
	src := `<?php
Route::get('/api/x', [ApiController::class, 'x'])->middleware('throttle:60,1');
Route::get('/api/free', [ApiController::class, 'free']);
`
	x := phpRLProps(t, src, "GET /api/x")
	if x["rate_limited"] != "true" {
		t.Errorf("GET /api/x: rate_limited=%q, want true (props: %v)", x["rate_limited"], x)
	}
	if x["rate_limit"] != "60/1min" {
		t.Errorf("GET /api/x: rate_limit=%q, want 60/1min", x["rate_limit"])
	}
	if x["rate_limit_scope"] != "route" {
		t.Errorf("GET /api/x: rate_limit_scope=%q, want route", x["rate_limit_scope"])
	}
	if x["rate_limit_source"] != "throttle:60,1" {
		t.Errorf("GET /api/x: rate_limit_source=%q, want throttle:60,1", x["rate_limit_source"])
	}

	// Negative: a plain route must NOT be stamped.
	free := phpRLProps(t, src, "GET /api/free")
	if free["rate_limited"] == "true" {
		t.Errorf("GET /api/free: rate_limited=true, want unthrottled (props: %v)", free)
	}
	if free["rate_limit"] != "" || free["rate_limit_source"] != "" {
		t.Errorf("GET /api/free: leaked rate_limit props (props: %v)", free)
	}
}

// TestLaravelRateLimit_PerRouteArrayMiddleware — the throttle token resolves
// even when it is one element of a middleware array alongside a non-throttle
// middleware (auth), and the auth middleware does NOT itself trigger a
// rate-limit stamp on the path.
func TestLaravelRateLimit_PerRouteArrayMiddleware(t *testing.T) {
	src := `<?php
Route::post('/api/upload', [UploadController::class, 'store'])->middleware(['auth', 'throttle:10,1']);
`
	p := phpRLProps(t, src, "POST /api/upload")
	if p["rate_limited"] != "true" {
		t.Errorf("POST /api/upload: rate_limited=%q, want true (props: %v)", p["rate_limited"], p)
	}
	if p["rate_limit"] != "10/1min" {
		t.Errorf("POST /api/upload: rate_limit=%q, want 10/1min", p["rate_limit"])
	}
	if p["rate_limit_source"] != "throttle:10,1" {
		t.Errorf("POST /api/upload: rate_limit_source=%q, want throttle:10,1", p["rate_limit_source"])
	}
}

// TestLaravelRateLimit_MultiMinuteWindow — `throttle:100,5` is 100 requests per
// 5-minute window → rate "100/5min" (window is honored, never normalised away).
func TestLaravelRateLimit_MultiMinuteWindow(t *testing.T) {
	src := `<?php
Route::get('/api/report', [ReportController::class, 'index'])->middleware('throttle:100,5');
`
	p := phpRLProps(t, src, "GET /api/report")
	if p["rate_limit"] != "100/5min" {
		t.Errorf("GET /api/report: rate_limit=%q, want 100/5min", p["rate_limit"])
	}
}

// TestLaravelRateLimit_NamedLimiterHonestPartial — `throttle:api` is a NAMED
// limiter whose limit/window live in a RateLimiter::for('api', …) registration
// (often app/Providers/RouteServiceProvider.php). rate_limited + source resolve;
// the numeric rate MUST be omitted (never fabricated).
func TestLaravelRateLimit_NamedLimiterHonestPartial(t *testing.T) {
	src := `<?php
Route::middleware('throttle:api')->get('/api/named', [ApiController::class, 'n']);
Route::get('/api/named2', [ApiController::class, 'n2'])->middleware('throttle:api');
`
	// The second form (chained after the verb route) is the one synthesizeLaravel
	// emits an endpoint for; assert on it.
	p := phpRLProps(t, src, "GET /api/named2")
	if p["rate_limited"] != "true" {
		t.Errorf("GET /api/named2: rate_limited=%q, want true (props: %v)", p["rate_limited"], p)
	}
	if p["rate_limit_source"] != "throttle:api" {
		t.Errorf("GET /api/named2: rate_limit_source=%q, want throttle:api", p["rate_limit_source"])
	}
	if p["rate_limit"] != "" {
		t.Errorf("GET /api/named2: rate_limit=%q, want omitted (named limiter is config-driven honest-partial)", p["rate_limit"])
	}
}

// TestLaravelRateLimit_GroupScope — a Route::group with a throttle middleware
// propagates the throttle to every nested route at "group" scope; a route
// OUTSIDE the group is unaffected (negative).
func TestLaravelRateLimit_GroupScope(t *testing.T) {
	src := `<?php
Route::group(['prefix' => 'admin', 'middleware' => ['throttle:30,1']], function () {
    Route::get('/dash', [AdminController::class, 'dash']);
    Route::post('/save', [AdminController::class, 'save']);
});
Route::get('/public', [PublicController::class, 'index']);
`
	dash := phpRLProps(t, src, "GET /admin/dash")
	if dash["rate_limited"] != "true" {
		t.Errorf("GET /admin/dash: rate_limited=%q, want true (props: %v)", dash["rate_limited"], dash)
	}
	if dash["rate_limit"] != "30/1min" {
		t.Errorf("GET /admin/dash: rate_limit=%q, want 30/1min", dash["rate_limit"])
	}
	if dash["rate_limit_scope"] != "group" {
		t.Errorf("GET /admin/dash: rate_limit_scope=%q, want group", dash["rate_limit_scope"])
	}
	save := phpRLProps(t, src, "POST /admin/save")
	if save["rate_limited"] != "true" || save["rate_limit"] != "30/1min" {
		t.Errorf("POST /admin/save: want group-inherited 30/1min (props: %v)", save)
	}

	// Negative: route outside the throttled group must be unthrottled.
	pub := phpRLProps(t, src, "GET /public")
	if pub["rate_limited"] == "true" {
		t.Errorf("GET /public: rate_limited=true, want unthrottled (props: %v)", pub)
	}
}

// TestLaravelRateLimit_PerRouteWinsOverGroup — when a nested route has its own
// throttle, the per-route ("route" scope) posture takes precedence over the
// group's.
func TestLaravelRateLimit_PerRouteWinsOverGroup(t *testing.T) {
	src := `<?php
Route::group(['middleware' => ['throttle:30,1']], function () {
    Route::get('/strict', [C::class, 'strict'])->middleware('throttle:5,1');
});
`
	p := phpRLProps(t, src, "GET /strict")
	if p["rate_limit"] != "5/1min" {
		t.Errorf("GET /strict: rate_limit=%q, want 5/1min (per-route wins over group)", p["rate_limit"])
	}
	if p["rate_limit_scope"] != "route" {
		t.Errorf("GET /strict: rate_limit_scope=%q, want route", p["rate_limit_scope"])
	}
}

// TestLaravelRateLimit_NonThrottleMiddlewareNegative — a route carrying only
// non-throttle middleware (auth, cache.headers) is NOT rate-limited.
func TestLaravelRateLimit_NonThrottleMiddlewareNegative(t *testing.T) {
	src := `<?php
Route::get('/api/cached', [C::class, 'cached'])->middleware(['auth', 'cache.headers:public;max_age=60']);
`
	p := phpRLProps(t, src, "GET /api/cached")
	if p["rate_limited"] == "true" {
		t.Errorf("GET /api/cached: rate_limited=true, want unthrottled — only auth/cache middleware (props: %v)", p)
	}
}
