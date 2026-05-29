// Hand-written kratos transport-server wiring fixture (issue #3255). Exercises
// the middleware/auth surface: http.Middleware(...) server option with an
// ordered chain including the jwt.Server auth middleware, a per-operation
// selector.Server(...) scope, and the validate.Validator() request-validation
// middleware install.
package server

import (
	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/http"
)

func NewHTTPServer() *http.Server {
	opts := []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			logging.Server(),
			validate.Validator(),
			selector.Server(jwt.Server(keyFunc)).Match(needAuth).Build(),
		),
	}
	return http.NewServer(opts...)
}
