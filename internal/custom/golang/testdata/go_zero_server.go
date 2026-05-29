// go-zero server-bootstrap + logic-layer wiring fixture (issue #3255).
// Exercises middleware (rest.WithMiddleware + server.Use), first-class JWT auth
// (rest.WithJwt), validate-tag request rules, and the httpx.Parse bind site.
package main

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// LoginRequest is a go-zero request DTO with go-playground validate tags.
type LoginRequest struct {
	Username string `json:"username" validate:"required,min=3"`
	Password string `json:"password" validate:"required,min=8"`
}

func NewServer() *rest.Server {
	server := rest.MustNewServer(rest.RestConf{},
		rest.WithMiddleware(corsMiddleware, traceMiddleware),
		rest.WithJwt("super-secret"),
	)
	server.Use(authMiddleware)
	return server
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := httpx.Parse(r, &req); err != nil {
		httpx.ErrorCtx(r.Context(), w, err)
		return
	}
}
