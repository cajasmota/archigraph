<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `build.verilog.verilator` — Verilator

Auto-generated. Back to [summary](../summary.md).

- **Language:** [verilog](../by-language/verilog.md)
- **Category:** [build_system](../by-category/build_system.md)
- **Capability cells:** 1

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency graph | 🟢 `partial` | `2026-06-24` | 5380 | `internal/extractors/verilog/extractor.go`<br>`internal/extractors/verilog/extractor_test.go` | Verilator detected from HDL signals — /* verilator lint_off|lint_on|coverage_off|public|... */ pragmas and `verilator_config (#5380 toolSpecs/buildToolEntities); emits SCOPE.Component(subtype=tool) + file->tool USES edge. Partial: signal-detection from inside HDL source (the only files routed to this extractor), NOT project-file (Makefile/.vc/-f filelist) parsing; no build-target/dependency-graph extraction. Proven by TestVerilog_VerilatorPragma / _VerilatorConfig / _NoToolFalsePositive. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update build.verilog.verilator ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
