// Router file — registers Gin routes pointing at handlers defined in
// handlers_gin.go. The bug we're fixing (#2678) is that http_endpoint
// synthetic entities were attributing their source_file/start_line to
// THIS file (the registration site) instead of the handler-def file.
package ginecho

import (
	"github.com/gin-gonic/gin"
)

func setupGin() *gin.Engine {
	r := gin.Default()
	r.GET("/users", listUsers)
	r.POST("/users", createUser)
	return r
}
