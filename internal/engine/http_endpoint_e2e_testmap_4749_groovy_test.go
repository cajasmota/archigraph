package engine

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/types"

	// Register the Groovy route-hit extractor so the test_suite (carrying
	// e2e_route_calls) comes from the REAL extractor, not a hand-built fixture.
	_ "github.com/cajasmota/grafel/internal/custom/groovy"
)

// Issue #4749 LIVE-REPRO (resolve side) — Groovy Spock + Spring MockMvc route-hit
// tests. Proves end-to-end that a Groovy spec calling a route by string
// (`mockMvc.perform(get("/path"))`) links to the http_endpoint_definition it
// exercises — the Groovy slice of the all-language program (#4615/#4749). The
// shared linkE2ERouteTestsToEndpoints pass is language-agnostic; only the Groovy
// route capture + the Grails/Ratpack producer synthesis are new.

const groovySpockMockMvcSrc4749 = `
import spock.lang.Specification

class BookControllerSpec extends Specification {
  def "lists books"() {
    when:
    def result = mockMvc.perform(get("/api/v1/books"))
    then:
    result.andExpect(status().isOk())
  }

  def "creates a book"() {
    expect:
    mockMvc.perform(post("/api/v1/books"))
  }
}
`

func TestIssue4749_GroovySpockE2ERouteTestsLinkToEndpoints(t *testing.T) {
	defs := []types.EntityRecord{
		def("GET", "/api/v1/books"),
		def("POST", "/api/v1/books"),
	}
	suite := realSuite(t, "custom_groovy_tests_route_e2e",
		"src/test/groovy/BookControllerSpec.groovy", "groovy", groovySpockMockMvcSrc4749)

	afterOut, edges := runE2ERouteResolve(t, defs, suite)
	if edges < 2 {
		t.Fatalf("expected >=2 e2e route TESTS edges (GET + POST), got %d", edges)
	}
	assertGroovyRouteEdges(t, edgeTargets(afterOut))
}

func assertGroovyRouteEdges(t *testing.T, targets map[string]bool) {
	t.Helper()
	wantGet, wantPost := false, false
	for to := range targets {
		if strings.Contains(to, "GET:/api/v1/books") {
			wantGet = true
		}
		if strings.Contains(to, "POST:/api/v1/books") {
			wantPost = true
		}
	}
	if !wantGet {
		t.Errorf("expected a TESTS edge to GET /api/v1/books; targets=%v", targets)
	}
	if !wantPost {
		t.Errorf("expected a TESTS edge to POST /api/v1/books; targets=%v", targets)
	}
}

// Grails-convention endpoints synthesize verb ANY; a Spock integration test that
// hits the route by string (any verb) must still link via verbsMatchCompat(ANY).
func TestIssue4749_GroovyGrailsAnyVerbE2ELink(t *testing.T) {
	defs := []types.EntityRecord{
		def("ANY", "/book/show"),
	}
	src := `
class BookFunctionalSpec extends Specification {
  def "fetches a book"() {
    when:
    def resp = get "$baseUrl/book/show"
    then:
    resp.status == 200
  }
}
`
	suite := realSuite(t, "custom_groovy_tests_route_e2e",
		"src/integration-test/groovy/BookFunctionalSpec.groovy", "groovy", src)
	afterOut, edges := runE2ERouteResolve(t, defs, suite)
	if edges < 1 {
		t.Fatalf("expected >=1 TESTS edge to the ANY-verb Grails endpoint, got %d", edges)
	}
	found := false
	for to := range edgeTargets(afterOut) {
		if strings.Contains(to, ":/book/show") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a TESTS edge to /book/show; targets=%v", edgeTargets(afterOut))
	}
}
