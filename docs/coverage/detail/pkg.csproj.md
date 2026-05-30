<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `pkg.csproj` — .csproj / packages.config

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C#](../by-language/csharp.md)
- **Category:** [package_manager](../by-category/package_manager.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Lockfile parsing | ✅ `full` | `2026-05-30` | 3263 | `internal/extractors/cross/manifest/extractor.go` | packages.lock.json NuGet v3 lock format parsed via JSON unmarshalling; covers direct and transitive deps with resolved versions |
| Manifest parsing | ✅ `full` | `2026-05-30` | 3263 | `internal/extractors/cross/manifest/extractor.go` | .csproj <PackageReference Include= Version=> elements parsed via regex; full coverage of NuGet PackageReference format |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update pkg.csproj ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
