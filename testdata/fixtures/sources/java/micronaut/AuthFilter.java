package com.example.filter;

import io.micronaut.http.HttpRequest;
import io.micronaut.http.MutableHttpResponse;
import io.micronaut.http.annotation.Filter;
import io.micronaut.http.filter.HttpServerFilter;
import io.micronaut.http.filter.ServerFilterChain;
import org.reactivestreams.Publisher;

/**
 * Micronaut HttpServerFilter middleware example.
 *
 * @Filter("/**") binds this filter to all incoming HTTP requests.
 * Implements HttpServerFilter to participate in the filter chain.
 */
@Filter("/**")
public class AuthFilter implements HttpServerFilter {

    @Override
    public Publisher<MutableHttpResponse<?>> doFilter(
            HttpRequest<?> request, ServerFilterChain chain) {
        String token = request.getHeaders().get("Authorization");
        if (token == null || token.isBlank()) {
            // In a real app: return unauthorized response
        }
        return chain.proceed(request);
    }
}
