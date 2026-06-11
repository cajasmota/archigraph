# ADR-0022: Authenticated shared HTTP MCP transport for team deployments

- **Status**: Proposed (EVALUATION — recommendation only; security model needs maintainer sign-off before any production enablement)
- **Date**: 2026-06-11
- **Deciders**: _pending_ (maintainer + security sign-off required — see "Decisions for the maintainer")
- **Issue**: #4296
- **Relates to / tensions with**: ADR-0004 (single MCP process per machine), ADR-0002 (clean-room MCP server in Go), ADR-0017 (single-binary daemon architecture), ADR-0008 (caller-CWD-aware routing)

## Context

Today archigraph exposes its graph over MCP through a **per-machine, local-only**
transport chain (ADR-0004, ADR-0017):

```
Claude Code / agent host
  → spawns `archigraph mcp-bridge`   (short-lived stdio process, one per session)
     stdin/stdout = MCP JSON-RPC 2.0
  → dials the daemon's Unix-domain socket / Windows named pipe
     (internal/daemon/transport)   = daemon JSON-RPC 1.0
  → daemonMCPCallTool → mcp.Server  (single long-lived per-machine process)
```

Two properties of this chain are load-bearing for the evaluation:

1. **The wire is never a network socket.** The bridge↔daemon hop is a Unix
   socket (macOS/Linux) or a named pipe (Windows) — OS-scoped to the local
   user. There is **no authentication layer** anywhere, because the OS file
   permissions on the socket/pipe *are* the trust boundary. Anyone who can open
   the socket is already the same local user.
2. **The MCP server is already transport-agnostic.** `mcp.Server` wraps a
   `mark3labs/mcp-go *MCPServer`; `ServeStdio` is just one way to drive it. The
   same `*MCPServer` is driven over the socket today via `daemonMCPCallTool`.
   The vendored `mcp-go@v0.52.0` already ships `server.StreamableHTTPServer`
   and an SSE server — i.e. an HTTP transport is *available in the dependency
   we already have*, with stateless mode, session management, and a context
   hook for per-request auth.

The request (#4296) is to **evaluate** a third transport: a **shared,
authenticated Streamable-HTTP server** so that one daemon can serve many
engineers (or CI jobs) over the network — "one server, many teammates,
API-key/Bearer auth, stateless mode, session timeout, container packaging."

This is architecturally significant because it **directly tensions with
ADR-0004**. ADR-0004's whole premise is *single process per machine, local
only*. A shared HTTP server is *single process per **team**, network-exposed* —
a different deployment topology with a fundamentally different trust model. We
are not overturning ADR-0004 for the laptop case; we are evaluating an
**additional, opt-in, off-by-default** deployment mode for teams that
explicitly want a central graph service.

## Transport options

| Option | Shape | Fit for shared multi-client | Verdict |
|---|---|---|---|
| **stdio (today)** | one process per session, pipes | None — inherently 1:1, local | Keep for laptop default |
| **Unix socket / named pipe (today)** | local IPC, OS-permission trust | None — single-host, single-user | Keep for the bridge hop |
| **SSE (mcp-go `SSEServer`)** | HTTP GET event stream + POST channel | Works, but SSE is the *older* MCP remote transport; two endpoints, stateful, being superseded | Not recommended as the primary |
| **Streamable HTTP (mcp-go `StreamableHTTPServer`)** | single endpoint, POST request/response, optional SSE upgrade for streaming, **supports stateless mode + session-id manager + per-request context hook** | **Best** — current MCP remote-transport spec, one URL, works behind any reverse proxy, stateless mode removes server-side session affinity (CI-friendly), `WithHTTPContextFunc` gives us a clean per-request auth seam | **Recommended transport if the mode is adopted** |

### Why Streamable HTTP over SSE

`mcp-go` exposes both. Streamable HTTP is the MCP spec's current remote
transport; SSE is retained for backward compatibility. Streamable HTTP gives
us a **single endpoint**, a **stateless mode** (`WithStateLess(true)` — no
server-side session table; each request is self-contained, which is exactly
what a horizontally-scaled or CI-driven deployment wants), and a
**`WithHTTPContextFunc` hook** that runs per request before any tool dispatch —
the natural place to terminate auth and inject the authenticated identity into
the request context. SSE would force two endpoints and sticky sessions for no
benefit here.

### Stateless vs session-stateful

- **Stateless** (recommended default for the shared mode): every request
  carries its own auth and `group`/CWD routing args; no server memory per
  client. Survives daemon restarts transparently, trivially load-balanced,
  ideal for headless/CI callers. Cost: no server-driven notifications
  (resource-change pushes) — acceptable, archigraph's tools are request/response.
- **Session-stateful**: enables `notifications/*` and resource subscriptions,
  at the cost of a session table + **idle session timeout** (the issue's
  "session timeout" ask). Defer unless a concrete streaming need appears.

## Auth options

The socket transport has **no auth** by design (OS permissions). An HTTP
transport crosses a network boundary, so auth becomes mandatory. Options,
in rough order of operational simplicity:

| Mechanism | Per-user identity | Rotation | Revocation | CI / headless | Notes |
|---|---|---|---|---|---|
| **Static bearer token / API key** (single shared secret) | ✗ (one identity for all) | Manual, global | All-or-nothing | Trivial | Simplest; **no per-user audit**, blast-radius = everyone on rotation. Fine for a trusted small team behind a VPN; weak for accountability. |
| **Per-user API keys** (keyed token store) | ✓ | Per-key | Per-key | Good (key per CI job) | Recommended starting point if any accountability is needed. Needs a key store + issuance/revocation UX. |
| **OAuth2 / OIDC bearer (JWT)** | ✓ (from IdP) | IdP-driven (short-lived) | IdP-driven | Workable (client-credentials grant) | Best identity story; **leans on an external IdP** the team must run/configure. mcp-go has `protected_resource.go` (OAuth protected-resource metadata) — partial scaffolding exists. Heaviest to operate. |
| **mTLS** (client certs) | ✓ (cert CN/SAN) | Cert lifecycle | CRL / short-lived certs | Strong for service-to-service | Strongest transport-level auth; **operationally heavy** (PKI, cert distribution). Often terminated at a reverse proxy instead of in-process. |

**Trade-off summary.** A single static token is the cheapest to ship and the
weakest for accountability and revocation. Per-user keys add a store but give
per-engineer audit and surgical revocation. OAuth2/OIDC gives the best identity
+ rotation story but imports an IdP dependency and real complexity. mTLS is the
strongest on the wire but the heaviest to operate and is usually a
reverse-proxy concern, not an in-process one.

**The prototype in this ADR implements only a pluggable `Authenticator`
interface plus a static-token stub** so the seam is reviewable. **It is not a
production auth backend** — choosing the real one is a maintainer/security
decision (see below).

## Multi-tenancy / isolation

A shared daemon holds **every registered group's graph in memory** (ADR-0004).
On the laptop that is fine — one user owns all groups. On a **shared** server
it is the central risk: by default any authenticated caller could query any
group. The HTTP mode therefore needs an **authorization** layer on top of
authentication:

- **Per-user group scoping.** The authenticated identity must map to a set of
  groups it may query. Request authz answer to "can user X query group Y?" must
  be enforced *after* routing resolves the target group (ADR-0008) and *before*
  the tool runs. The prototype exposes this as an `Authorizer` seam
  (`CanAccessGroup`) that defaults to **deny-by-policy TODO** — it must be
  filled by the adopted model, not silently allow-all.
- **Routing interaction.** ADR-0008's CWD-based routing is meaningless across a
  network (the server's CWD is not the caller's). In HTTP mode, **explicit
  `group` args become mandatory** (or are derived from the authenticated
  identity's allowed set); the singleton-group fallback is the only safe
  implicit path.
- **Rate limiting.** A shared endpoint needs per-identity rate limiting to stop
  one client starving others (graph queries can be CPU-heavy). Prototype leaves
  a `RateLimiter` TODO seam.
- **Audit.** Every authenticated request should log `(identity, group, tool,
  outcome)`. archigraph already has an activity broker / MCP activity log
  (`SetMCPActivityLog`); the HTTP mode should feed it the authenticated identity
  rather than an anonymous session id.

## TLS / network

- **Recommended: terminate TLS at a reverse proxy** (nginx/Caddy/Traefik or a
  cloud LB) and have archigraph speak plain HTTP on a loopback/private-network
  bind. This keeps cert management out of the Go binary (consistent with
  ADR-0001's "single binary, minimal surface" ethos), lets ops reuse existing
  TLS automation, and is where mTLS would naturally live.
- **In-process TLS** (`http.Server` with a cert) is supported by the skeleton
  as an option for self-hosters who don't want a proxy, but is **not the
  default** and is documented as the less-preferred path.
- **Bind address.** Default bind, *if the mode is ever enabled*, must be an
  explicit operator choice. The skeleton refuses to start without an explicit
  addr and never binds `0.0.0.0` implicitly.
- **Container packaging** (the issue's ask) is a natural fit for the
  reverse-proxy model: ship the daemon as a container, front it with the
  cluster's ingress, inject the token/secret via the orchestrator. Out of scope
  for this prototype but unblocked by the stateless Streamable-HTTP shape.

## Decision (recommendation)

**Recommend: adopt the HTTP MCP transport as an OPT-IN, OFF-BY-DEFAULT
additional deployment mode, gated behind `ARCHIGRAPH_HTTP_MCP`, using mcp-go's
Streamable HTTP server in stateless mode, with a pluggable
authentication + authorization middleware — and DO NOT pick the production
security model unilaterally.** ADR-0004 remains the default and only sanctioned
laptop topology.

Concretely, this ADR lands:

1. A `transport.Transport` interface in `internal/mcp/transport/` that the
   existing stdio path satisfies (`StdioTransport`), establishing the seam.
2. An `HTTPTransport` skeleton wrapping `mcp-go`'s `StreamableHTTPServer`,
   feature-flagged via `ARCHIGRAPH_HTTP_MCP` (**default OFF**), with a
   pluggable `Authenticator` middleware (static-token **stub**) and explicit
   TODOs where the real auth backend, `Authorizer`, and `RateLimiter` must go.
3. **No wiring into the daemon's default path.** The skeleton is constructed
   only when the flag is set and never started otherwise. The production daemon
   entrypoint is untouched in this change.

### Phased rollout (proposed, gated on maintainer decisions)

- **Phase 0 (this ADR):** interface + off-by-default skeleton + stub auth + tests. No prod exposure.
- **Phase 1:** maintainer picks the auth model (below); implement the real `Authenticator` + a key/secret backend; add per-identity audit; keep flag off by default; document a single-tenant pilot behind a reverse proxy + VPN.
- **Phase 2:** implement `Authorizer` (per-user group scoping) + `RateLimiter`; enforce explicit-`group` routing; container packaging + ops docs; opt-in for trusted teams.
- **Phase 3 (only if demanded):** session-stateful mode for notifications, OAuth2/OIDC, mTLS — each its own decision.

## Decisions for the maintainer (require human / security sign-off)

These are intentionally **NOT decided** by this ADR or prototype:

1. **Do we adopt the shared HTTP mode at all**, given it tensions with ADR-0004?
   (The recommendation is "yes, opt-in & off-by-default" — but adoption is the maintainer's call.)
2. **Which authentication model**: static shared token vs per-user API keys vs OAuth2/OIDC vs mTLS (or proxy-terminated combination). This sets the accountability/revocation posture.
3. **Where does the secret/key store live** (env-injected secret, file, KMS, IdP)? The prototype ships a stub only.
4. **Authorization policy**: default-deny vs default-allow for group access; how identity→group mappings are configured. (Recommendation: default-deny.)
5. **TLS termination**: in-process vs reverse proxy (recommendation: proxy) and whether mTLS is required.
6. **Default bind + network exposure policy** (loopback/private only; never implicit `0.0.0.0`).
7. **Rate-limit + audit requirements** for compliance.
8. **Stateless vs session-stateful** and the idle **session timeout** value if stateful.

Until items 2–6 are signed off, `ARCHIGRAPH_HTTP_MCP` must remain **off by
default and undocumented as production-ready**; the skeleton's stub auth is a
placeholder, not a security control.

## Consequences

### Positive
- Establishes a clean, tested `Transport` seam — stdio stays the default, HTTP slots in beside it without touching the laptop path.
- Reuses the already-vendored `mcp-go` Streamable-HTTP server (no new dependency, consistent with ADR-0002's clean-room stance).
- Unblocks team/CI deployments **without committing** to a security model prematurely.
- Stateless shape is restart-safe and load-balancer-friendly.

### Negative
- Introduces a network-facing surface to a codebase whose trust model was "local user only." Even off by default, the code exists and must be kept safe (hence the explicit no-implicit-bind, stub-not-secret guards).
- Tensions with ADR-0004; we now maintain two deployment topologies conceptually.
- A real adoption pulls in auth/authz/rate-limit/audit/TLS — non-trivial follow-on work, deliberately deferred to phases gated on maintainer decisions.

### Neutral
- The `Transport` interface is useful regardless of the HTTP decision: it documents what already exists (stdio + socket-driven) behind one contract.

## Alternatives considered

- **Do nothing / reject (laptop-only forever).** Honors ADR-0004 purely; rejected as the *default* answer because the team-deployment demand is real and the dependency already supports it — but kept as the off-by-default reality until a maintainer opts in.
- **SSE transport.** Rejected as primary; superseded by Streamable HTTP, two endpoints, sticky sessions.
- **Roll our own HTTP/JSON-RPC server.** Rejected; `mcp-go` already implements the spec-correct Streamable HTTP transport (ADR-0002 already sanctions depending on it).
- **Build the production auth model now.** Rejected for this ticket — #4296 is an evaluation; the security model is a maintainer/security decision, so we ship the seam + stub and document the open decisions instead.
</content>
</invoke>
