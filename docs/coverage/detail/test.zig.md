<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `test.zig` — zig test

Auto-generated. Back to [summary](../summary.md).

- **Language:** [zig](../by-language/zig.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | ✅ `full` | `2026-06-24` | 5377 | `internal/extractors/cross/testmap/frameworks.go`<br>`internal/extractors/cross/testmap/frameworks_zig.go`<br>`internal/extractors/cross/testmap/frameworks_zig_test.go`<br>`internal/extractors/cross/testmap/resolver.go` | zig test linkage via the cross-language testmap extractor. detectZigTest (frameworks_zig.go) detects top-level test "name" { ... } / test ident { ... } / anonymous test { ... } blocks (zigTestRE, line-anchored so a test substring in an ident/string/comment never opens a block). Each block's balanced brace body (extractBraceBody) is scanned by the resolver: a direct production call (add(2,2), invisible-free via directCallRE) resolves to a high-confidence TESTS edge, and the identifier form test add { } seeds add as the naming-convention subject. The std.testing.* assertion DSL (expect/expectEqual/expectError/... bare + dotted forms) plus the bare try/expect tokens are denylisted in resolver.go so test-harness plumbing never surfaces as the SUT. FILENAME gated on the .zig extension (Zig tests live in ANY .zig file, often the production source itself) — NOT import gated (a bare test token would over-match the substring import matcher, e.g. C++ gtest/gtest.h; the elm-test precedent). The detector self-confirms — a .zig file with no test block yields zero cases and is dropped. Proven by TestZigTest_DirectCallHighConfidence / _IdentifierForm / _BodyScoped / _AssertionDSLNotSubject / _NonTestZigDropped. |
| Target extraction | ✅ `full` | `2026-06-24` | 5377 | `internal/extractors/cross/testmap/frameworks.go`<br>`internal/extractors/cross/testmap/frameworks_zig.go`<br>`internal/extractors/cross/testmap/frameworks_zig_test.go`<br>`internal/extractors/cross/testmap/resolver.go` | Each test block becomes one SCOPE.Pattern test_case entity: quoted descriptions normalise to a bare snake_case qname (nimTestCaseName — no framework prefix), the identifier form keeps the ident, and anonymous tests get an indexed anonymous_test_N name. Sibling-test bodies are isolated (the brace walk is balanced + quote-aware) so a call in one test never leaks into another. Honest partial slice: comptime/dynamically-generated tests are not modelled. Proven by TestZigTest_BodyScoped / _IdentifierForm. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update test.zig ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
