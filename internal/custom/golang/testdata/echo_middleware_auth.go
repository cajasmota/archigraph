package testdata

// Fixture: echo middleware + auth for issue #3213. Mixes built-in middleware
// with the echo jwt middleware (echojwt / middleware.JWT) and basic auth.

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func setupEcho() *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger(), middleware.Recover())
	e.Use(middleware.JWT([]byte("secret")))
	e.Use(middleware.BasicAuth(validate))
	return e
}

func validate(u, p string, c echo.Context) (bool, error) { return true, nil }
