package com.example.webflux;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.MediaType;
import org.springframework.web.reactive.function.server.RequestPredicates;
import org.springframework.web.reactive.function.server.RouterFunction;
import org.springframework.web.reactive.function.server.RouterFunctions;
import org.springframework.web.reactive.function.server.ServerResponse;
import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.server.WebFilter;
import org.springframework.web.server.WebFilterChain;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

/**
 * Spring WebFlux functional routing fixture for #3080.
 *
 * Demonstrates:
 *   - Functional DSL route registration via RouterFunctions.route() chained builder
 *     (route_extraction, endpoint_synthesis)
 *   - Functional DSL route registration via RouterFunctions.route(predicate, handler)
 *     two-argument overload (route_extraction)
 *   - WebFilter middleware implementation (middleware_coverage)
 *   - Path parameters in {param} curly-brace style
 */
@Configuration
public class RouterConfig {

    private final UserHandler userHandler;
    private final OrderHandler orderHandler;

    public RouterConfig(UserHandler userHandler, OrderHandler orderHandler) {
        this.userHandler = userHandler;
        this.orderHandler = orderHandler;
    }

    // -----------------------------------------------------------------------
    // Functional DSL — chained builder form (route_extraction)
    // -----------------------------------------------------------------------
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

    // -----------------------------------------------------------------------
    // Functional DSL — two-argument predicate form (route_extraction)
    // -----------------------------------------------------------------------
    @Bean
    public RouterFunction<ServerResponse> orderRoutes() {
        return RouterFunctions
                .route(RequestPredicates.GET("/orders"), orderHandler::listOrders)
                .andRoute(RequestPredicates.POST("/orders"), orderHandler::createOrder)
                .andRoute(RequestPredicates.GET("/orders/{orderId}"), orderHandler::getOrder)
                .andRoute(RequestPredicates.PUT("/orders/{orderId}"), orderHandler::updateOrder)
                .andRoute(RequestPredicates.DELETE("/orders/{orderId}"), orderHandler::cancelOrder);
    }

    // -----------------------------------------------------------------------
    // Nested router with path prefix (route_extraction)
    // -----------------------------------------------------------------------
    @Bean
    public RouterFunction<ServerResponse> adminRoutes() {
        return RouterFunctions.route()
                .path("/admin", builder -> builder
                        .GET("/stats", req -> ServerResponse.ok()
                                .contentType(MediaType.APPLICATION_JSON)
                                .bodyValue("{}"))
                        .DELETE("/cache", req -> ServerResponse.ok().build()))
                .build();
    }
}

// -----------------------------------------------------------------------
// WebFilter middleware (middleware_coverage)
// -----------------------------------------------------------------------

/**
 * RequestLoggingFilter logs every incoming request before it reaches a handler.
 * Implements WebFilter to plug into the reactive filter chain.
 */
class RequestLoggingFilter implements WebFilter {

    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        System.out.printf("[REQ] %s %s%n",
                exchange.getRequest().getMethod(),
                exchange.getRequest().getPath());
        return chain.filter(exchange);
    }
}

/**
 * AuthenticationFilter validates the Authorization header for all requests
 * under /api/**. Demonstrates middleware_coverage with conditional logic.
 */
class AuthenticationFilter implements WebFilter {

    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        String path = exchange.getRequest().getPath().value();
        if (!path.startsWith("/api/")) {
            return chain.filter(exchange);
        }
        String authHeader = exchange.getRequest().getHeaders().getFirst("Authorization");
        if (authHeader == null || !authHeader.startsWith("Bearer ")) {
            exchange.getResponse().setStatusCode(org.springframework.http.HttpStatus.UNAUTHORIZED);
            return exchange.getResponse().setComplete();
        }
        return chain.filter(exchange);
    }
}

// Placeholder handler classes — present to make the file self-contained.
class UserHandler {
    public Mono<ServerResponse> listUsers(
            org.springframework.web.reactive.function.server.ServerRequest req) {
        return ServerResponse.ok().bodyValue(Flux.empty());
    }
    public Mono<ServerResponse> createUser(
            org.springframework.web.reactive.function.server.ServerRequest req) {
        return ServerResponse.status(201).build();
    }
    public Mono<ServerResponse> getUser(
            org.springframework.web.reactive.function.server.ServerRequest req) {
        return ServerResponse.ok().build();
    }
    public Mono<ServerResponse> updateUser(
            org.springframework.web.reactive.function.server.ServerRequest req) {
        return ServerResponse.ok().build();
    }
    public Mono<ServerResponse> deleteUser(
            org.springframework.web.reactive.function.server.ServerRequest req) {
        return ServerResponse.noContent().build();
    }
}

class OrderHandler {
    public Mono<ServerResponse> listOrders(
            org.springframework.web.reactive.function.server.ServerRequest req) {
        return ServerResponse.ok().bodyValue(Flux.empty());
    }
    public Mono<ServerResponse> createOrder(
            org.springframework.web.reactive.function.server.ServerRequest req) {
        return ServerResponse.status(201).build();
    }
    public Mono<ServerResponse> getOrder(
            org.springframework.web.reactive.function.server.ServerRequest req) {
        return ServerResponse.ok().build();
    }
    public Mono<ServerResponse> updateOrder(
            org.springframework.web.reactive.function.server.ServerRequest req) {
        return ServerResponse.ok().build();
    }
    public Mono<ServerResponse> cancelOrder(
            org.springframework.web.reactive.function.server.ServerRequest req) {
        return ServerResponse.noContent().build();
    }
}
