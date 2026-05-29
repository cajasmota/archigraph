// Kratos server-wiring + PGV validation fixture (#3255).
// Hand-written kratos server file: wires the global middleware chain via
// http.Middleware(...) (recovery + jwt auth), a per-route selector via
// selector.Server(...), and enforces protoc-gen-validate Validate() on the
// decoded request. The generated *.pb.validate.go Validate() method def is
// also present so the request_validation rule surface resolves.
package server

import (
	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"
)

func NewHTTPServer() *http.Server {
	srv := http.NewServer(
		http.Middleware(
			recovery.Recovery(),
			jwt.Server(keyFunc),
		),
	)
	srv.Use(selector.Server(jwt.Server(keyFunc)).Match(authMatcher).Build())
	return srv
}

// CreateUserRequest is a protoc-gen-validate message; the generated Validate()
// method enforces the field rules.
func (m *CreateUserRequest) Validate() error {
	return m.validate(false)
}

func (m *CreateUserRequest) ValidateAll() error {
	return m.validate(true)
}

func handleCreate(in *CreateUserRequest) error {
	if err := in.Validate(); err != nil {
		return err
	}
	return nil
}
