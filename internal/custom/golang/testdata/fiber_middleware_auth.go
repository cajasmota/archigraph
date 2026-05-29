package testdata

// Fixture: fiber middleware + auth for issue #3213. Includes a path-mounted
// .Use("/api", mw) form (the leading string literal must be skipped, not
// treated as a middleware value) plus the fiber jwtware auth middleware.

import (
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func setupFiber() *fiber.App {
	app := fiber.New()
	app.Use(logger.New(), recover.New())
	app.Use("/api", jwtware.New(jwtware.Config{}))
	return app
}
