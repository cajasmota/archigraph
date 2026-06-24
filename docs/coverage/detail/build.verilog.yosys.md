<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `build.verilog.yosys` — Yosys

Auto-generated. Back to [summary](../summary.md).

- **Language:** [verilog](../by-language/verilog.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 1

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | 🟢 `partial` | `2026-06-24` | 5380 | `internal/extractors/verilog/extractor.go`<br>`internal/extractors/verilog/extractor_test.go` | Yosys detected from HDL attributes — (* top *) / (* blackbox *) / (* whitebox *) / (* abc9_box *) / (* nomem2reg *) ... (#5380 yosysAttrRE); emits SCOPE.Component(subtype=tool, tool=yosys). Partial: in-HDL attribute signal detection, NOT .ys script / synth_* command parsing. Proven by TestVerilog_YosysAttr. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update build.verilog.yosys ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
