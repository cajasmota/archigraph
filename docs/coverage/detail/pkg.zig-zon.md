<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `pkg.zig-zon` — build.zig.zon (Zig package manifest)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [zig](../by-language/zig.md)
- **Category:** [package_manager](../by-category/package_manager.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Lockfile parsing | — `not_applicable` | `2026-06-24` | 5377 | `internal/classifier/classifier.go`<br>`internal/extractors/cross/manifest/buildzig.go`<br>`internal/extractors/cross/manifest/buildzig_test.go`<br>`internal/extractors/cross/manifest/extractor.go` | build.zig.zon IS the lockfile — every dependency is content-addressed by a SHA-256 .hash (the exact, immutable pin), so there is no separate lockfile format. The pinned version is recovered directly from manifest_parsing (the .hash value). |
| Manifest parsing | ✅ `full` | `2026-06-24` | 5377 | `internal/classifier/classifier.go`<br>`internal/extractors/cross/manifest/buildzig.go`<br>`internal/extractors/cross/manifest/buildzig_test.go`<br>`internal/extractors/cross/manifest/extractor.go` | build.zig.zon is the Zig Object Notation package manifest (since Zig 0.11). parseBuildZigZon locates the top-level .dependencies = .{ ... } anonymous-struct literal (zonDependenciesHeadRE + a quote-aware matchingBrace hand-walk since regex cannot balance nested .{ }), then mines each .name = .{ .url=..., .hash=... } entry (zonDepEntryRE; bare .ident and .@"quoted ident" field shapes). The version is the content .hash (preferred — the exact pin) falling back to the .url archive. package_manager=zig; emits DEPENDS_ON + DEPENDS_ON_PACKAGE + SBOM package nodes like every other ecosystem. Wired into IsManifest/detectPackageManager/dispatchParser/parsers by exact basename build.zig.zon. Proven by TestBuildZigZon_Dependencies / TestBuildZig_DependsOnEdges / TestBuildZig_IsManifest. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update pkg.zig-zon ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
