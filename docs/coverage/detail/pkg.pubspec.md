<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `pkg.pubspec` — pubspec.yaml

Auto-generated. Back to [summary](../summary.md).

- **Language:** [dart](../by-language/dart.md)
- **Category:** [package_manager](../by-category/package_manager.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Lockfile parsing | ✅ `full` | `2026-06-24` | 5361 | `internal/extractors/cross/manifest/extractor_test.go` | pubspec.lock lockfile parsing (parsePubspecLock): the resolved dependency tree under packages: with exact versions, dev classification from the dependency: line (direct dev), and transitive deps the manifest never names (marked indirect=true); the trailing sdks: block is excluded. Emitted dependency_kind=locked. Proven by TestPubspecLock_Resolved (drift 2.14.1 direct main, build_runner direct dev, meta transitive/indirect, sdks dart excluded). |
| Manifest parsing | ✅ `full` | `2026-05-28` | — | `internal/extractors/cross/manifest/extractor.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update pkg.pubspec ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
