package groovy_test

import (
	"context"
	"strings"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"

	_ "github.com/cajasmota/grafel/internal/custom/groovy"
)

func fi(path, lang, src string) extreg.FileInput {
	return extreg.FileInput{Path: path, Language: lang, Content: []byte(src)}
}

func extractSuite(t *testing.T, path, src string) (string, bool) {
	t.Helper()
	e, ok := extreg.Get("custom_groovy_tests_route_e2e")
	if !ok {
		t.Fatal("custom_groovy_tests_route_e2e not registered")
	}
	ents, err := e.Extract(context.Background(), fi(path, "groovy", src))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(ents) == 0 {
		return "", false
	}
	if len(ents) != 1 {
		t.Fatalf("expected exactly 1 test_suite, got %d", len(ents))
	}
	if ents[0].Subtype != "test_suite" {
		t.Errorf("expected test_suite, got %q", ents[0].Subtype)
	}
	return ents[0].Properties["e2e_route_calls"], true
}

// Spock + Spring MockMvc route hits are captured onto a single test_suite.
func TestGroovyRouteE2E_SpockMockMvc(t *testing.T) {
	src := `
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
	calls, ok := extractSuite(t, "src/test/groovy/BookControllerSpec.groovy", src)
	if !ok {
		t.Fatal("expected a test_suite")
	}
	for _, want := range []string{"GET /api/v1/books", "POST /api/v1/books"} {
		if !strings.Contains(calls, want) {
			t.Errorf("expected route call %q in %q", want, calls)
		}
	}
}

// Ratpack test-client route hits are captured.
func TestGroovyRouteE2E_RatpackClient(t *testing.T) {
	src := `
import ratpack.test.embed.EmbeddedApp

class HealthSpec extends Specification {
  def "health is ok"() {
    when:
    def resp = testHttpClient.get("api/health")
    then:
    resp.statusCode == 200
  }
}
`
	calls, ok := extractSuite(t, "src/test/groovy/HealthSpec.groovy", src)
	if !ok {
		t.Fatal("expected a test_suite")
	}
	if !strings.Contains(calls, "GET /api/health") {
		t.Errorf("expected GET /api/health in %q", calls)
	}
}

// Grails RestBuilder bare-verb form (`get "$baseUrl/books"`) keeps the static path.
func TestGroovyRouteE2E_GrailsBareVerb(t *testing.T) {
	src := `
class BookFunctionalSpec extends Specification {
  def "fetches books"() {
    when:
    def resp = get "$baseUrl/books"
    then:
    resp.status == 200
  }
}
`
	calls, ok := extractSuite(t, "src/integration-test/groovy/BookFunctionalSpec.groovy", src)
	if !ok {
		t.Fatal("expected a test_suite")
	}
	if !strings.Contains(calls, "GET /books") {
		t.Errorf("expected GET /books in %q", calls)
	}
}

// A pure unit spec with no route hit emits NO suite (honest exclusion).
func TestGroovyRouteE2E_NoRouteNoSuite(t *testing.T) {
	src := `
class CalculatorSpec extends Specification {
  def "adds"() {
    expect:
    1 + 1 == 2
  }
}
`
	if _, ok := extractSuite(t, "src/test/groovy/CalculatorSpec.groovy", src); ok {
		t.Error("expected no test_suite for a routeless spec")
	}
}

// A non-test file is never turned into a suite (the route-string would be a
// production route, not a test hit).
func TestGroovyRouteE2E_NonTestFileSkipped(t *testing.T) {
	src := `class BookController { def index() { get "/books" } }`
	if _, ok := extractSuite(t, "grails-app/controllers/BookController.groovy", src); ok {
		t.Error("expected no test_suite for a non-test file")
	}
}

// A fully-interpolated route (`"${url}"`) is dropped — no static path.
func TestGroovyRouteE2E_InterpolatedDropped(t *testing.T) {
	src := `
class XSpec extends Specification {
  def "x"() {
    when:
    mockMvc.perform(get("/api/books"))
    def r = testHttpClient.get("${dynamicPath}")
    then:
    true
  }
}
`
	calls, ok := extractSuite(t, "src/test/groovy/XSpec.groovy", src)
	if !ok {
		t.Fatal("expected a test_suite (the static /api/books hit)")
	}
	if !strings.Contains(calls, "GET /api/books") {
		t.Errorf("expected GET /api/books in %q", calls)
	}
	if strings.Contains(calls, "${") {
		t.Errorf("interpolated route must be dropped; got %q", calls)
	}
}
