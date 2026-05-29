package testdata

// Fixture: gin middleware + auth registration for issue #3213.
// Exercises an ordered multi-arg .Use(...) chain plus a dedicated auth
// middleware (jwt) so the shared detector can prove ordering + auth kind.

import (
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func setupGin(authMw *jwt.GinJWTMiddleware) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), cors.Default())
	r.Use(authMw.MiddlewareFunc())

	api := r.Group("/api")
	api.Use(RequireAuth())
	api.GET("/profile", profile)
	return r
}

func RequireAuth() gin.HandlerFunc { return func(c *gin.Context) {} }
func profile(c *gin.Context)       {}
