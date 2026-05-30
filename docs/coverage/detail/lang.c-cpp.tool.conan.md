<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.tool.conan` — Conan

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [package_manager](../by-category/package_manager.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Lockfile parsing | — `not_applicable` | `2026-05-30` | — | — | conan.lock is a separate file format not in standard manifest names; conanfile.txt/.py are manifest-only |
| Manifest parsing | 🟢 `partial` | `2026-05-30` | — | `internal/extractors/cross/manifest/extractor.go` | Regex: [requires]/[build_requires] sections in conanfile.txt; requires= in conanfile.py |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.tool.conan ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
