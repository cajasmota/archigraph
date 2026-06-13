package erlang

import (
	"context"
	"testing"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func runTestFrameworks(t *testing.T, path, lang, src string) []types.EntityRecord {
	t.Helper()
	e := &erlangTestFrameworksExtractor{}
	out, err := e.Extract(context.Background(), extractor.FileInput{
		Path:     path,
		Language: lang,
		Content:  []byte(src),
	})
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	return out
}

func findBySubtype(recs []types.EntityRecord, subtype string) []types.EntityRecord {
	var out []types.EntityRecord
	for _, r := range recs {
		if r.Subtype == subtype {
			out = append(out, r)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// eunit — happy path
// ---------------------------------------------------------------------------

func TestErlangTestFrameworks_Eunit(t *testing.T) {
	src := `-module(calc_tests).
-include_lib("eunit/include/eunit.hrl").

add_test() ->
    ?assertEqual(3, calc:add(1, 2)).

sub_test() ->
    ?assertEqual(1, calc:sub(2, 1)).

range_test_() ->
    [?_assertEqual(N, N) || N <- lists:seq(1, 3)].
`
	recs := runTestFrameworks(t, "/proj/test/calc_tests.erl", "erlang", src)

	cases := findBySubtype(recs, "test_case")
	if len(cases) != 3 {
		t.Fatalf("expected 3 eunit test cases, got %d (%v)", len(cases), cases)
	}
	gen := 0
	for _, c := range cases {
		if c.Properties["framework"] != "eunit" {
			t.Errorf("case %s framework=%q want eunit", c.Name, c.Properties["framework"])
		}
		if c.Properties["test_kind"] == "eunit_generator" {
			gen++
		}
		if c.Properties["module_under_test"] != "calc" {
			t.Errorf("case %s module_under_test=%q want calc", c.Name, c.Properties["module_under_test"])
		}
	}
	if gen != 1 {
		t.Errorf("expected exactly 1 generator (range_test_), got %d", gen)
	}

	suites := findBySubtype(recs, "test_suite")
	if len(suites) != 1 {
		t.Fatalf("expected 1 suite, got %d", len(suites))
	}
	suite := suites[0]
	if suite.Properties["framework"] != "eunit" {
		t.Errorf("suite framework=%q want eunit", suite.Properties["framework"])
	}
	if suite.Properties["module_under_test"] != "calc" {
		t.Errorf("suite module_under_test=%q want calc", suite.Properties["module_under_test"])
	}
	// TESTS edge to the SUT module by naming convention.
	var hasTests bool
	for _, rel := range suite.Relationships {
		if rel.Kind == "TESTS" && rel.ToID == "calc" {
			hasTests = true
		}
	}
	if !hasTests {
		t.Errorf("suite missing TESTS edge to calc (rels: %v)", suite.Relationships)
	}
}

// ---------------------------------------------------------------------------
// common_test — happy path
// ---------------------------------------------------------------------------

func TestErlangTestFrameworks_CommonTest(t *testing.T) {
	src := `-module(http_SUITE).
-include_lib("common_test/include/ct.hrl").
-export([all/0, groups/0, init_per_suite/1, end_per_suite/1]).

all() ->
    [get_users, post_user].

groups() ->
    [].

init_per_suite(Config) ->
    Config.

end_per_suite(_Config) ->
    ok.

get_users(Config) ->
    ?assert(true).

post_user(_Config) ->
    ?assert(true).
`
	recs := runTestFrameworks(t, "/proj/test/http_SUITE.erl", "erlang", src)

	cases := findBySubtype(recs, "test_case")
	names := map[string]bool{}
	for _, c := range cases {
		names[c.Name] = true
		if c.Properties["framework"] != "common_test" {
			t.Errorf("case %s framework=%q want common_test", c.Name, c.Properties["framework"])
		}
	}
	if !names["test:get_users"] || !names["test:post_user"] {
		t.Errorf("expected get_users + post_user cases, got %v", names)
	}
	// Scaffolding callbacks must be excluded.
	for _, bad := range []string{"test:all", "test:groups", "test:init_per_suite", "test:end_per_suite"} {
		if names[bad] {
			t.Errorf("scaffolding callback %q must NOT be a test case", bad)
		}
	}

	suites := findBySubtype(recs, "test_suite")
	if len(suites) != 1 {
		t.Fatalf("expected 1 suite, got %d", len(suites))
	}
	if suites[0].Properties["module_under_test"] != "http" {
		t.Errorf("suite module_under_test=%q want http", suites[0].Properties["module_under_test"])
	}
	var hasTests bool
	for _, rel := range suites[0].Relationships {
		if rel.Kind == "TESTS" && rel.ToID == "http" {
			hasTests = true
		}
	}
	if !hasTests {
		t.Errorf("suite missing TESTS edge to http SUT")
	}
}

// ---------------------------------------------------------------------------
// Negative: wrong-language file → no-op
// ---------------------------------------------------------------------------

func TestErlangTestFrameworks_WrongLanguageNoOp(t *testing.T) {
	src := `-module(calc_tests).
-include_lib("eunit/include/eunit.hrl").
add_test() -> ?assertEqual(3, calc:add(1,2)).
`
	recs := runTestFrameworks(t, "/proj/test/calc_tests.erl", "python", src)
	if len(recs) != 0 {
		t.Errorf("wrong language must be a no-op, got %d records", len(recs))
	}
}

// ---------------------------------------------------------------------------
// Negative: no-match (non-test erlang file) → no-op
// ---------------------------------------------------------------------------

func TestErlangTestFrameworks_NoMatchNoOp(t *testing.T) {
	src := `-module(calc).
-export([add/2]).
add(A, B) -> A + B.
`
	recs := runTestFrameworks(t, "/proj/src/calc.erl", "erlang", src)
	if len(recs) != 0 {
		t.Errorf("non-test erlang module must be a no-op, got %d records (%v)", len(recs), recs)
	}
}

// A file with the framework include but no test functions emits nothing.
func TestErlangTestFrameworks_SignalButNoTestsNoOp(t *testing.T) {
	src := `-module(helpers_tests).
-include_lib("eunit/include/eunit.hrl").
% helper, not a test
make_fixture() -> {ok, []}.
`
	recs := runTestFrameworks(t, "/proj/test/helpers_tests.erl", "erlang", src)
	if len(recs) != 0 {
		t.Errorf("eunit file with no *_test functions must be a no-op, got %v", recs)
	}
}
