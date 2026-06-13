<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.framework.juce` — JUCE

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** Desktop
- **Capability cells:** 16

## Capabilities


### Process

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| IPC extraction | 🔴 `missing` | — | 4979 | — | Detection-only (#4979). Audio app/plugin framework; no extractor for JUCE InterprocessConnection / plugin-host messaging yet. |
| Main renderer split | — `not_applicable` | — | — | — | Audio/GUI framework; audio thread vs message thread is not a main-process/renderer-process split. NA. |

### Native

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Native module imports | 🔴 `missing` | — | 4979 | — | Detection-only (#4979). juce_add_plugin()/juce_add_gui_app() + juce_* modules are CMake markers; no native-module-import extraction. |

### Updates

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | ✅ `full` | — | — | `internal/graph/graph.go`<br>`internal/mcp/tools.go`<br>`internal/types/confidence.go` | — |
| Config consumption | 🔴 `missing` | — | 3641 | — | — |
| Constant propagation | ✅ `full` | — | — | `internal/links/constant_propagation.go`<br>`internal/substrate/c_cpp.go` | — |
| DB effect | 🟢 `partial` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_c_cpp.go` | — |
| Dead code detection | 🟢 `partial` | — | — | `internal/links/reachability.go`<br>`internal/mcp/dead_code.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_c_cpp.go` | — |
| Env fallback recognition | ✅ `full` | — | — | `internal/substrate/c_cpp.go` | — |
| Error flow | ✅ `full` | — | — | `internal/extractor/exception_flow.go`<br>`internal/extractors/cpp/exception_flow.go`<br>`internal/extractors/cpp/exception_flow_test.go` | — |
| Feature flag gating | 🔴 `missing` | — | feature_flag_gating:#3706-not-yet-extracted | — | — |
| Fs effect | 🟢 `partial` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_c_cpp.go` | — |
| HTTP effect | 🟢 `partial` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_c_cpp.go` | — |
| Import resolution quality | 🟢 `partial` | — | — | `internal/engine/cpp_gui_cv_game_detection_test.go`<br>`internal/engine/rules/cpp/frameworks/juce.yaml`<br>`internal/substrate/c_cpp.go` | Detection signature for the framework lives in juce.yaml (JuceHeader.h include, AudioProcessor/JUCEApplication markers, juce_add_plugin() CMake DSL); generic c-cpp include resolution applies. Fixtures: TestCppGuiCvGameDetection (happy/wrong-language/no-match). |
| Mutation effect | 🟢 `partial` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_c_cpp.go` | — |
| Reachability analysis | 🟢 `partial` | — | — | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_c_cpp.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.framework.juce ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
