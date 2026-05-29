// huma middleware/security/validation wiring fixture (issue #3255). Exercises
// api.UseMiddleware (middleware), a Components.SecuritySchemes registration +
// an operation Security requirement (auth), and input-struct OpenAPI-schema
// validation tags (request_validation).
package api

import (
	"github.com/danielgtaylor/huma/v2"
)

// CreateUserInput is a huma input struct; huma auto-validates each field from
// its OpenAPI-schema tags.
type CreateUserInput struct {
	Body struct {
		Name  string `json:"name" required:"true" minLength:"2" maxLength:"64"`
		Email string `json:"email" format:"email"`
		Age   int    `json:"age" minimum:"0" maximum:"130"`
	}
}

func register(api huma.API) {
	api.UseMiddleware(authMiddleware, loggingMiddleware)

	api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		},
	}

	huma.Register(api, huma.Operation{
		Method:   "POST",
		Path:     "/users",
		Security: []map[string][]string{{"bearer": {}}},
	}, createUser)
}
