package golang

import (
	"context"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

// runFiber runs the fiber extractor over src for the rate-limit tests.
func runFiber(t *testing.T, src string) []types.EntityRecord {
	t.Helper()
	ents, err := (&fiberExtractor{}).Extract(context.Background(), extractor.FileInput{
		Path: "main.go", Language: "go", Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("fiber extract: %v", err)
	}
	return ents
}

// TestGinGroupRateLimitXTimeRate — the canonical spec case:
// a `rate.NewLimiter(rate.Limit(5), 1)` binding applied as group middleware →
// routes under the group are rate_limited=true rate="5/s"; an unthrottled route
// is NOT stamped (negative).
func TestGinGroupRateLimitXTimeRate(t *testing.T) {
	src := `package main
import (
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)
func main() {
	r := gin.Default()
	r.GET("/health", healthCheck)
	limiterMw := rate.NewLimiter(rate.Limit(5), 1)
	throttled := r.Group("/api", limiterMw)
	throttled.GET("/me", getMe)
	throttled.POST("/orders", createOrder)
}
`
	ents := runGin(t, src)

	me := findEndpoint(t, ents, "GET /api/me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /api/me: rate_limited=%q, want true (props: %v)", me.Properties["rate_limited"], me.Properties)
	}
	if me.Properties["rate_limit"] != "5/s" {
		t.Errorf("GET /api/me: rate_limit=%q, want 5/s", me.Properties["rate_limit"])
	}
	if me.Properties["rate_limit_scope"] != "group" {
		t.Errorf("GET /api/me: rate_limit_scope=%q, want group", me.Properties["rate_limit_scope"])
	}
	if me.Properties["rate_limit_source"] == "" {
		t.Errorf("GET /api/me: rate_limit_source empty, want evidence symbol")
	}

	orders := findEndpoint(t, ents, "POST /api/orders")
	if orders.Properties["rate_limited"] != "true" {
		t.Errorf("POST /api/orders: rate_limited=%q, want true", orders.Properties["rate_limited"])
	}
	if orders.Properties["rate_limit"] != "5/s" {
		t.Errorf("POST /api/orders: rate_limit=%q, want 5/s", orders.Properties["rate_limit"])
	}

	// Negative: /health (raw engine, no limiter) must NOT be stamped.
	health := findEndpoint(t, ents, "GET /health")
	if health.Properties["rate_limited"] == "true" {
		t.Errorf("GET /health: rate_limited=true, want unthrottled (props: %v)", health.Properties)
	}
}

// TestGinInlineRouteRateLimit — inline route limiter middleware binds to that
// exact route only.
func TestGinInlineRouteRateLimit(t *testing.T) {
	src := `package main
import "github.com/gin-gonic/gin"
func main() {
	r := gin.Default()
	r.GET("/me", RateLimit(), getMe)
	r.GET("/public", getPublic)
}
`
	ents := runGin(t, src)

	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /me: rate_limited=%q, want true", me.Properties["rate_limited"])
	}
	if me.Properties["rate_limit_scope"] != "route" {
		t.Errorf("GET /me: rate_limit_scope=%q, want route", me.Properties["rate_limit_scope"])
	}
	if me.Properties["rate_limit_source"] != "RateLimit" {
		t.Errorf("GET /me: rate_limit_source=%q, want RateLimit", me.Properties["rate_limit_source"])
	}

	pub := findEndpoint(t, ents, "GET /public")
	if pub.Properties["rate_limited"] == "true" {
		t.Errorf("GET /public: rate_limited=true, want unthrottled")
	}
}

// TestGinTollboothSetMax — a tollbooth limiter with SetMax(100) applied as
// engine-wide middleware → every route rate_limited rate="100/s".
func TestGinTollboothSetMax(t *testing.T) {
	src := `package main
import (
	"github.com/gin-gonic/gin"
	"github.com/didip/tollbooth/v7"
)
func main() {
	r := gin.Default()
	lim := tollbooth.NewLimiter(1, nil)
	lim.SetMax(100)
	r.Use(LimitHandler(lim))
	r.GET("/me", getMe)
}
`
	ents := runGin(t, src)

	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /me: rate_limited=%q, want true (props: %v)", me.Properties["rate_limited"], me.Properties)
	}
	if me.Properties["rate_limit"] != "100/s" {
		t.Errorf("GET /me: rate_limit=%q, want 100/s", me.Properties["rate_limit"])
	}
	if me.Properties["rate_limit_scope"] != "engine" {
		t.Errorf("GET /me: rate_limit_scope=%q, want engine", me.Properties["rate_limit_scope"])
	}
}

// TestGinTollboothNewLimiterLiteral — tollbooth.NewLimiter(100, nil) first-arg
// literal resolves the rate directly.
func TestGinTollboothNewLimiterLiteral(t *testing.T) {
	src := `package main
import (
	"github.com/gin-gonic/gin"
	"github.com/didip/tollbooth/v7"
)
func main() {
	r := gin.Default()
	lim := tollbooth.NewLimiter(100, nil)
	r.Use(tollbooth.LimitHandler(lim))
	r.GET("/me", getMe)
}
`
	ents := runGin(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /me: rate_limited=%q, want true", me.Properties["rate_limited"])
	}
	if me.Properties["rate_limit"] != "100/s" {
		t.Errorf("GET /me: rate_limit=%q, want 100/s (props: %v)", me.Properties["rate_limit"], me.Properties)
	}
	if me.Properties["rate_limit_source"] == "" {
		t.Errorf("GET /me: rate_limit_source empty")
	}
}

// TestGoRateLimitNegativeUnapplied — a limiter binding constructed but NEVER
// applied to any route/group/engine produces NO stamp (the spec negative case).
func TestGoRateLimitNegativeUnapplied(t *testing.T) {
	src := `package main
import (
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)
func main() {
	r := gin.Default()
	_ = rate.NewLimiter(rate.Limit(5), 1) // built but never applied
	r.GET("/me", getMe)
}
`
	ents := runGin(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] == "true" {
		t.Errorf("GET /me: rate_limited=true, want unthrottled (unapplied limiter; props: %v)", me.Properties)
	}
}

// TestGoRateLimitNonThrottleUnaffected — a non-throttle middleware (logging)
// must NOT be classified as a rate limiter.
func TestGoRateLimitNonThrottleUnaffected(t *testing.T) {
	src := `package main
import "github.com/gin-gonic/gin"
func main() {
	r := gin.Default()
	r.Use(gin.Logger())
	r.GET("/me", getMe)
}
`
	ents := runGin(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] == "true" {
		t.Errorf("GET /me: rate_limited=true, want unthrottled (logger is not a limiter; props: %v)", me.Properties)
	}
}

// TestEchoRateLimitTrailingMiddleware — echo passes middleware after the
// handler; the throttle binds to that route.
func TestEchoRateLimitTrailingMiddleware(t *testing.T) {
	src := `package main
import "github.com/labstack/echo/v4"
func main() {
	e := echo.New()
	e.GET("/me", getMe, RateLimiterMiddleware())
	e.GET("/public", getPublic)
}
`
	ents := runEcho(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /me: rate_limited=%q, want true (props: %v)", me.Properties["rate_limited"], me.Properties)
	}
	if me.Properties["rate_limit_scope"] != "route" {
		t.Errorf("GET /me: rate_limit_scope=%q, want route", me.Properties["rate_limit_scope"])
	}
	pub := findEndpoint(t, ents, "GET /public")
	if pub.Properties["rate_limited"] == "true" {
		t.Errorf("GET /public: rate_limited=true, want unthrottled")
	}
}

// TestGoRateLimitHonestPartialImported — an applied limiter whose constructor
// lives in another module (no literal rate resolvable) → rate_limited=true with
// rate OMITTED (honest-partial).
func TestGoRateLimitHonestPartialImported(t *testing.T) {
	src := `package main
import "github.com/gin-gonic/gin"
func main() {
	r := gin.Default()
	r.Use(ThrottleMiddleware)
	r.GET("/me", getMe)
}
`
	ents := runGin(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /me: rate_limited=%q, want true", me.Properties["rate_limited"])
	}
	if me.Properties["rate_limit"] != "" {
		t.Errorf("GET /me: rate_limit=%q, want omitted (honest-partial)", me.Properties["rate_limit"])
	}
}

// --- chi (#3628 deepen) ----------------------------------------------------

// TestChiEngineRateLimitTollbooth — chi's dominant idiom: a tollbooth limiter
// applied via the engine-wide `r.Use(tollbooth.LimitHandler(lim))` stack throttles
// every route; the resolved max (NewLimiter first-arg) attaches as the rate.
func TestChiEngineRateLimitTollbooth(t *testing.T) {
	src := `package main
import (
	"github.com/go-chi/chi/v5"
	"github.com/didip/tollbooth/v7"
)
func main() {
	r := chi.NewRouter()
	lim := tollbooth.NewLimiter(50, nil)
	r.Use(tollbooth.LimitHandler(lim))
	r.Get("/me", getMe)
}
`
	ents := runChi(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /me: rate_limited=%q, want true (props: %v)", me.Properties["rate_limited"], me.Properties)
	}
	if me.Properties["rate_limit"] != "50/s" {
		t.Errorf("GET /me: rate_limit=%q, want 50/s", me.Properties["rate_limit"])
	}
	if me.Properties["rate_limit_scope"] != "engine" {
		t.Errorf("GET /me: rate_limit_scope=%q, want engine", me.Properties["rate_limit_scope"])
	}
	if me.Properties["rate_limit_source"] != "tollbooth.LimitHandler" {
		t.Errorf("GET /me: rate_limit_source=%q, want tollbooth.LimitHandler", me.Properties["rate_limit_source"])
	}
}

// TestChiEngineRateLimitUlule — ulule/limiter via `mgin.NewMiddleware(instance)`
// applied engine-wide → every route throttled; rate lives in the (separately
// constructed) limiter.Rate so it is honest-partial (omitted), not fabricated.
func TestChiEngineRateLimitUlule(t *testing.T) {
	src := `package main
import (
	"github.com/go-chi/chi/v5"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/stdlib"
)
func main() {
	r := chi.NewRouter()
	rate, _ := limiter.NewRateFromFormatted("100-H")
	instance := limiter.New(newStore(), rate)
	r.Use(mgin.NewMiddleware(instance).Handler)
	r.Get("/me", getMe)
}
`
	ents := runChi(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /me: rate_limited=%q, want true (props: %v)", me.Properties["rate_limited"], me.Properties)
	}
	if me.Properties["rate_limit_source"] != "mgin.NewMiddleware" {
		t.Errorf("GET /me: rate_limit_source=%q, want mgin.NewMiddleware", me.Properties["rate_limit_source"])
	}
	if me.Properties["rate_limit"] != "" {
		t.Errorf("GET /me: rate_limit=%q, want omitted (honest-partial; rate in limiter.Rate)", me.Properties["rate_limit"])
	}
}

// TestChiRateLimitNegativeNonLimiterUse — a non-limiter `r.Use(middleware.Logger)`
// must NOT stamp a chi route as throttled.
func TestChiRateLimitNegativeNonLimiterUse(t *testing.T) {
	src := `package main
import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)
func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/me", getMe)
}
`
	ents := runChi(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] == "true" {
		t.Errorf("GET /me: rate_limited=true, want unthrottled (logger is not a limiter; props: %v)", me.Properties)
	}
}

// --- fiber (#3628 deepen) --------------------------------------------------

// TestFiberGroupRateLimitXTimeRate — fiber group-scope limiter:
// `api := app.Group("/api", limiterMw)` with `limiterMw := rate.NewLimiter(
// rate.Limit(7), 1)` → routes under the group are rate_limited rate="7/s";
// an ungrouped route is NOT stamped (negative).
func TestFiberGroupRateLimitXTimeRate(t *testing.T) {
	src := `package main
import (
	"github.com/gofiber/fiber/v2"
	"golang.org/x/time/rate"
)
func main() {
	app := fiber.New()
	limiterMw := rate.NewLimiter(rate.Limit(7), 1)
	api := app.Group("/api", limiterMw)
	api.Get("/me", getMe)
	app.Get("/public", getPublic)
}
`
	ents := runFiber(t, src)
	me := findEndpoint(t, ents, "GET /api/me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /api/me: rate_limited=%q, want true (props: %v)", me.Properties["rate_limited"], me.Properties)
	}
	if me.Properties["rate_limit"] != "7/s" {
		t.Errorf("GET /api/me: rate_limit=%q, want 7/s", me.Properties["rate_limit"])
	}
	if me.Properties["rate_limit_scope"] != "group" {
		t.Errorf("GET /api/me: rate_limit_scope=%q, want group", me.Properties["rate_limit_scope"])
	}
	if me.Properties["rate_limit_source"] != "rate.NewLimiter" {
		t.Errorf("GET /api/me: rate_limit_source=%q, want rate.NewLimiter", me.Properties["rate_limit_source"])
	}

	pub := findEndpoint(t, ents, "GET /public")
	if pub.Properties["rate_limited"] == "true" {
		t.Errorf("GET /public: rate_limited=true, want unthrottled (props: %v)", pub.Properties)
	}
}

// TestFiberInlineRateLimit — fiber Title-case inline limiter middleware
// `app.Get("/me", RateLimit(), getMe)` binds the throttle to that route only.
func TestFiberInlineRateLimit(t *testing.T) {
	src := `package main
import "github.com/gofiber/fiber/v2"
func main() {
	app := fiber.New()
	app.Get("/me", RateLimit(), getMe)
	app.Get("/public", getPublic)
}
`
	ents := runFiber(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /me: rate_limited=%q, want true (props: %v)", me.Properties["rate_limited"], me.Properties)
	}
	if me.Properties["rate_limit_scope"] != "route" {
		t.Errorf("GET /me: rate_limit_scope=%q, want route", me.Properties["rate_limit_scope"])
	}
	if me.Properties["rate_limit_source"] != "RateLimit" {
		t.Errorf("GET /me: rate_limit_source=%q, want RateLimit", me.Properties["rate_limit_source"])
	}
	pub := findEndpoint(t, ents, "GET /public")
	if pub.Properties["rate_limited"] == "true" {
		t.Errorf("GET /public: rate_limited=true, want unthrottled")
	}
}

// TestFiberEngineRateLimitTollboothNewLimiterLiteral — fiber engine-wide
// `app.Use(tollbooth.LimitHandler(lim))` with a literal-rate tollbooth limiter.
func TestFiberEngineRateLimitTollboothNewLimiterLiteral(t *testing.T) {
	src := `package main
import (
	"github.com/gofiber/fiber/v2"
	"github.com/didip/tollbooth/v7"
)
func main() {
	app := fiber.New()
	lim := tollbooth.NewLimiter(200, nil)
	app.Use(tollbooth.LimitHandler(lim))
	app.Get("/me", getMe)
}
`
	ents := runFiber(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] != "true" {
		t.Errorf("GET /me: rate_limited=%q, want true (props: %v)", me.Properties["rate_limited"], me.Properties)
	}
	if me.Properties["rate_limit"] != "200/s" {
		t.Errorf("GET /me: rate_limit=%q, want 200/s", me.Properties["rate_limit"])
	}
	if me.Properties["rate_limit_scope"] != "engine" {
		t.Errorf("GET /me: rate_limit_scope=%q, want engine", me.Properties["rate_limit_scope"])
	}
}

// TestFiberRateLimitNegativeNonLimiter — a non-limiter middleware (logger) on a
// fiber engine must NOT stamp the route as throttled.
func TestFiberRateLimitNegativeNonLimiter(t *testing.T) {
	src := `package main
import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)
func main() {
	app := fiber.New()
	app.Use(logger.New())
	app.Get("/me", getMe)
}
`
	ents := runFiber(t, src)
	me := findEndpoint(t, ents, "GET /me")
	if me.Properties["rate_limited"] == "true" {
		t.Errorf("GET /me: rate_limited=true, want unthrottled (logger is not a limiter; props: %v)", me.Properties)
	}
}
