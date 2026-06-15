// Handler definitions for the Gin router. These are the lines that should
// own the http_endpoint synthetic entities after #2678 fix.
package ginecho

import (
	"github.com/gin-gonic/gin"
)

// listUsers handles GET /users.
func listUsers(c *gin.Context) {
	c.JSON(200, gin.H{"users": []string{"alice", "bob"}})
}

// createUser handles POST /users.
func createUser(c *gin.Context) {
	c.JSON(201, gin.H{"id": 1})
}
