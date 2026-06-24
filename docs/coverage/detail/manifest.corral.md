<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `manifest.corral` — corral (corral.json)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [pony](../by-language/pony.md)
- **Category:** [package_manager](../by-category/package_manager.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Lockfile parsing | — `not_applicable` | — | — | — | #5384: corral has no separate lockfile format — it folds resolved transitive deps into the same corral.json deps[] array, so the manifest IS the resolved set (parsed under manifest_parsing). N/A by ecosystem design. |
| Manifest parsing | ✅ `full` | `2026-06-24` | 5384 | `internal/extractors/cross/manifest/corral.go`<br>`internal/extractors/cross/manifest/corral_test.go`<br>`internal/extractors/cross/manifest/extractor.go` | #5384 (epic #5360): Pony corral.json (and legacy bundle.json) deps[] array parsed into external_dependency + DEPENDS_ON + SCOPE.Package records (package_manager=corral, parseCorralJSON). Dep name = locator's final path segment with .git stripped (github.com/ponylang/http_server.git -> http_server; corralDepName handles scp-style git@host:owner/name locators); version from the deps[].version field (or legacy deps[].tag). The legacy {type,repo} entry form is also accepted. corral folds resolved transitive deps into the same deps[] array, so the manifest doubles as the resolved set — there is no separate lockfile format. corral.json is wired by exact basename into IsManifest/detectPackageManager/dispatchParser/parsers. bundle.json is an ambiguous basename (collides with JS/webpack bundle artifacts), so it is corral-signal gated: it only anchors + emits deps when it parses to a non-empty corral deps[] array, else a complete no-op (TestBundleJSON_GenericNoOp). Proven by TestCorralJSON_Deps / _VersionCarried, TestBundleJSON_LegacyDeps, TestCorralJSON_NoDepsNoOp, TestCorral_IsManifest. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update manifest.corral ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
