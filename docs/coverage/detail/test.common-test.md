<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `test.common-test` — Common Test (CT)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [erlang](../by-language/erlang.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | ✅ `full` | `2026-06-14` | 4988 | `internal/custom/erlang/test_frameworks.go`<br>`internal/custom/erlang/test_frameworks_test.go` | #4930/#4988: common_test recognised by -include_lib("common_test/include/ct.hrl"), the *_SUITE.erl filename, or an all/0 export. Emits one test_suite (SCOPE.Pattern) per suite file carrying a TESTS edge to the module-under-test by naming convention (foo_SUITE.erl -> foo). Proven by TestErlangTestFrameworks_CommonTest. |
| Target extraction | ✅ `full` | `2026-06-14` | 4988 | `internal/custom/erlang/test_frameworks.go`<br>`internal/custom/erlang/test_frameworks_test.go` | #4930/#4988: each case(Config)/case(_Config) test-case function becomes a test_case entity, EXCLUDING the CT scaffolding callbacks (all/0, groups/0, suite/0, init_per_*/end_per_*) which are not test cases. Proven by TestErlangTestFrameworks_CommonTest (get_users + post_user cases; scaffolding excluded). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update test.common-test ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
