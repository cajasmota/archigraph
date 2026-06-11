package engine

import "testing"

// TestGrails_ConventionRoutes covers the Grails controller convention: each
// action method maps to `/<controller>/<action>` with verb ANY.
func TestGrails_ConventionRoutes(t *testing.T) {
	src := `
package com.x

class BookController {
    def index() { render Book.list() }
    def show(Long id) { render Book.get(id) }
    def save() { new Book(params).save() }

    def beforeInterceptor = { log.info("before") }
}
`
	ids, _ := runDetect(t, "groovy", "grails-app/controllers/com/x/BookController.groovy", src)
	requireContains(t, ids, []string{
		"http:ANY:/book/index",
		"http:ANY:/book/show",
		"http:ANY:/book/save",
	}, "grails-convention")
	// beforeInterceptor is a lifecycle hook, NOT an action — must not synthesize.
	for _, id := range ids {
		if id == "http:ANY:/book/beforeInterceptor" {
			t.Fatalf("grails: lifecycle hook beforeInterceptor must not be an endpoint; ids=%v", ids)
		}
	}
}

// TestGrails_UrlMappings covers explicit UrlMappings.groovy with dollar-params
// and an explicit method.
func TestGrails_UrlMappings(t *testing.T) {
	src := `
class UrlMappings {
    static mappings = {
        "/book/$id"(controller: "book", action: "show")
        "/book"(controller: "book", action: "save", method: "POST")
        "/author/$name/books"(controller: "author", action: "books")
    }
}
`
	ids, _ := runDetect(t, "groovy", "grails-app/controllers/UrlMappings.groovy", src)
	requireContains(t, ids, []string{
		"http:ANY:/book/{id}",
		"http:POST:/book",
		"http:ANY:/author/{name}/books",
	}, "grails-urlmappings")
}

// TestRatpack_HandlerDSL covers the Ratpack verb-DSL handler routes.
func TestRatpack_HandlerDSL(t *testing.T) {
	src := `
import ratpack.server.RatpackServer

RatpackServer.start { server ->
    server.handlers { chain ->
        chain.get("api/books") { ctx -> ctx.render("books") }
        chain.get("api/book/:id") { ctx -> ctx.render("book") }
        chain.post("api/books") { ctx -> ctx.render("created") }
    }
}
`
	ids, _ := runDetect(t, "groovy", "src/ratpack/ratpack.groovy", src)
	requireContains(t, ids, []string{
		"http:GET:/api/books",
		"http:GET:/api/book/{id}",
		"http:POST:/api/books",
	}, "ratpack-handler-dsl")
}

// TestRatpack_InterpolatedDropped is the honest-exclusion guard.
func TestRatpack_InterpolatedDropped(t *testing.T) {
	src := `
import ratpack.server.RatpackServer
RatpackServer.start { server ->
    server.handlers { chain ->
        chain.get("api/static") { ctx -> ctx.render("ok") }
        chain.get("${dynamicPrefix}/x") { ctx -> ctx.render("dyn") }
    }
}
`
	ids, _ := runDetect(t, "groovy", "src/ratpack/ratpack.groovy", src)
	requireContains(t, ids, []string{"http:GET:/api/static"}, "ratpack-static")
	for _, id := range ids {
		if id == "http:GET:/${dynamicPrefix}/x" || id == "http:GET:/{dynamicPrefix}/x" {
			t.Fatalf("ratpack: interpolated route must be dropped; ids=%v", ids)
		}
	}
}
