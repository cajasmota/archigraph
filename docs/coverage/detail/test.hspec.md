<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `test.hspec` — hspec (Haskell testing)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [haskell](../by-language/haskell.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | 🟢 `partial` | `2026-06-24` | 5373 | `internal/extractors/haskell/depth.go`<br>`internal/extractors/haskell/depth_test.go` | Each hspec suite carries a stem-affinity TESTS edge to the module its filename names (UserSpec→User). Honest partial: direct-call resolution from inside the it() body and e2e route TESTS edges (test→HTTP endpoint) are not yet wired for Haskell — follow-up. |
| Target extraction | ✅ `full` | `2026-06-24` | 5373 | `internal/extractors/haskell/depth.go`<br>`internal/extractors/haskell/depth_test.go` | hspec describe/context/it/specify spec blocks (hspecBlockRE) are lifted into one SCOPE.Operation(subtype=test_suite) per *Spec.hs / *Test.hs file (or any file importing Test.Hspec), carrying example_count (it+specify leaves) and framework=hspec. A stem-affinity TESTS edge links the suite to the tested module (UserSpec→User). An example-less spec emits no suite (honest). Proven by TestHspec_SpecSuite + TestHspec_ExampleLessNoEmit. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update test.hspec ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
