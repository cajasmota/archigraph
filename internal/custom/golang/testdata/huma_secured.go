// Huma middleware + auth + validation fixture (#3255).
// Covers API-level middleware (api.UseMiddleware), OpenAPI SecuritySchemes
// registration, a per-operation Security requirement, and request validation
// derived from OpenAPI schema tags on the input struct fields.
package humafixture

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type CreateUserInput struct {
	Body struct {
		Name  string `json:"name" minLength:"2" maxLength:"50" required:"true"`
		Email string `json:"email" format:"email"`
		Age   int    `json:"age" minimum:"0" maximum:"130"`
	}
}

type CreateUserOutput struct {
	Body struct {
		ID string `json:"id"`
	}
}

func configure(config huma.Config) huma.Config {
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
		"apiKey": {Type: "apiKey", In: "header", Name: "X-API-Key"},
	}
	return config
}

func registerSecured(api huma.API) {
	api.UseMiddleware(loggingMiddleware, authMiddleware)

	huma.Register(api, huma.Operation{
		Method:   http.MethodPost,
		Path:     "/users",
		Summary:  "Create user",
		Security: []map[string][]string{{"bearer": {"write"}}},
	}, createUser)
}

func createUser(ctx context.Context, in *CreateUserInput) (*CreateUserOutput, error) {
	return &CreateUserOutput{}, nil
}

func loggingMiddleware(ctx huma.Context, next func(huma.Context)) { next(ctx) }
func authMiddleware(ctx huma.Context, next func(huma.Context))    { next(ctx) }
