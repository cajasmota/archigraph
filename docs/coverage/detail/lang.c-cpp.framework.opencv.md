<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.framework.opencv` — OpenCV

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** Desktop
- **Capability cells:** 16

## Capabilities


### Process

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| IPC extraction | — `not_applicable` | — | — | — | Computer-vision / ML library (in-process algorithms); no IPC/messaging surface to extract. |
| Main renderer split | — `not_applicable` | — | — | — | CV library, not a multi-process desktop app; no main/renderer split. NA. |

### Native

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Native module imports | 🔴 `missing` | — | 4979 | — | Detection-only (#4979). find_package(OpenCV) + opencv_* link targets are CMake markers; no native-module-import extraction. |

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
| Import resolution quality | 🟢 `partial` | — | — | `internal/engine/cpp_gui_cv_game_detection_test.go`<br>`internal/engine/rules/cpp/frameworks/opencv.yaml`<br>`internal/substrate/c_cpp.go` | Detection signature for the framework lives in opencv.yaml (opencv2/ includes, cv:: namespace markers, find_package(OpenCV) CMake); generic c-cpp include resolution applies. Fixtures: TestCppGuiCvGameDetection (happy/wrong-language/no-match). |
| Mutation effect | 🟢 `partial` | — | — | `internal/links/effect_propagation.go`<br>`internal/substrate/effect_sinks_c_cpp.go` | — |
| Reachability analysis | 🟢 `partial` | — | — | `internal/links/reachability.go`<br>`internal/substrate/entry_points.go`<br>`internal/substrate/entry_points_c_cpp.go` | — |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.framework.opencv ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
