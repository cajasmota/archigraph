<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.verilog.core` — Verilog / SystemVerilog

Auto-generated. Back to [summary](../summary.md).

- **Language:** [verilog](../by-language/verilog.md)
- **Category:** [language](../by-category/language.md)
- **Capability cells:** 1

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Core extraction | 🟢 `partial` | `2026-06-24` | 5380 | `internal/extractors/verilog/extractor.go`<br>`internal/extractors/verilog/extractor_test.go` | Regex bootstrap (no tree-sitter grammar). Module/interface/package/class -> SCOPE.Component; function/task -> SCOPE.Operation; input/output/inout ports (ANSI header + classic body) -> SCOPE.Schema(subtype=port) with direction + width props, CONTAINS-wired from the owning module (#5380 buildPortEntities); module instantiations -> USES edges carrying instance_name + module_type + parameterized props so the instance topology graph is navigable (#5380 collectInstantiations); import/`include -> IMPORTS. Partial: no full type resolution / no signal-level dataflow / no generate-block elaboration (regex limits). Proven by TestVerilog_ANSIPorts / _ClassicPorts / _PortDedup / _InstanceTopology. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.verilog.core ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
