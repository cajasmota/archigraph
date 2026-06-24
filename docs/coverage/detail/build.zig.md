<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `build.zig` — zig build (build.zig)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [zig](../by-language/zig.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | ✅ `full` | `2026-06-24` | 5377 | `internal/extractors/cross/manifest/buildzig.go`<br>`internal/extractors/cross/manifest/buildzig_test.go`<br>`internal/extractors/cross/manifest/extractor.go` | build.zig b.dependency("name", ...) calls parsed into external_dependency + DEPENDS_ON + SCOPE.Package records under package_manager=zig (buildZigDependencyRE; receiver-agnostic so non-canonical builder names match). zig build is both build orchestrator and dependency fetcher — there is no separate package-manager binary. Wired into IsManifest/detectPackageManager/dispatchParser/parsers by exact basename build.zig. Proven by TestBuildZig_DependencyGraph / _DependsOnEdges. |
| Target extraction | ✅ `full` | `2026-06-24` | 5377 | `internal/extractors/cross/manifest/buildzig.go`<br>`internal/extractors/cross/manifest/buildzig_test.go`<br>`internal/extractors/cross/manifest/extractor.go` | build.zig declared build targets (addExecutable/addStaticLibrary/addSharedLibrary/addObject/addModule/addTest/addLibrary) are mined by extractBuildZigTargets (name from the .name="..." options field or the leading positional string for addModule; quote-aware matchingParen arg-slice walk) and surfaced as a comma-joined build_targets property on the manifest project anchor — queryable without a new entity kind, mirroring the cmake target treatment. build.zig.zon does NOT carry build_targets. Honest partial slice: comptime-generated targets are not modelled. Proven by TestBuildZig_TargetExtraction / TestBuildZigZon_NoBuildTargets. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update build.zig ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
