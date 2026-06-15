// Handler definitions for the Echo router.
package ginecho

import (
	"github.com/labstack/echo/v4"
)

// listItems handles GET /items.
func listItems(c echo.Context) error {
	return c.JSON(200, map[string]any{"items": []string{"a", "b"}})
}

// createItem handles POST /items.
func createItem(c echo.Context) error {
	return c.JSON(201, map[string]any{"id": 42})
}
