<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `test.eunit` — EUnit

Auto-generated. Back to [summary](../summary.md).

- **Language:** [erlang](../by-language/erlang.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | ✅ `full` | `2026-06-14` | 4988 | `internal/custom/erlang/test_frameworks.go`<br>`internal/custom/erlang/test_frameworks_test.go` | #4930/#4988: eunit recognised by -include_lib("eunit/include/eunit.hrl") or the *_tests.erl filename convention. Emits one test_suite (SCOPE.Pattern) per test file carrying a TESTS edge to the module-under-test resolved by the naming convention (foo_tests.erl -> foo). Proven by TestErlangTestFrameworks_Eunit. |
| Target extraction | ✅ `full` | `2026-06-14` | 4988 | `internal/custom/erlang/test_frameworks.go`<br>`internal/custom/erlang/test_frameworks_test.go` | #4930/#4988: each name_test/0 (simple test) and name_test_/0 (test generator, test_kind=eunit_generator) function becomes a test_case entity (SCOPE.Pattern; Properties[framework=eunit, test_function, test_kind, module_under_test]). A file with the eunit include but no *_test functions is a no-op (TestErlangTestFrameworks_SignalButNoTestsNoOp). Proven by TestErlangTestFrameworks_Eunit. NOTE: a complementary route-hit test->endpoint linkage for eunit/CT exists via internal/custom/erlang/tests_route_e2e.go (#4749). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update test.eunit ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
