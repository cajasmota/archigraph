// Elixir Tesla / Req consumer-side HTTP client synthesis (#3511, epic #3505).
//
// Closes the biggest Elixir outbound-HTTP blind spot: before this pass the
// only Elixir HTTP clients recognised were Finch.build and HTTPoison (#1483).
// Tesla and Req are the two most idiomatic modern Elixir HTTP clients and were
// entirely invisible, so any Elixir service that called a downstream API via
// Tesla/Req produced no http_endpoint_call entity and no cross-repo FETCHES
// edge — the cross-link graph was one-directional.
//
// This file emits one synthetic http_endpoint_call per detected Tesla/Req
// call site (via the shared emitFn from applyHTTPEndpointSynthesis), so the
// existing cross-repo linker pairs them with producer-side definitions by
// canonical path Name.
//
// Patterns covered
// ----------------
//
// Tesla (a `use Tesla` module with a BaseUrl middleware + verb calls):
//
//	defmodule MyApp.GatewayClient do
//	  use Tesla
//	  plug Tesla.Middleware.BaseUrl, "https://gateway:4000"
//	  plug Tesla.Middleware.JSON
//
//	  def get_user(id), do: Tesla.get(client(), "/users/#{id}")
//	  def list, do: get(client(), "/users")          # bare verb (use Tesla import)
//	  def create(b), do: post(client(), "/users", b)
//	end
//
//	→ http:GET:/users/{id}, http:GET:/users, http:POST:/users  (framework=tesla)
//
// The BaseUrl middleware host is stripped from the canonical path (the
// producer side serves "/users" without the base prefix), exactly mirroring
// the Finch/@base_url treatment.
//
// Req:
//
//	Req.get!("https://catalog:3001/products")        # → http:GET:/products
//	Req.post!("https://catalog:3001/products", json: body)
//	Req.get("https://catalog:3001/products/" <> id)
//	req = Req.new(base_url: "https://catalog:3001")
//	Req.get(req, url: "/products")                    # → http:GET:/products
//
//	→ framework=req
//
// Verbs: get, post, put, patch, delete, head, options (and their `!` bang
// variants for Req). Interpolated `#{...}` path segments resolve to `{name}`
// placeholders via the shared canonicalizeElixirInterpolation helper, reusing
// the @module-attr / System.get_env symbol table built by buildElixirSymbolTable.
package engine

import (
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/engine/httproutes"
)

// ---------------------------------------------------------------------------
// Tesla
// ---------------------------------------------------------------------------

// elixirTeslaUseRe detects a `use Tesla` directive, which both enables the
// bare verb form (`get(client, path)`) and signals that this module is a Tesla
// client. Bare verbs are only scanned when this is present, to avoid colliding
// with Phoenix controller-action `get`/`post` references elsewhere.
var elixirTeslaUseRe = regexp.MustCompile(`(?m)^\s*use\s+Tesla\b`)

// elixirTeslaBaseURLRe captures the `plug Tesla.Middleware.BaseUrl, "url"`
// middleware that establishes the client's base URL. Group 1 = base URL literal.
var elixirTeslaBaseURLRe = regexp.MustCompile(
	`(?m)^\s*plug\s+Tesla\.Middleware\.BaseUrl\s*,\s*"([^"\n\r]+)"`,
)

// elixirTeslaQualifiedVerbRe matches `Tesla.<verb>(client, "/path")` where the
// path is a string literal (possibly interpolated). The first argument is the
// client; the SECOND argument is the URL/path.
// Group 1 = verb, group 2 = path literal body.
var elixirTeslaQualifiedVerbRe = regexp.MustCompile(
	`\bTesla\.(get|post|put|patch|delete|head|options)\s*\(\s*[a-z_][\w.]*(?:\(\))?\s*,\s*"([^"\n\r]*)"`,
)

// elixirTeslaBareVerbRe matches the imported bare verb form
// `get(client, "/path")` / `post(client, "/path", body)`. Only scanned when
// `use Tesla` is present in the file. The receiver (client) must be a bare
// identifier or a zero-arg call like `client()`; we accept an identifier
// optionally followed by `()`.
// Group 1 = verb, group 2 = path literal body.
var elixirTeslaBareVerbRe = regexp.MustCompile(
	`(?:^|[^\w.])(get|post|put|patch|delete|head|options)\s*\(\s*[a-z_][a-z0-9_]*(?:\(\))?\s*,\s*"([^"\n\r]*)"`,
)

// ---------------------------------------------------------------------------
// Req
// ---------------------------------------------------------------------------

// elixirReqLiteralVerbRe matches `Req.get("url")`, `Req.get!("url")`,
// `Req.post!("url", json: body)`. Group 1 = verb (without bang), group 2 =
// URL literal body. The optional `!` is consumed but not captured.
var elixirReqLiteralVerbRe = regexp.MustCompile(
	`\bReq\.(get|post|put|patch|delete|head|options)!?\s*\(\s*"((?:https?://|/)[^"\n\r#]*)"`,
)

// elixirReqInterpVerbRe matches `Req.get("...#{...}...")` — an interpolated
// URL literal passed directly. Group 1 = verb, group 2 = interpolated body.
var elixirReqInterpVerbRe = regexp.MustCompile(
	`\bReq\.(get|post|put|patch|delete|head|options)!?\s*\(\s*"([^"\n\r]*#\{[^"\n\r]*)"`,
)

// elixirReqConcatVerbRe matches `Req.get("https://host/base/" <> id)` — the
// idiomatic Elixir string-concat URL form. Group 1 = verb, group 2 = literal
// prefix (the dynamic suffix becomes a {param} placeholder).
var elixirReqConcatVerbRe = regexp.MustCompile(
	`\bReq\.(get|post|put|patch|delete|head|options)!?\s*\(\s*"((?:https?://|/)[^"\n\r#]*)"\s*<>\s*[a-z_@]`,
)

// elixirReqURLOptRe matches `Req.get(req, url: "/path")` where the path is
// supplied via the `url:` keyword option on a pre-built request. Group 1 =
// verb, group 2 = path literal body.
var elixirReqURLOptRe = regexp.MustCompile(
	`\bReq\.(get|post|put|patch|delete|head|options)!?\s*\(\s*[a-z_][\w.]*(?:\(\))?\s*,\s*url:\s*"([^"\n\r]*)"`,
)

// synthesizeElixirTeslaReq scans an Elixir source file for Tesla and Req
// consumer-side HTTP calls and emits http_endpoint_call synthetics. It is
// called from applyHTTPEndpointSynthesis for lang="elixir" alongside the
// existing Finch/HTTPoison synthesizer.
func synthesizeElixirTeslaReq(content string, emit emitFn) {
	hasTesla := strings.Contains(content, "Tesla")
	hasReq := strings.Contains(content, "Req.")
	if !hasTesla && !hasReq {
		return
	}

	syms := buildElixirSymbolTable(content)

	// emitPath canonicalises a raw path/URL literal (resolving interpolation
	// against the symbol table) and emits a synthetic. basePrefix, when
	// non-empty, is prepended before canonicalisation so a relative Tesla path
	// inherits the BaseUrl host (which is then stripped to leave the route).
	emitPath := func(verb, raw, basePrefix, framework string) {
		if strings.Contains(raw, "#{") {
			joined := raw
			if basePrefix != "" && strings.HasPrefix(raw, "/") {
				joined = basePrefix + raw
			}
			path, ok := canonicalizeElixirInterpolation(joined, syms)
			if !ok {
				return
			}
			canonical := httproutes.Canonicalize(httproutes.FrameworkExpress, path)
			emit(strings.ToUpper(verb), canonical, framework, "", "")
			return
		}
		joined := raw
		if basePrefix != "" && strings.HasPrefix(raw, "/") {
			joined = basePrefix + raw
		}
		path, ok := normalizeRawClientPath(joined)
		if !ok {
			return
		}
		canonical := httproutes.Canonicalize(httproutes.FrameworkExpress, path)
		emit(strings.ToUpper(verb), canonical, framework, "", "")
	}

	// ----- Tesla -----
	if hasTesla {
		isTeslaClient := elixirTeslaUseRe.MatchString(content)

		// BaseUrl middleware → base prefix for relative verb paths.
		basePrefix := ""
		if bm := elixirTeslaBaseURLRe.FindStringSubmatch(content); bm != nil {
			basePrefix = bm[1]
		}

		// Tesla.<verb>(client, "/path")
		for _, m := range elixirTeslaQualifiedVerbRe.FindAllStringSubmatch(content, -1) {
			if len(m) < 3 {
				continue
			}
			emitPath(m[1], m[2], basePrefix, "tesla")
		}

		// Bare verb form (get(client, "/path")) — only inside a use Tesla module.
		if isTeslaClient {
			for _, m := range elixirTeslaBareVerbRe.FindAllStringSubmatch(content, -1) {
				if len(m) < 3 {
					continue
				}
				emitPath(m[1], m[2], basePrefix, "tesla")
			}
		}
	}

	// ----- Req -----
	if hasReq {
		// Req.get("https://host/path") / Req.get!("/path")
		for _, m := range elixirReqLiteralVerbRe.FindAllStringSubmatch(content, -1) {
			if len(m) < 3 {
				continue
			}
			emitPath(m[1], m[2], "", "req")
		}

		// Req.get("...#{...}...") interpolated literal
		for _, m := range elixirReqInterpVerbRe.FindAllStringSubmatch(content, -1) {
			if len(m) < 3 {
				continue
			}
			emitPath(m[1], m[2], "", "req")
		}

		// Req.get("https://host/base/" <> id) — concat form: emit the literal
		// prefix with a trailing {id} placeholder for the dynamic suffix.
		for _, m := range elixirReqConcatVerbRe.FindAllStringSubmatch(content, -1) {
			if len(m) < 3 {
				continue
			}
			prefix := m[2]
			if !strings.HasSuffix(prefix, "/") {
				prefix += "/"
			}
			emitPath(m[1], prefix+"{id}", "", "req")
		}

		// Req.get(req, url: "/path")
		for _, m := range elixirReqURLOptRe.FindAllStringSubmatch(content, -1) {
			if len(m) < 3 {
				continue
			}
			emitPath(m[1], m[2], "", "req")
		}
	}
}
