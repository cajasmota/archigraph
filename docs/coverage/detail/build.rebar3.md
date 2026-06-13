<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `build.rebar3` — rebar3 (rebar.config)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [erlang](../by-language/erlang.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | ✅ `full` | `2026-06-14` | 4987 | `internal/extractors/cross/manifest/erlang.go`<br>`internal/extractors/cross/manifest/erlang_test.go` | #4930/#4987: rebar.config {deps,[...]} parsed into external_dependency + DEPENDS_ON + SCOPE.Package records via the cross-manifest extractor. Bare-atom deps (cowboy), {name,"x.y.z"} versioned deps (jsx) and {name,{pkg|git,...}} source-spec deps (ranch/mylib) are recovered; source-spec keywords (pkg/git/branch/tag/ref) are NOT leaked as deps; {plugins,[...]} entries are emitted as dev deps. rebar.lock {<<"name">>,{pkg,...,<<"ver">>},N} entries are parsed as dependency_kind=locked. package_manager=rebar3 (hex.pm). Proven by TestRebarConfig_Deps, TestRebarConfig_VersionCarried, TestRebarLock_Locked. |
| Target extraction | 🟢 `partial` | — | 4987 | `internal/extractors/cross/manifest/erlang.go`<br>`internal/extractors/cross/manifest/erlang_test.go` | #4987: *.app.src {applications,[...]} runtime-application deps are recovered (OTP/stdlib apps kernel/stdlib/... filtered out) — TestAppSrc_Applications. Profiles ({profiles,[...]}) and per-profile target/release definitions are not yet modelled as distinct build targets (honest partial). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update build.rebar3 ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
