package java

// Dedicated tests for the Spring WebFlux functional routing + WebFilter
// extractor (issue #3080).
//
// These tests validate ExtractSpringWebFlux in spring_webflux_routes.go.
// DO NOT add these tests to extractors_test.go.

import (
	"testing"
)

// ctx constructs a PatternContext for spring_webflux framework.
func webfluxCtx(source, file string) PatternContext {
	return PatternContext{
		Source:    source,
		Language:  "java",
		Framework: "spring_webflux",
		FilePath:  file,
	}
}

// hasRouteWithVerb returns true if result contains at least one Route entity
// whose http_verb property equals verb and whose Name (path) equals path.
func hasRouteWithVerb(result PatternResult, verb, path string) bool {
	for _, e := range result.Entities {
		if e.Kind != "Route" {
			continue
		}
		if e.Name != path {
			continue
		}
		if v, ok := e.Properties["http_verb"]; ok && v == verb {
			return true
		}
	}
	return false
}

// hasMiddlewareClass returns true if result contains a Middleware entity for
// className.
func hasMiddlewareClass(result PatternResult, className string) bool {
	for _, e := range result.Entities {
		if e.Kind != "Middleware" {
			continue
		}
		if e.Name == className {
			return true
		}
	}
	return false
}

// ============================================================================
// route_extraction — chained builder DSL
// ============================================================================

func TestExtractSpringWebFlux_ChainedRoutes(t *testing.T) {
	src := `package com.example;

import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.RouterFunctions;
import org.springframework.web.reactive.function.server.ServerResponse;

@Configuration
public class RouterConfig {
    @Bean
    public RouterFunction<ServerResponse> routes() {
        return RouterFunctions.route()
                .GET("/users", handler::listUsers)
                .POST("/users", handler::createUser)
                .GET("/users/{id}", handler::getUser)
                .PUT("/users/{id}", handler::updateUser)
                .DELETE("/users/{id}", handler::deleteUser)
                .PATCH("/users/{id}/status", handler::patchStatus)
                .build();
    }
}
`
	result := ExtractSpringWebFlux(webfluxCtx(src, "RouterConfig.java"))

	want := []struct{ verb, path string }{
		{"GET", "/users"},
		{"POST", "/users"},
		{"GET", "/users/{id}"},
		{"PUT", "/users/{id}"},
		{"DELETE", "/users/{id}"},
		{"PATCH", "/users/{id}/status"},
	}
	for _, w := range want {
		if !hasRouteWithVerb(result, w.verb, w.path) {
			t.Errorf("expected Route %s %s, not found in %+v", w.verb, w.path, entitySummary(result))
		}
	}
}

// ============================================================================
// route_extraction — two-argument predicate DSL
// ============================================================================

func TestExtractSpringWebFlux_PredicateRoutes(t *testing.T) {
	src := `package com.example;

import org.springframework.web.reactive.function.server.RequestPredicates;
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.RouterFunctions;
import org.springframework.web.reactive.function.server.ServerResponse;

@Configuration
public class OrderRouter {
    @Bean
    public RouterFunction<ServerResponse> orderRoutes() {
        return RouterFunctions
                .route(RequestPredicates.GET("/orders"), handler::listOrders)
                .andRoute(RequestPredicates.POST("/orders"), handler::createOrder)
                .andRoute(RequestPredicates.GET("/orders/{orderId}"), handler::getOrder)
                .andRoute(RequestPredicates.DELETE("/orders/{orderId}"), handler::cancelOrder);
    }
}
`
	result := ExtractSpringWebFlux(webfluxCtx(src, "OrderRouter.java"))

	want := []struct{ verb, path string }{
		{"GET", "/orders"},
		{"POST", "/orders"},
		{"GET", "/orders/{orderId}"},
		{"DELETE", "/orders/{orderId}"},
	}
	for _, w := range want {
		if !hasRouteWithVerb(result, w.verb, w.path) {
			t.Errorf("expected Route %s %s, not found in %+v", w.verb, w.path, entitySummary(result))
		}
	}
}

// ============================================================================
// route_extraction — path parameters canonicalized
// ============================================================================

func TestExtractSpringWebFlux_PathParams(t *testing.T) {
	src := `package com.example;

import org.springframework.web.reactive.function.server.RouterFunctions;
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.ServerResponse;

@Configuration
public class OrgRouter {
    @Bean
    public RouterFunction<ServerResponse> routes() {
        return RouterFunctions.route()
                .GET("/orgs/{orgId}/repos/{repoId}", handler::getRepo)
                .DELETE("/orgs/{orgId}/repos/{repoId}", handler::deleteRepo)
                .build();
    }
}
`
	result := ExtractSpringWebFlux(webfluxCtx(src, "OrgRouter.java"))

	if !hasRouteWithVerb(result, "GET", "/orgs/{orgId}/repos/{repoId}") {
		t.Errorf("expected GET /orgs/{orgId}/repos/{repoId}, got %+v", entitySummary(result))
	}
	if !hasRouteWithVerb(result, "DELETE", "/orgs/{orgId}/repos/{repoId}") {
		t.Errorf("expected DELETE /orgs/{orgId}/repos/{repoId}, got %+v", entitySummary(result))
	}
}

// ============================================================================
// route_extraction — provenance + framework property
// ============================================================================

func TestExtractSpringWebFlux_RouteProperties(t *testing.T) {
	src := `package com.example;

import org.springframework.web.reactive.function.server.RouterFunctions;
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.ServerResponse;

public class Cfg {
    @Bean
    public RouterFunction<ServerResponse> routes() {
        return RouterFunctions.route()
                .GET("/health", req -> ServerResponse.ok().build())
                .build();
    }
}
`
	result := ExtractSpringWebFlux(webfluxCtx(src, "Cfg.java"))
	if len(result.Entities) == 0 {
		t.Fatal("expected at least one entity")
	}
	var found *SecondaryEntity
	for i := range result.Entities {
		if result.Entities[i].Kind == "Route" && result.Entities[i].Name == "/health" {
			found = &result.Entities[i]
			break
		}
	}
	if found == nil {
		t.Fatal("Route /health not found")
	}
	if found.Properties["framework"] != "spring_webflux" {
		t.Errorf("expected framework=spring_webflux, got %v", found.Properties["framework"])
	}
	if found.Provenance == "" {
		t.Error("expected non-empty provenance")
	}
}

// ============================================================================
// middleware_coverage — WebFilter implementation
// ============================================================================

func TestExtractSpringWebFlux_WebFilter(t *testing.T) {
	src := `package com.example;

import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.server.WebFilter;
import org.springframework.web.server.WebFilterChain;
import reactor.core.publisher.Mono;

public class RequestLoggingFilter implements WebFilter {
    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        System.out.println(exchange.getRequest().getPath());
        return chain.filter(exchange);
    }
}
`
	result := ExtractSpringWebFlux(webfluxCtx(src, "RequestLoggingFilter.java"))

	if !hasMiddlewareClass(result, "RequestLoggingFilter") {
		t.Errorf("expected Middleware entity for RequestLoggingFilter, got %+v", entitySummary(result))
	}
}

func TestExtractSpringWebFlux_MultipleWebFilters(t *testing.T) {
	src := `package com.example;

import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.server.WebFilter;
import org.springframework.web.server.WebFilterChain;
import reactor.core.publisher.Mono;

public class AuthFilter implements WebFilter {
    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        return chain.filter(exchange);
    }
}

class RateLimitFilter implements WebFilter {
    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        return chain.filter(exchange);
    }
}
`
	result := ExtractSpringWebFlux(webfluxCtx(src, "Filters.java"))

	if !hasMiddlewareClass(result, "AuthFilter") {
		t.Errorf("expected Middleware entity for AuthFilter, got %+v", entitySummary(result))
	}
	if !hasMiddlewareClass(result, "RateLimitFilter") {
		t.Errorf("expected Middleware entity for RateLimitFilter, got %+v", entitySummary(result))
	}
}

// ============================================================================
// middleware_coverage — properties
// ============================================================================

func TestExtractSpringWebFlux_WebFilterProperties(t *testing.T) {
	src := `package com.example;

import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.server.WebFilter;
import org.springframework.web.server.WebFilterChain;
import reactor.core.publisher.Mono;

public class CorsFilter implements WebFilter {
    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        return chain.filter(exchange);
    }
}
`
	result := ExtractSpringWebFlux(webfluxCtx(src, "CorsFilter.java"))
	var found *SecondaryEntity
	for i := range result.Entities {
		if result.Entities[i].Kind == "Middleware" {
			found = &result.Entities[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected Middleware entity")
	}
	if found.Properties["middleware_type"] != "web_filter" {
		t.Errorf("expected middleware_type=web_filter, got %v", found.Properties["middleware_type"])
	}
	if found.Properties["framework"] != "spring_webflux" {
		t.Errorf("expected framework=spring_webflux, got %v", found.Properties["framework"])
	}
}

// ============================================================================
// Quick-exit: no-op for non-WebFlux files
// ============================================================================

func TestExtractSpringWebFlux_NoSignalNoOp(t *testing.T) {
	src := `package com.example;

public class OrderService {
    public String findById(long id) { return null; }
}
`
	result := ExtractSpringWebFlux(webfluxCtx(src, "OrderService.java"))
	if len(result.Entities) != 0 {
		t.Errorf("expected 0 entities for plain Java class, got %d: %+v",
			len(result.Entities), entitySummary(result))
	}
}

func TestExtractSpringWebFlux_WrongLanguageNoOp(t *testing.T) {
	ctx := PatternContext{
		Source:    "RouterFunctions.route().GET(\"/users\", h)",
		Language:  "python",
		Framework: "spring_webflux",
		FilePath:  "app.py",
	}
	result := ExtractSpringWebFlux(ctx)
	if len(result.Entities) != 0 {
		t.Errorf("expected 0 entities for wrong language, got %d", len(result.Entities))
	}
}

// ============================================================================
// Fixture file smoke test
// ============================================================================

func TestExtractSpringWebFlux_FixtureFile(t *testing.T) {
	src := `package com.example.webflux;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.reactive.function.server.RequestPredicates;
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.RouterFunctions;
import org.springframework.web.reactive.function.server.ServerResponse;
import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.server.WebFilter;
import org.springframework.web.server.WebFilterChain;
import reactor.core.publisher.Mono;

@Configuration
public class RouterConfig {
    @Bean
    public RouterFunction<ServerResponse> userRoutes() {
        return RouterFunctions.route()
                .GET("/users", userHandler::listUsers)
                .POST("/users", userHandler::createUser)
                .GET("/users/{id}", userHandler::getUser)
                .PUT("/users/{id}", userHandler::updateUser)
                .DELETE("/users/{id}", userHandler::deleteUser)
                .build();
    }

    @Bean
    public RouterFunction<ServerResponse> orderRoutes() {
        return RouterFunctions
                .route(RequestPredicates.GET("/orders"), orderHandler::listOrders)
                .andRoute(RequestPredicates.POST("/orders"), orderHandler::createOrder);
    }
}

class RequestLoggingFilter implements WebFilter {
    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        return chain.filter(exchange);
    }
}

class AuthenticationFilter implements WebFilter {
    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        return chain.filter(exchange);
    }
}
`
	result := ExtractSpringWebFlux(webfluxCtx(src, "RouterConfig.java"))

	wantRoutes := []struct{ verb, path string }{
		{"GET", "/users"},
		{"POST", "/users"},
		{"GET", "/users/{id}"},
		{"PUT", "/users/{id}"},
		{"DELETE", "/users/{id}"},
		{"GET", "/orders"},
		{"POST", "/orders"},
	}
	for _, w := range wantRoutes {
		if !hasRouteWithVerb(result, w.verb, w.path) {
			t.Errorf("fixture: expected Route %s %s, got %+v", w.verb, w.path, entitySummary(result))
		}
	}

	wantFilters := []string{"RequestLoggingFilter", "AuthenticationFilter"}
	for _, cls := range wantFilters {
		if !hasMiddlewareClass(result, cls) {
			t.Errorf("fixture: expected Middleware %s, got %+v", cls, entitySummary(result))
		}
	}
}

// ============================================================================
// helpers
// ============================================================================

func entitySummary(result PatternResult) []string {
	var s []string
	for _, e := range result.Entities {
		s = append(s, e.Kind+":"+e.Name)
	}
	return s
}
