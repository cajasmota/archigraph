<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `build.verilog.vivado` — Vivado

Auto-generated. Back to [summary](../summary.md).

- **Language:** [verilog](../by-language/verilog.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 1

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | 🟢 `partial` | `2026-06-24` | 5380 | `internal/extractors/verilog/extractor.go`<br>`internal/extractors/verilog/extractor_test.go` | Vivado synthesis attributes detected from HDL signals — (* synthesis *) / (* keep *) / (* dont_touch *) / (* mark_debug *) / (* ram_style *) / (* async_reg *) ... (#5380 synthAttrRE); emits SCOPE.Component(subtype=tool, tool=synthesis). Partial: in-HDL synthesis-attribute signal detection (shared Vivado/Quartus synthesis flow), NOT .xpr/.tcl project parsing. Proven by TestVerilog_SynthesisAttrs. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update build.verilog.vivado ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
