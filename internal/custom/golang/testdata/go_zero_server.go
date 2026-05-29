// go-zero server-wiring + validation fixture (#3255).
// Covers middleware (rest.WithMiddleware + server.Use), built-in JWT auth
// (rest.WithJwt), and request validation via httpx.Parse(...) on a typed
// request struct carrying go-playground `validate:` tags.
package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type CreateUserReq struct {
	Name  string `json:"name" validate:"required,min=2"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=130"`
}

func RegisterMiddleware(server *rest.Server, authMW rest.Middleware) {
	server.Use(authMW)
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodPost, Path: "/users", Handler: createUserHandler},
		},
		rest.WithMiddleware(authMW),
		rest.WithJwt("secret-signing-key"),
	)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateUserReq
	if err := httpx.Parse(r, &req); err != nil {
		httpx.Error(w, err)
		return
	}
}
