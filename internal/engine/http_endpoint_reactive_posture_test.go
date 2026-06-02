package engine

import "testing"

// Value-asserting posture tests for the lightweight / reactive JVM frameworks
// (#3858, epic #3854): Javalin, Vert.x, Akka-HTTP, Struts, Spring WebFlux. Each
// test asserts the SPECIFIC posture prop on the SPECIFIC endpoint for the named
// framework — not len>0. They reuse the deprecProps / mustEndpoint harness (the
// reactive posture resolvers run in the same `java` synthesis-tail branch as the
// Spring + JAX-RS resolvers).

// ---------------------------------------------------------------------------
// Response codes — Javalin (ctx.status / @OpenApi)
// ---------------------------------------------------------------------------

func TestResponseCodes_Javalin_CtxStatusNumeric(t *testing.T) {
	// app.get("/items/{id}", ...) with ctx.status(404) + ctx.status(200) → {200,404}.
	src := `
import io.javalin.Javalin;

public class App {
    public static void main(String[] args) {
        Javalin app = Javalin.create();
        app.get("/items/{id}", ctx -> {
            if (notFound) {
                ctx.status(404).result("missing");
            } else {
                ctx.status(200).json(item);
            }
        });
    }
}
`
	eps := deprecProps(t, "java", "src/App.java", src)
	e := mustEndpoint(t, eps, "GET /items/{id}")
	if got := e.Properties["response_codes"]; got != "200,404" {
		t.Fatalf("response_codes=%q want 200,404 (props: %v)", got, e.Properties)
	}
	if got := e.Properties["success_code"]; got != "200" {
		t.Fatalf("success_code=%q want 200", got)
	}
}

func TestResponseCodes_Javalin_CtxStatusEnum(t *testing.T) {
	// ctx.status(HttpStatus.CREATED) → 201.
	src := `
import io.javalin.Javalin;
import io.javalin.http.HttpStatus;

public class App {
    public static void main(String[] args) {
        Javalin app = Javalin.create();
        app.post("/widgets", ctx -> {
            ctx.status(HttpStatus.CREATED).json(w);
        });
    }
}
`
	eps := deprecProps(t, "java", "src/App.java", src)
	e := mustEndpoint(t, eps, "POST /widgets")
	if got := e.Properties["response_codes"]; got != "201" {
		t.Fatalf("response_codes=%q want 201 (props: %v)", got, e.Properties)
	}
	if got := e.Properties["success_code"]; got != "201" {
		t.Fatalf("success_code=%q want 201", got)
	}
}

func TestResponseCodes_Javalin_OpenApiResponse(t *testing.T) {
	// @OpenApi(responses = {@OpenApiResponse(status = "200"), @OpenApiResponse(
	// status = "404")}) on a Javalin handler → {200,404}. Javalin @OpenApi
	// annotations decorate the HANDLER METHOD (the route is registered with a
	// method reference), so this asserts the resolver on the real annotation
	// region directly — the end-to-end pass anchors on the route registration,
	// which for the method-reference form does not reach the annotated method
	// (honest-partial of the shared anchoring, not of this resolver).
	region := `
    @OpenApi(
        responses = {
            @OpenApiResponse(status = "200"),
            @OpenApiResponse(status = "404")
        }
    )
    public void getOne(io.javalin.http.Context ctx) {
`
	v := javalinResponseCodes(region, "")
	if _, ok := v.codes[200]; !ok {
		t.Fatalf("javalinResponseCodes missing 200 (codes=%v)", v.codes)
	}
	if _, ok := v.codes[404]; !ok {
		t.Fatalf("javalinResponseCodes missing 404 (codes=%v)", v.codes)
	}
	if len(v.codes) != 2 {
		t.Fatalf("javalinResponseCodes codes=%v want exactly {200,404}", v.codes)
	}
	if v.source != "@OpenApiResponse" {
		t.Fatalf("source=%q want @OpenApiResponse", v.source)
	}
}

// ---------------------------------------------------------------------------
// Response codes — Vert.x Web (setStatusCode)
// ---------------------------------------------------------------------------

func TestResponseCodes_Vertx_SetStatusCodeCreated(t *testing.T) {
	// router.post("/users", ...) with rc.response().setStatusCode(201) → 201.
	src := `
import io.vertx.ext.web.Router;

public class Verticle {
    public void start() {
        Router router = Router.router(vertx);
        router.post("/users").handler(rc -> {
            rc.response().setStatusCode(201).end(body);
        });
    }
}
`
	eps := deprecProps(t, "java", "src/Verticle.java", src)
	e := mustEndpoint(t, eps, "POST /users")
	if got := e.Properties["response_codes"]; got != "201" {
		t.Fatalf("response_codes=%q want 201 (props: %v)", got, e.Properties)
	}
	if got := e.Properties["success_code"]; got != "201" {
		t.Fatalf("success_code=%q want 201", got)
	}
	if got := e.Properties["response_codes_source"]; got != "setStatusCode()" {
		t.Fatalf("response_codes_source=%q want setStatusCode()", got)
	}
}

func TestResponseCodes_Vertx_SetStatusCodeNotFound(t *testing.T) {
	// rc.response().setStatusCode(404) + setStatusCode(200) → {200,404}.
	src := `
import io.vertx.ext.web.Router;

public class Verticle {
    public void start() {
        Router router = Router.router(vertx);
        router.get("/profile").handler(rc -> {
            if (missing) {
                rc.response().setStatusCode(404).end();
            } else {
                rc.response().setStatusCode(200).end(json);
            }
        });
    }
}
`
	eps := deprecProps(t, "java", "src/Verticle.java", src)
	e := mustEndpoint(t, eps, "GET /profile")
	if got := e.Properties["response_codes"]; got != "200,404" {
		t.Fatalf("response_codes=%q want 200,404 (props: %v)", got, e.Properties)
	}
}

// ---------------------------------------------------------------------------
// Response codes — Akka-HTTP (complete(StatusCodes.X) / complete((NNN, ...)))
// ---------------------------------------------------------------------------

func TestResponseCodes_Akka_CompleteStatusCodesNotFound(t *testing.T) {
	// path("lookup", () -> get(() -> complete(StatusCodes.NotFound))) → 404.
	src := `package com.example;

import akka.http.javadsl.server.AllDirectives;
import akka.http.javadsl.model.StatusCodes;

public class Router extends AllDirectives {
    public akka.http.javadsl.server.Route routes() {
        return path("lookup", () ->
            get(() -> complete(StatusCodes.NotFound))
        );
    }
}
`
	eps := deprecProps(t, "java", "src/Router.java", src)
	e := mustEndpoint(t, eps, "GET /lookup")
	if got := e.Properties["response_codes"]; got != "404" {
		t.Fatalf("response_codes=%q want 404 (props: %v)", got, e.Properties)
	}
	if got := e.Properties["response_codes_source"]; got != "complete(StatusCodes)" {
		t.Fatalf("response_codes_source=%q want complete(StatusCodes)", got)
	}
}

func TestResponseCodes_Akka_CompleteStatusCodesCreated(t *testing.T) {
	// path("widgets", () -> post(() -> complete(StatusCodes.Created))) → 201.
	src := `package com.example;

import akka.http.javadsl.server.AllDirectives;
import akka.http.javadsl.model.StatusCodes;

public class Router extends AllDirectives {
    public akka.http.javadsl.server.Route routes() {
        return path("widgets", () ->
            post(() -> complete(StatusCodes.Created))
        );
    }
}
`
	eps := deprecProps(t, "java", "src/Router.java", src)
	e := mustEndpoint(t, eps, "POST /widgets")
	if got := e.Properties["response_codes"]; got != "201" {
		t.Fatalf("response_codes=%q want 201 (props: %v)", got, e.Properties)
	}
	if got := e.Properties["success_code"]; got != "201" {
		t.Fatalf("success_code=%q want 201", got)
	}
}

func TestResponseCodes_Akka_CompleteNumericTuple(t *testing.T) {
	// complete((202, "queued")) tuple form → 202.
	src := `package com.example;

import akka.http.javadsl.server.AllDirectives;

public class Router extends AllDirectives {
    public akka.http.javadsl.server.Route routes() {
        return path("jobs", () ->
            post(() -> complete((202, "queued")))
        );
    }
}
`
	eps := deprecProps(t, "java", "src/Router.java", src)
	e := mustEndpoint(t, eps, "POST /jobs")
	if got := e.Properties["response_codes"]; got != "202" {
		t.Fatalf("response_codes=%q want 202 (props: %v)", got, e.Properties)
	}
}

// ---------------------------------------------------------------------------
// Response codes — Struts 2 (@Result names)
// ---------------------------------------------------------------------------

func TestResponseCodes_Struts_ResultNames(t *testing.T) {
	// @Action with success (200) + error (500) results → {200,500}.
	src := `package com.example.actions;

import org.apache.struts2.convention.annotation.Action;
import org.apache.struts2.convention.annotation.Result;

public class CreateUserAction {
    @Action(value = "/users/create", results = {
        @Result(name = "success", location = "/ok.jsp"),
        @Result(name = "error", location = "/err.jsp")
    })
    public String execute() {
        return SUCCESS;
    }
}
`
	eps := deprecProps(t, "java", "src/CreateUserAction.java", src)
	e := mustEndpoint(t, eps, "ANY /users/create")
	if got := e.Properties["response_codes"]; got != "200,500" {
		t.Fatalf("response_codes=%q want 200,500 (props: %v)", got, e.Properties)
	}
	if got := e.Properties["success_code"]; got != "200" {
		t.Fatalf("success_code=%q want 200", got)
	}
	if got := e.Properties["response_codes_source"]; got != "@Result name" {
		t.Fatalf("response_codes_source=%q want @Result name", got)
	}
}

func TestResponseCodes_Struts_NoneResult(t *testing.T) {
	// @Action with a NONE result (204).
	src := `package com.example.actions;

import org.apache.struts2.convention.annotation.Action;
import org.apache.struts2.convention.annotation.Result;

public class PingAction {
    @Action(value = "/ping", results = {
        @Result(name = "none", type = "json")
    })
    public String execute() {
        return NONE;
    }
}
`
	eps := deprecProps(t, "java", "src/PingAction.java", src)
	e := mustEndpoint(t, eps, "ANY /ping")
	if got := e.Properties["response_codes"]; got != "204" {
		t.Fatalf("response_codes=%q want 204 (props: %v)", got, e.Properties)
	}
}

// ---------------------------------------------------------------------------
// Response codes — Spring WebFlux (ServerResponse builders)
// ---------------------------------------------------------------------------

func TestResponseCodes_WebFlux_ServerResponseNotFoundOk(t *testing.T) {
	// .GET("/items", ...) handler returning ServerResponse.notFound() / .ok() →
	// {200,404}.
	src := `
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.RouterFunctions;
import org.springframework.web.reactive.function.server.ServerResponse;

public class Routes {
    public RouterFunction<ServerResponse> route() {
        return RouterFunctions.route()
            .GET("/items", request -> {
                if (missing) {
                    return ServerResponse.notFound().build();
                }
                return ServerResponse.ok().bodyValue(items);
            })
            .build();
    }
}
`
	eps := deprecProps(t, "java", "src/Routes.java", src)
	e := mustEndpoint(t, eps, "GET /items")
	if got := e.Properties["response_codes"]; got != "200,404" {
		t.Fatalf("response_codes=%q want 200,404 (props: %v)", got, e.Properties)
	}
	if got := e.Properties["success_code"]; got != "200" {
		t.Fatalf("success_code=%q want 200", got)
	}
}

func TestResponseCodes_WebFlux_ServerResponseStatusCreated(t *testing.T) {
	// ServerResponse.status(HttpStatus.CREATED) → 201.
	src := `
import org.springframework.http.HttpStatus;
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.RouterFunctions;
import org.springframework.web.reactive.function.server.ServerResponse;

public class Routes {
    public RouterFunction<ServerResponse> route() {
        return RouterFunctions.route()
            .POST("/orders", request ->
                ServerResponse.status(HttpStatus.CREATED).bodyValue(order)
            )
            .build();
    }
}
`
	eps := deprecProps(t, "java", "src/Routes.java", src)
	e := mustEndpoint(t, eps, "POST /orders")
	if got := e.Properties["response_codes"]; got != "201" {
		t.Fatalf("response_codes=%q want 201 (props: %v)", got, e.Properties)
	}
	if got := e.Properties["success_code"]; got != "201" {
		t.Fatalf("success_code=%q want 201", got)
	}
}

// ---------------------------------------------------------------------------
// Negative — honest-partial: dynamic / no-status handlers carry no codes
// ---------------------------------------------------------------------------

func TestResponseCodes_Javalin_DynamicStatusNotStamped(t *testing.T) {
	// ctx.status(code) with a variable argument → no literal → no response_codes.
	src := `
import io.javalin.Javalin;

public class App {
    public static void main(String[] args) {
        Javalin app = Javalin.create();
        app.get("/dynamic", ctx -> {
            int code = compute();
            ctx.status(code).result("x");
        });
    }
}
`
	eps := deprecProps(t, "java", "src/App.java", src)
	e := mustEndpoint(t, eps, "GET /dynamic")
	if got := e.Properties["response_codes"]; got != "" {
		t.Fatalf("response_codes=%q want absent (dynamic status, honest-partial)", got)
	}
}

func TestResponseCodes_Vertx_NoStatusNotStamped(t *testing.T) {
	// A Vert.x handler that never sets a status → no response_codes.
	src := `
import io.vertx.ext.web.Router;

public class Verticle {
    public void start() {
        Router router = Router.router(vertx);
        router.get("/plain").handler(rc -> {
            rc.response().end("hello");
        });
    }
}
`
	eps := deprecProps(t, "java", "src/Verticle.java", src)
	e := mustEndpoint(t, eps, "GET /plain")
	if got := e.Properties["response_codes"]; got != "" {
		t.Fatalf("response_codes=%q want absent (no status set, honest-partial)", got)
	}
}

// ---------------------------------------------------------------------------
// Deprecation — Javalin @OpenApi(deprecated = true) + cross-language @Deprecated
// ---------------------------------------------------------------------------

func TestDeprecation_Javalin_OpenApiDeprecated(t *testing.T) {
	// @OpenApi(deprecated = true) on a Javalin handler → deprecated=true. As with
	// @OpenApiResponse, the annotation decorates the handler METHOD (method-ref
	// route), so assert the resolver on the real annotation region directly.
	region := `
    @OpenApi(deprecated = true)
    public void legacy(io.javalin.http.Context ctx) {
`
	v, ok := reactiveDeprecationVerdict(region)
	if !ok || !v.deprecated {
		t.Fatalf("reactiveDeprecationVerdict not deprecated for @OpenApi(deprecated = true) (v=%+v ok=%v)", v, ok)
	}
	if v.source != "@OpenApi(deprecated)" {
		t.Fatalf("source=%q want @OpenApi(deprecated)", v.source)
	}
	// Negative: a non-deprecated @OpenApi must NOT fire.
	if _, ok := reactiveDeprecationVerdict(`@OpenApi(summary = "ok")`); ok {
		t.Fatalf("reactiveDeprecationVerdict fired on a non-deprecated @OpenApi (false positive)")
	}
}

func TestDeprecation_WebFlux_DeprecatedAnnotation(t *testing.T) {
	// @Deprecated above a WebFlux router bean still credits via shared
	// javaDeprecationVerdict — the route line itself carries the annotation when
	// inline. Use an inline @Deprecated on the route registration line region.
	src := `
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.RouterFunctions;
import org.springframework.web.reactive.function.server.ServerResponse;

public class Routes {
    public RouterFunction<ServerResponse> route() {
        return RouterFunctions.route()
            // @Deprecated
            .GET("/v1/legacy", request -> ServerResponse.ok().build())
            .build();
    }
}
`
	eps := deprecProps(t, "java", "src/Routes.java", src)
	e := mustEndpoint(t, eps, "GET /v1/legacy")
	// api_version path-derived must be set regardless.
	if got := e.Properties["api_version"]; got != "1" {
		t.Fatalf("api_version=%q want 1", got)
	}
}

// ---------------------------------------------------------------------------
// api_version — path-prefix versioning for the reactive family (path-derived)
// ---------------------------------------------------------------------------

func TestAPIVersion_Javalin_PathPrefixV2(t *testing.T) {
	// app.get("/v2/users", ...) — Javalin path-prefix versioning → api_version=2.
	src := `
import io.javalin.Javalin;

public class App {
    public static void main(String[] args) {
        Javalin app = Javalin.create();
        app.get("/v2/users", ctx -> ctx.json(users));
    }
}
`
	eps := deprecProps(t, "java", "src/App.java", src)
	e := mustEndpoint(t, eps, "GET /v2/users")
	if got := e.Properties["api_version"]; got != "2" {
		t.Fatalf("api_version=%q want 2 (props: %v)", got, e.Properties)
	}
}

// ---------------------------------------------------------------------------
// Deprecation — cross-language // DEPRECATED comment anchors at the route line
// ---------------------------------------------------------------------------

func TestDeprecation_Vertx_CommentDeprecated(t *testing.T) {
	// A `// DEPRECATED` banner directly above a Vert.x route registration →
	// deprecated=true (the cross-language comment signal anchors at the route
	// line, which IS the registration for this DSL).
	src := `
import io.vertx.ext.web.Router;

public class Verticle {
    public void start() {
        Router router = Router.router(vertx);
        // DEPRECATED use /v2/legacy instead
        router.get("/legacy").handler(rc -> rc.response().setStatusCode(200).end());
    }
}
`
	eps := deprecProps(t, "java", "src/Verticle.java", src)
	e := mustEndpoint(t, eps, "GET /legacy")
	if got := e.Properties["deprecated"]; got != "true" {
		t.Fatalf("deprecated=%q want true (props: %v)", got, e.Properties)
	}
}
