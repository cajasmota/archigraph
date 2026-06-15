// huma OpenAPI fixture — huma.Register binds an Operation struct to a
// handler. The handler is the third argument; the Operation literal
// carries Method + Path that we extract.
package humafixture

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type GetUserInput struct {
	ID string `path:"id"`
}

type GetUserOutput struct {
	Body struct {
		ID string `json:"id"`
	}
}

type CreateUserInput struct {
	Body struct {
		Name string `json:"name"`
	}
}

type CreateUserOutput struct {
	Body struct {
		ID string `json:"id"`
	}
}

func register(api huma.API) {
	huma.Register(api, huma.Operation{
		Method:  http.MethodGet,
		Path:    "/users/{id}",
		Summary: "Get user by id",
	}, getUser)

	huma.Register(api, huma.Operation{
		Method:  "POST",
		Path:    "/users",
		Summary: "Create user",
	}, createUser)
}
