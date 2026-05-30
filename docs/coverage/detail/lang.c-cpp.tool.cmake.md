<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.tool.cmake` — CMake

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | 🟢 `partial` | `2026-05-30` | — | `internal/extractors/cross/manifest/extractor.go` | Regex: find_package() and target_link_libraries() → external dep entities + DEPENDS_ON edges |
| Target extraction | 🟢 `partial` | `2026-05-30` | — | `internal/extractors/cross/manifest/extractor.go` | Regex: target names from target_link_libraries() first arg |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.tool.cmake ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
