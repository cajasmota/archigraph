// websocket_routes.go — dedicated WebSocket-route extraction for Rust web
// frameworks (#4965, Routing/websocket_route_extraction). Sibling of axum.go /
// actix_web.go / minor_fw_routing.go.
//
// Before this pass there was NO dedicated websocket_route_extraction capability
// key in the http_backend taxonomy: axum's `WebSocketUpgrade` and actix's
// `ws::WebsocketContext<H>` were each emitted as a bare `SCOPE.Operation/
// websocket` entity by the producer route extractors (axum.go step 8,
// actix_web.go step 7), but those carried NO route path and NO handler
// attribution — and warp `warp::ws()`, actix `actix_ws::handle` /
// `actix_web_actors::ws::start`, and tokio-tungstenite `accept_async` were not
// recovered at all. #4965 adds the dedicated capability and this extractor,
// which recovers the WS UPGRADE ROUTE (verb+path) and the HANDLER it dispatches
// to wherever the framework idiom exposes them.
//
// Entity shape (mirrors the existing bare ws entity so this MERGES onto / is a
// strict superset of what the producers emit — Kind/Subtype unchanged, only the
// property contract is richer):
//
//	Kind    = SCOPE.Operation
//	Subtype = websocket
//	Name    = "WS <route_path>" when a route path is recovered (axum/warp), else
//	          the handler/struct name (actix actor) or "WS accept" (tungstenite).
//
// Property contract:
//
//	framework        — axum | actix_web | warp | tokio_tungstenite
//	provenance       — INFERRED_FROM_<FRAMEWORK>_WEBSOCKET
//	websocket        — "true" (uniform facet flag so a consumer can select WS
//	                   ops without reasoning about the Subtype)
//	route_path       — the upgrade route literal, normalised to canonical {param}
//	                   form, when recovered (axum .route, warp filter chain)
//	http_method      — the upgrade route verb (GET for the WS handshake), when a
//	                   route is recovered
//	handler_name     — the upgrade handler fn / actor struct, when recovered
//	upgrade_mechanism— the idiom that proved this is a WS route
//	                   (WebSocketUpgrade | on_upgrade | ws::start | actix_ws::handle |
//	                    WebsocketContext | warp::ws | accept_async)
//
// Honest-partial (NEVER fabricated):
//   - axum: a `.route("/p", get(h))` is stamped as WS ONLY when the named handler
//     `h`'s body proves a WS upgrade (returns `WebSocketUpgrade` or calls
//     `.on_upgrade(...)`); a plain HTTP route is left to axum.go. A bare
//     `WebSocketUpgrade`/`.on_upgrade` with no resolvable route is still emitted
//     as an unrouted evidence entity (route_path omitted) so coverage is not lost.
//   - actix: `ws::WebsocketContext<H>` / `ws::start(...)` / `actix_ws::handle(...)`
//     are handler/struct-attributed; actix WS is mounted on a normal HTTP route
//     elsewhere so route_path is generally omitted (honest — not fabricated).
//   - warp: `warp::ws()` in a filter chain ending in `.and_then/.map(handler)`
//     recovers path+handler exactly as the producer warp chain does.
//   - tokio-tungstenite: `accept_async(stream)` is a server-side accept point,
//     not an HTTP route — emitted with upgrade_mechanism=accept_async, no path.
//
// Wrong-language / no-match → no-op (guarded on file.Language=="rust" and a
// fast substring gate).
//
// Refs #4965, #4921.
package rust

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_rust_websocket_routes", &rustWebSocketExtractor{})
}

type rustWebSocketExtractor struct{}

func (e *rustWebSocketExtractor) Language() string { return "custom_rust_websocket_routes" }

var (
	// A handler-fn body proves a WS upgrade when it returns a WebSocketUpgrade
	// (`ws: WebSocketUpgrade) -> Response` / `-> impl IntoResponse { ws.on_upgrade(...) }`)
	// or calls `.on_upgrade(`. Either is sufficient.
	reWsAxumUpgradeInBody = regexp.MustCompile(`WebSocketUpgrade|\.on_upgrade\s*\(`)

	// actix actor WS: `ws::start(MyWs{...}, &req, stream)` / `ws::WebsocketContext<H>`.
	// Group 1 (start) = the actor struct/expr head; the WebsocketContext form is
	// handled by the existing reActixWebSocket (actix_web.go) — we re-scan it here
	// to stamp the richer contract.
	reWsActixStart = regexp.MustCompile(`ws::start\s*\(\s*([A-Za-z_]\w*)`)
	// actix-ws (the newer handshake crate): `actix_ws::handle(&req, stream)`.
	reWsActixWsHandle = regexp.MustCompile(`actix_ws::handle\s*\(`)

	// warp: a filter chain that contains `warp::ws()` and a terminal
	// `.and_then/.map(...)`. Matches the whole chain blob (up to the statement
	// terminator) so path + handler can be recovered from it independently.
	reWsWarpChain = regexp.MustCompile(`warp::ws\s*\(\s*\)[^;]*?\.(?:and_then|map)\s*\([^;]*`)
	// Upgrade handler inside the terminal closure: `ws.on_upgrade(handle_ws)`.
	reWsWarpOnUpgrade = regexp.MustCompile(`\.on_upgrade\s*\(\s*(\w+)`)
	// Bare-fn terminal: `.and_then(ws_handler)` / `.map(ws_handler)`.
	reWsWarpBareFn = regexp.MustCompile(`\.(?:and_then|map)\s*\(\s*(\w+)\s*\)`)

	// tokio-tungstenite server accept: `accept_async(stream)` /
	// `accept_hdr_async(stream, cb)` / `tokio_tungstenite::accept_async(...)`.
	reWsTungsteniteAccept = regexp.MustCompile(`\baccept(?:_hdr)?_async\s*\(`)
)

func (e *rustWebSocketExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/rust")
	_, span := tracer.Start(ctx, "indexer.rust_websocket_routes.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("file_path", file.Path),
		))
	defer span.End()

	if len(file.Content) == 0 || file.Language != "rust" {
		return nil, nil
	}
	src := string(file.Content)

	// Fast gate: a WS surface must mention one of the recognised idioms.
	if !strings.Contains(src, "WebSocketUpgrade") && !strings.Contains(src, "on_upgrade") &&
		!strings.Contains(src, "ws::start") && !strings.Contains(src, "WebsocketContext") &&
		!strings.Contains(src, "actix_ws::handle") && !strings.Contains(src, "warp::ws") &&
		!strings.Contains(src, "accept_async") {
		return nil, nil
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	for _, ent := range e.extractAxum(src, file) {
		add(ent)
	}
	for _, ent := range e.extractActix(src, file) {
		add(ent)
	}
	for _, ent := range e.extractWarp(src, file) {
		add(ent)
	}
	for _, ent := range e.extractTungstenite(src, file) {
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// --- axum surface -------------------------------------------------------------
//
// axum mounts a WS upgrade on a normal `.route("/ws", get(handler))`; the
// handler takes a `WebSocketUpgrade` and returns `ws.on_upgrade(...)`. We build
// a set of handler names whose body proves a WS upgrade, then stamp the routes
// that name such a handler with the full WS contract (route_path + verb +
// handler). A bare `WebSocketUpgrade`/`.on_upgrade` with no resolvable route is
// still emitted (unrouted) so coverage is not lost.
func (e *rustWebSocketExtractor) extractAxum(src string, file extractor.FileInput) []types.EntityRecord {
	// axum's distinctive token is the `WebSocketUpgrade` extractor type. A file
	// that uses `warp::ws()` is a warp WS surface (it also calls `.on_upgrade`,
	// but on warp's `Ws`, not axum's) and is handled by extractWarp — don't let
	// the axum surface claim it.
	if !strings.Contains(src, "WebSocketUpgrade") {
		return nil
	}

	// handler-name set: a fn whose body proves a WS upgrade.
	wsHandlers := map[string]bool{}
	for _, fm := range rustDepFnRe.FindAllStringSubmatchIndex(src, -1) {
		name := src[fm[2]:fm[3]]
		body := rustRespBodyWindow(src, fm[1])
		if reWsAxumUpgradeInBody.MatchString(body) {
			wsHandlers[name] = true
		}
	}

	var out []types.EntityRecord

	if len(wsHandlers) > 0 {
		// Recompute the nest-prefix map exactly as axum.go does so paths compose.
		nestPrefix := map[string]string{}
		for _, m := range reAxumNest.FindAllStringSubmatchIndex(src, -1) {
			nestPrefix[src[m[4]:m[5]]] = rustNormalizePath(src[m[2]:m[3]])
		}
		seen := map[string]bool{}
		for _, m := range reAxumRoute.FindAllStringSubmatchIndex(src, -1) {
			path := rustNormalizePath(src[m[2]:m[3]])
			methodRouter := src[m[4]:m[5]]
			prefix := axumRouteNestPrefix(src, m[0], nestPrefix)
			fullPath := rustJoinPaths(prefix, path)
			for _, vm := range reAxumMethodRouter.FindAllStringSubmatch(methodRouter, -1) {
				method := strings.ToUpper(vm[1])
				handler := vm[2]
				if !wsHandlers[handler] {
					continue // plain HTTP route — left to axum.go
				}
				name := "WS " + fullPath
				if seen[name] {
					continue
				}
				seen[name] = true
				ent := makeEntity(name, "SCOPE.Operation", "websocket", file.Path, file.Language, lineOf(src, m[0]))
				setProps(&ent, "framework", "axum",
					"provenance", "INFERRED_FROM_AXUM_WEBSOCKET", "websocket", "true",
					"http_method", method, "route_path", fullPath, "handler_name", handler,
					"upgrade_mechanism", "WebSocketUpgrade")
				if prefix != "" {
					setProps(&ent, "nest_prefix", prefix)
				}
				out = append(out, ent)
			}
		}
		if len(out) > 0 {
			return out
		}
	}

	// No WS route resolved to a known handler: emit a single unrouted evidence
	// entity so the WS surface is still recorded (honest-partial — route_path
	// omitted, never fabricated).
	loc := reWsAxumUpgradeInBody.FindStringIndex(src)
	if loc == nil {
		return nil
	}
	mech := "WebSocketUpgrade"
	if !strings.Contains(src, "WebSocketUpgrade") {
		mech = "on_upgrade"
	}
	ent := makeEntity("WS upgrade (axum)", "SCOPE.Operation", "websocket", file.Path, file.Language, lineOf(src, loc[0]))
	setProps(&ent, "framework", "axum",
		"provenance", "INFERRED_FROM_AXUM_WEBSOCKET", "websocket", "true",
		"upgrade_mechanism", mech)
	return []types.EntityRecord{ent}
}

// --- actix surface ------------------------------------------------------------
//
// actix WS is mounted on a normal HTTP route; the upgrade idiom is the handler
// body's `ws::start(Actor{...}, &req, stream)` (actix-web-actors) or
// `actix_ws::handle(&req, stream)` (actix-ws). The actor struct names the
// session. route_path is generally NOT recoverable from the upgrade idiom alone
// (the route is a plain `.route("/ws", web::get().to(h))` elsewhere), so it is
// honestly omitted.
func (e *rustWebSocketExtractor) extractActix(src string, file extractor.FileInput) []types.EntityRecord {
	if !strings.Contains(src, "ws::start") && !strings.Contains(src, "WebsocketContext") &&
		!strings.Contains(src, "actix_ws::handle") {
		return nil
	}
	var out []types.EntityRecord
	seen := map[string]bool{}
	emit := func(name, handler, mech string, off int) {
		key := "actix:" + name
		if seen[key] {
			return
		}
		seen[key] = true
		ent := makeEntity(name, "SCOPE.Operation", "websocket", file.Path, file.Language, lineOf(src, off))
		setProps(&ent, "framework", "actix_web",
			"provenance", "INFERRED_FROM_ACTIX_WEBSOCKET", "websocket", "true",
			"upgrade_mechanism", mech)
		if handler != "" {
			setProps(&ent, "handler_name", handler)
		}
		out = append(out, ent)
	}

	// ws::start(Actor{...}) — the actor struct is the WS session handler.
	for _, m := range reWsActixStart.FindAllStringSubmatchIndex(src, -1) {
		actor := src[m[2]:m[3]]
		emit("WS "+actor+" (actix)", actor, "ws::start", m[0])
	}
	// ws::WebsocketContext<H> — the actor type parameter names the session.
	for _, m := range reActixWebSocket.FindAllStringSubmatchIndex(src, -1) {
		h := ""
		if m[2] >= 0 {
			h = src[m[2]:m[3]]
		} else if m[4] >= 0 {
			h = src[m[4]:m[5]]
		}
		if h == "" {
			continue
		}
		emit("WS "+h+" (actix)", h, "WebsocketContext", m[0])
	}
	// actix_ws::handle(&req, stream) — the newer handshake crate (no actor type).
	for _, m := range reWsActixWsHandle.FindAllStringIndex(src, -1) {
		emit("WS handshake (actix-ws)", "", "actix_ws::handle", m[0])
	}
	return out
}

// --- warp surface -------------------------------------------------------------
//
// warp upgrades via a `warp::ws()` filter composed into a path/method chain that
// terminates in `.and_then/.map(handler)`. We recover path+handler exactly as the
// producer warp chain does (reWarpPathMacroIn / reWarpPathFn).
func (e *rustWebSocketExtractor) extractWarp(src string, file extractor.FileInput) []types.EntityRecord {
	if !strings.Contains(src, "warp::ws") {
		return nil
	}
	var out []types.EntityRecord
	seen := map[string]bool{}
	for _, m := range reWsWarpChain.FindAllStringIndex(src, -1) {
		blob := src[m[0]:m[1]]
		// Handler: prefer the on_upgrade(handler) target inside the terminal
		// closure, else the bare-fn `.and_then/.map(handler)` form.
		handler := ""
		if hm := reWsWarpOnUpgrade.FindStringSubmatch(blob); hm != nil {
			handler = hm[1]
		} else if hm := reWsWarpBareFn.FindStringSubmatch(blob); hm != nil {
			handler = hm[1]
		}
		// Path: prefer the macro form, else the function form. A WS chain may carry
		// no path filter (root upgrade) — then route_path is omitted.
		path := ""
		if pm := reWarpPathMacroIn.FindStringSubmatch(blob); pm != nil {
			path = normWarpPath(pm[1])
		} else if pf := reWarpPathFn.FindStringSubmatch(blob); pf != nil {
			path = "/" + strings.Trim(pf[1], "/")
		}
		// Look just before the chain for a path filter on the same statement when
		// the path precedes warp::ws() (chain blob starts at warp::ws()).
		if path == "" {
			pre := src[:m[0]]
			if i := strings.LastIndex(pre, ";"); i >= 0 {
				pre = pre[i+1:]
			}
			if pm := reWarpPathMacroIn.FindStringSubmatch(pre); pm != nil {
				path = normWarpPath(pm[1])
			} else if pf := reWarpPathFn.FindStringSubmatch(pre); pf != nil {
				path = "/" + strings.Trim(pf[1], "/")
			}
		}
		name := "WS " + path
		if path == "" {
			name = "WS " + handler + " (warp)"
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		ent := makeEntity(name, "SCOPE.Operation", "websocket", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "warp",
			"provenance", "INFERRED_FROM_WARP_WEBSOCKET", "websocket", "true",
			"http_method", "GET", "handler_name", handler,
			"upgrade_mechanism", "warp::ws")
		if path != "" {
			setProps(&ent, "route_path", path)
		}
		out = append(out, ent)
	}
	return out
}

// --- tokio-tungstenite surface ------------------------------------------------
//
// tokio-tungstenite is a raw WS library, not an HTTP router: the server accepts
// upgraded connections with `accept_async(stream)` / `accept_hdr_async(...)`.
// There is no route path; we emit a single accept-point entity per file
// (honest — upgrade_mechanism=accept_async, route_path omitted).
func (e *rustWebSocketExtractor) extractTungstenite(src string, file extractor.FileInput) []types.EntityRecord {
	loc := reWsTungsteniteAccept.FindStringIndex(src)
	if loc == nil {
		return nil
	}
	ent := makeEntity("WS accept (tokio-tungstenite)", "SCOPE.Operation", "websocket", file.Path, file.Language, lineOf(src, loc[0]))
	setProps(&ent, "framework", "tokio_tungstenite",
		"provenance", "INFERRED_FROM_TUNGSTENITE_WEBSOCKET", "websocket", "true",
		"upgrade_mechanism", "accept_async")
	return []types.EntityRecord{ent}
}
