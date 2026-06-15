// Echo router — same shape as the Gin router but using labstack/echo.
package ginecho

import (
	"github.com/labstack/echo/v4"
)

func setupEcho() *echo.Echo {
	e := echo.New()
	e.GET("/items", listItems)
	e.POST("/items", createItem)
	return e
}
