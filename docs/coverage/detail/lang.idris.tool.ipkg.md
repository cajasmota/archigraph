<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.idris.tool.ipkg` — ipkg (Idris package manifest)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [idris](../by-language/idris.md)
- **Category:** [package_manager](../by-category/package_manager.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Lockfile parsing | — `not_applicable` | `2026-06-24` | 5382 | `internal/extractors/cross/manifest/ipkg.go` | ipkg has no lockfile format — the resolved dependency set is pinned by the package collection / pack (pack.toml), which is not a per-package manifest. depends names are bare (no version-constraint syntax), so there is nothing to lock at the manifest level. |
| Manifest parsing | ✅ `full` | `2026-06-24` | 5382 | `internal/extractors/cross/manifest/extractor.go`<br>`internal/extractors/cross/manifest/ipkg.go`<br>`internal/extractors/cross/manifest/ipkg_test.go` | parseIpkg mines the *.ipkg `depends` field (and the `pkgs` synonym) — a comma/newline-separated list of bare package names (the canonical multi-line layout starts continuation lines with a leading comma; -- line comments stripped). Idris's ipkg depends has NO version-constraint syntax, so every dep version is empty (honest). The Idris stdlib floor (base/prelude/contrib/network/…) is KEPT as a real edge, mirroring the nimble nim / luarocks lua / cabal base interpreter-floor treatment (#5365/#5367/#5373). Manifest metadata (package header, modules list, main, executable, sourcedir, version) is surfaced on the project anchor as a compact deterministic `ipkg_config` property — no new entity kind, the same model as the Zig build_targets / ReScript rescript_config props (#5377/#5378). Suffix-dispatched in IsManifest/detectPackageManager/dispatchParser to package_manager=idris2; emits DEPENDS_ON + DEPENDS_ON_PACKAGE + SBOM package nodes like every other ecosystem. Proven by TestIpkg_Dependencies / _DependsOnEdges / _ConfigAnchor / _PkgsSynonym / _IsManifest / _NoDependencies. Honest scope: only the declared manifest dependency/metadata surface is recovered. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.idris.tool.ipkg ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
