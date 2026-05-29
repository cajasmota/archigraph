package testdata

// Fixture: chi middleware + auth for issue #3213. Uses the chi/middleware
// built-ins plus a custom Authenticator (jwtauth) auth middleware.

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
)

func setupChi(ta *jwtauth.JWTAuth) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Logger, middleware.Recoverer)
	r.Use(jwtauth.Verifier(ta))
	r.Use(jwtauth.Authenticator)
	return r
}
