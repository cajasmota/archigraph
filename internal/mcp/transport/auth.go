package transport

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
)

// ErrUnauthenticated is returned by an Authenticator when a request carries no
// valid credential. The HTTP transport maps it to a 401 response.
var ErrUnauthenticated = errors.New("transport: unauthenticated")

// Identity is the authenticated principal extracted from a request. For the
// shared HTTP transport this is the unit that authorization (ADR-0022
// multi-tenancy: "can user X query group Y?") and audit are keyed on.
//
// PROTOTYPE: only Subject is populated by the stub. A production identity would
// also carry the allowed group set, scopes/roles, and token metadata used for
// rotation/revocation — see the Authorizer TODO in http.go.
type Identity struct {
	// Subject is a stable identifier for the caller (user id, key id, service
	// account). The stub fills this with a placeholder.
	Subject string
}

// Authenticator validates the credential on an incoming HTTP request and
// returns the authenticated Identity. Implementations MUST NOT trust the
// network; they are the only auth control on a transport that crosses a host
// boundary.
//
// This is the pluggable seam called out in ADR-0022: the production
// implementation (static shared token vs per-user API keys vs OAuth2/OIDC vs
// mTLS) and its secret/key backend are an explicit MAINTAINER DECISION. The
// only implementation shipped here is StaticTokenAuthenticator, a STUB for
// reviewing the seam — not a secret store and not a security control.
type Authenticator interface {
	// Authenticate inspects the request and returns the caller's Identity, or
	// ErrUnauthenticated (or another error) when the credential is missing or
	// invalid. It must not mutate the request.
	Authenticate(r *http.Request) (Identity, error)
}

// StaticTokenAuthenticator checks the Authorization: Bearer header against a
// single in-memory token using a constant-time compare.
//
// PROTOTYPE / STUB ONLY. A single shared static token gives NO per-user
// identity, NO selective revocation, and a global blast radius on rotation
// (ADR-0022 "Auth options"). It exists so the middleware seam is testable
// without a real secret backend. DO NOT ship this as the production
// authenticator.
type StaticTokenAuthenticator struct {
	// Token is the expected bearer credential. An empty Token rejects every
	// request (fail-closed) so a misconfigured deployment never accidentally
	// authenticates everyone.
	Token string

	// Subject is the placeholder identity returned on success.
	Subject string
}

// Authenticate implements Authenticator.
func (a StaticTokenAuthenticator) Authenticate(r *http.Request) (Identity, error) {
	if a.Token == "" {
		// Fail closed: no configured token means no one is authenticated.
		return Identity{}, ErrUnauthenticated
	}
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return Identity{}, ErrUnauthenticated
	}
	got := strings.TrimSpace(h[len(prefix):])
	// constant-time compare to avoid leaking length/content via timing.
	if subtle.ConstantTimeCompare([]byte(got), []byte(a.Token)) != 1 {
		return Identity{}, ErrUnauthenticated
	}
	subj := a.Subject
	if subj == "" {
		subj = "stub-subject"
	}
	return Identity{Subject: subj}, nil
}

var _ Authenticator = StaticTokenAuthenticator{}

// identityCtxKey is the unexported context key under which the authenticated
// Identity is stashed for downstream handlers/tools.
type identityCtxKey struct{}

// withIdentity returns a copy of ctx carrying id.
func withIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityCtxKey{}, id)
}

// IdentityFromContext extracts the authenticated Identity injected by the auth
// middleware, if present. Tool handlers running under the HTTP transport can
// use this for per-identity authorization and audit (ADR-0022).
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityCtxKey{}).(Identity)
	return id, ok
}

// authMiddleware wraps next with auth: it runs the Authenticator, writes 401 on
// failure, and on success injects the Identity into the request context before
// delegating. It is the single choke point for authentication on the HTTP
// transport.
func authMiddleware(auth Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := auth.Authenticate(r)
		if err != nil {
			// Generic message: never leak which part of the credential failed.
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// TODO(ADR-0022, maintainer decision): authorization goes here.
		//   Resolve the target group (ADR-0008 routing is network-meaningless;
		//   explicit `group` arg becomes mandatory) and enforce an Authorizer
		//   "CanAccessGroup(id, group)" check (default-DENY) before dispatch.
		//   Also: per-identity RateLimiter, and audit-log (id, group, tool).
		next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), id)))
	})
}
