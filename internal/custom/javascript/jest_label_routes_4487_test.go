package javascript_test

import (
	"context"
	"strings"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"

	_ "github.com/cajasmota/grafel/internal/custom/javascript"
)

// Issue #4487 — contract/oracle specs name the endpoint under test in their
// describe()/it() label (`… — GET /api/v1/inspections/:id/…`) rather than
// driving it through supertest. The Jest extractor must surface those
// label-named routes on the suite's `e2e_route_calls` property so the shared
// resolve pass links the suite to the http_endpoint_definition. Without this
// the endpoints show Tests (0) even though a covering contract spec exists.

// e2eRouteCalls returns the e2e_route_calls property of the single test_suite
// the Jest extractor emits for src, or "" if none.
func e2eRouteCalls(t *testing.T, path, src string) string {
	t.Helper()
	e, ok := extreg.Get("custom_js_jest")
	if !ok {
		t.Fatal("jest extractor not registered")
	}
	ents, err := e.Extract(context.Background(), extreg.FileInput{
		Path: path, Language: "typescript", Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	for _, ent := range ents {
		if ent.Subtype == "test_suite" || ent.Kind == "test_suite" {
			if ent.Properties != nil {
				return ent.Properties["e2e_route_calls"]
			}
		}
	}
	return ""
}

// realCounterPartyContractSpec is a BYTE-COPY of the head of the live
// core-backend-v3 contract spec
// test/contract/inspections/inspection-counter-party-results.contract.spec.ts.
// It issues NO supertest call — the endpoint is named only in the describe
// label — which is exactly the shape that left endpoints showing Tests (0).
const realCounterPartyContractSpec = `import { assertSemanticPathParams } from '../request-contract';
import {
  mapInspectionCounterPartyResults,
  type CounterPartyResultsRaw,
} from '../../../src/modules/inspections/mappers/inspection-counter-party-results.mapper';

const COUNTER_PARTY_RESULTS_PATH = '/api/v1/inspections/:inspectionId/counter-party-results';

describe('contract: inspections counter-party-results — GET /api/v1/inspections/:id/counter-party-results', () => {
  it('path uses semantic :inspectionId param — not :pk or bare :id', () => {
    expect(COUNTER_PARTY_RESULTS_PATH).toMatch(/:inspectionId/);
  });
  it('success status is 200', () => {
    expect(200).toBe(200);
  });
  it('404 when inspection not found — verified in service via NotFoundException', () => {
    expect(true).toBe(true);
  });
});
`

// TestIssue4487_LabelRouteSurfacedFromRealContractSpec is the core RED→GREEN
// proof at the extractor boundary: the byte-copied contract spec yields the
// label-named route on e2e_route_calls even though it never calls supertest.
func TestIssue4487_LabelRouteSurfacedFromRealContractSpec(t *testing.T) {
	got := e2eRouteCalls(t, "test/contract/inspections/inspection-counter-party-results.contract.spec.ts", realCounterPartyContractSpec)
	if got == "" {
		t.Fatal("BEFORE→AFTER: e2e_route_calls is empty; contract spec route not surfaced from describe label")
	}
	want := "GET /api/v1/inspections/:id/counter-party-results"
	if !strings.Contains(got, want) {
		t.Fatalf("e2e_route_calls = %q, want it to contain %q", got, want)
	}
}

// TestIssue4487_LabelRoutesInDescribeAndIt covers both label positions and the
// common formats (em-dash separator in a describe, leading verb in an it) and
// asserts de-duplication of identical routes.
func TestIssue4487_LabelRoutesInDescribeAndIt(t *testing.T) {
	src := `describe('contract: clients list — GET /api/v1/clients', () => {
  it('GET /api/v1/clients returns 200 when authorized', () => { expect(1).toBe(1); });
  it('POST /api/v1/clients/:clientId/create_contact creates a contact', () => { expect(1).toBe(1); });
});`
	got := e2eRouteCalls(t, "test/contract/clients/client-list.contract.spec.ts", src)
	lines := splitNonEmpty(got)
	wantSet := map[string]bool{
		"GET /api/v1/clients":                              false,
		"POST /api/v1/clients/:clientId/create_contact":    false,
	}
	for _, ln := range lines {
		if _, ok := wantSet[ln]; ok {
			wantSet[ln] = true
		}
	}
	for w, seen := range wantSet {
		if !seen {
			t.Errorf("missing label route %q; got=%v", w, lines)
		}
	}
	// "GET /api/v1/clients" appears in BOTH the describe and an it label; it must
	// be emitted exactly once.
	n := 0
	for _, ln := range lines {
		if ln == "GET /api/v1/clients" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("expected de-duplicated single 'GET /api/v1/clients', got %d", n)
	}
}

// TestIssue4487_NoFalseRouteFromProse proves a label that mentions a verb word
// without a `/`-rooted path produces no spurious route.
func TestIssue4487_NoFalseRouteFromProse(t *testing.T) {
	src := `describe('mapper: getInspection transforms wire fields', () => {
  it('POST body is not relevant here', () => { expect(1).toBe(1); });
});`
	got := e2eRouteCalls(t, "x.spec.ts", src)
	if strings.TrimSpace(got) != "" {
		t.Fatalf("expected no label routes from prose, got %q", got)
	}
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			out = append(out, ln)
		}
	}
	return out
}
