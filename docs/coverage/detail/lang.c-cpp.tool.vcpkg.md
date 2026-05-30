<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.tool.vcpkg` — vcpkg

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [package_manager](../by-category/package_manager.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Lockfile parsing | 🟢 `partial` | `2026-05-30` | — | `internal/extractors/cross/manifest/extractor.go` | vcpkg.json also serves as a pinned manifest (version-gte semantics) |
| Manifest parsing | 🟢 `partial` | `2026-05-30` | — | `internal/extractors/cross/manifest/extractor.go` | JSON: vcpkg.json dependencies[] — string and object {name,version-gte} forms |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.tool.vcpkg ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
