<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.haskell.tool.cabal` — Cabal / Stack / hpack (Haskell build)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [haskell](../by-language/haskell.md)
- **Category:** [package_manager](../by-category/package_manager.md)
- **Capability cells:** 1

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Manifest parsing | ✅ `full` | `2026-06-24` | 5373 | `internal/extractors/cross/manifest/cabal.go`<br>`internal/extractors/cross/manifest/cabal_test.go`<br>`internal/extractors/cross/manifest/extractor.go` | Three Haskell manifest shapes are parsed (#5373): *.cabal (parseCabal mines every build-depends: field across library/executable/test-suite/benchmark stanzas, comma+newline split, test-suite/benchmark deps flagged is_dev), package.yaml (parsePackageYaml reads the hpack dependencies: YAML list, tests:/benchmarks: blocks flagged is_dev) and stack.yaml (parseStackYaml emits extra-deps as pinned kind=locked: <name>-<version> versioned form + git github:/git: source pins). The GHC 'base' floor is kept as a real edge (mirrors nimble nim / luarocks lua). Suffix/exact-name dispatched in IsManifest/detectPackageManager/dispatchParser to package managers cabal/hpack/stack; emits DEPENDS_ON + DEPENDS_ON_PACKAGE + SBOM package nodes like every other ecosystem. Honest scope: cabal 'if flag(...)' conditionals are flattened; the Stack resolver snapshot set is not enumerated (remote package set, not a local manifest). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.haskell.tool.cabal ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
