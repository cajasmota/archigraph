<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `pkg.rebar3` — rebar3 / hex.pm (rebar.config, rebar.lock)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [erlang](../by-language/erlang.md)
- **Category:** [package_manager](../by-category/package_manager.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Lockfile parsing | ✅ `full` | `2026-06-14` | 4987 | `internal/extractors/cross/manifest/erlang.go`<br>`internal/extractors/cross/manifest/erlang_test.go` | #4987: rebar.lock parsed (rebarLockEntryRE) — each {<<"name">>,{pkg,<<"name">>,<<"version">>},N} locked entry emits a dependency_kind=locked dep with the resolved hex version. Proven by TestRebarLock_Locked. |
| Manifest parsing | ✅ `full` | `2026-06-14` | 4987 | `internal/extractors/cross/manifest/erlang.go`<br>`internal/extractors/cross/manifest/erlang_test.go` | #4930/#4987: rebar.config {deps,[...]}/{plugins,[...]} and *.app.src {applications,[...]} parsed into DEPENDS_ON external dependency records resolved from hex.pm. IsManifest/detectPackageManager/dispatchParser recognise rebar.config, rebar.lock, *.app.src, erlang.mk and erlang.mk-style Makefiles. Proven by TestRebarConfig_Deps, TestAppSrc_Applications, TestErlang_NotAManifest. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update pkg.rebar3 ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
