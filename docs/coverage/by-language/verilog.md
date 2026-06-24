<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# verilog

**Frameworks**: 0 · **Tools**: 4 · **ORMs**: 0 · **Other**: 1

Back to [summary](../summary.md).

### Legend

Each group column shows `glyph covered/applicable` — **covered** = capabilities with extraction, **applicable** = covered + missing (not-applicable capabilities are excluded from both). The glyph is the group's **support level**:

| Glyph | Level | Meaning |
|---|---|---|
| ✅ | **Comprehensive** | every applicable capability is `full` — fixture-proven, resolves the general case |
| 🟢 | **Supported** | every applicable capability is extracted; some only *heuristically* (detected by pattern, not full AST/data-flow resolution) |
| 🟡 | **Partial** | some capabilities extracted, some still missing |
| 🔴 | **Not extracted** | nothing extracted yet |
| — | **N/A** | capability does not apply to this framework |

Examples: `🟢 20/20` = fully supported, some capabilities heuristic · `🟡 12/20` = 8 not yet extracted. Detail pages use the same palette **per cell** (✅ full · 🟢 heuristic/partial · 🔴 missing · — n/a).

## Tools

| Name | Dependency graph | Dependency usage status | Lockfile parsing | Manifest parsing | Target extraction | Notes |
|---|---|---|---|---|---|---|
| [Quartus](../detail/build.verilog.quartus.md) | 🟢 | — | — | — | — | |
| [Verilator](../detail/build.verilog.verilator.md) | 🟢 | — | — | — | — | |
| [Vivado](../detail/build.verilog.vivado.md) | 🟢 | — | — | — | — | |
| [Yosys](../detail/build.verilog.yosys.md) | 🟢 | — | — | — | — | |

## Other

| Name | Category | Status | Notes |
|---|---|---|---|
| [Verilog / SystemVerilog](../detail/lang.verilog.core.md) | [language](../by-category/language.md) | 🟢 | |
