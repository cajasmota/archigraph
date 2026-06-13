<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `build.erlang-mk` — erlang.mk (Makefile)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [erlang](../by-language/erlang.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | ✅ `full` | `2026-06-14` | 4987 | `internal/extractors/cross/manifest/erlang.go`<br>`internal/extractors/cross/manifest/erlang_test.go` | #4930/#4987: erlang.mk DEPS / TEST_DEPS / BUILD_DEPS / LOCAL_DEPS / DOC_DEPS / SHELL_DEPS assignment lines parsed into external_dependency records (package_manager=erlang_mk); TEST_/DOC_/SHELL_ variants flagged is_dev=true. A Makefile is only treated as an erlang.mk build when it includes erlang.mk or declares PROJECT (isErlangMk signal) — a plain non-Erlang Makefile is a no-op (TestErlangMk_PlainMakefileNoOp). Make-variable refs ($(...)) skipped. Proven by TestErlangMk_Deps. |
| Target extraction | 🔴 `missing` | — | 4987 | — | #4987: erlang.mk dep_<name> = git url ref provenance lines and custom make targets are not yet modelled as build targets; only the DEPS dependency graph is extracted. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update build.erlang-mk ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
