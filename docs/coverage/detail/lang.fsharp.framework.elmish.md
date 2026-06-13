<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.fsharp.framework.elmish` — Fable Elmish/Feliz (F# frontend)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [F#](../by-language/fsharp.md)
- **Category:** [http_framework](../by-category/http_framework.md)
- **Subcategory:** UI Frontend
- **Capability cells:** 42

## Capabilities


### Structure

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Component extraction | ✅ `full` | `2026-06-13` | [link](https://github.com/cajasmota/archigraph/issues/5129) | `internal/extractors/fsharp/elmish_feliz.go`<br>`internal/extractors/fsharp/elmish_feliz_test.go`<br>`internal/extractors/fsharp/testdata/elmish_counter.fs` | Feliz/Fable.React component functions (let-bound bodies using the Html./prop./React. DSL or a ReactElement return) re-kinded SCOPE.UIComponent (feliz_component); nested component calls emit RENDERS edges. The MVU view is itself re-kinded. Import-gated on open Elmish/Feliz/Fable. |
| Context extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Data Flow

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Branch conditions | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Data fetching | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Prop extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| State management | ✅ `full` | `2026-06-13` | [link](https://github.com/cajasmota/archigraph/issues/5129) | `internal/extractors/fsharp/elmish_feliz.go`<br>`internal/extractors/fsharp/elmish_feliz_test.go`<br>`internal/extractors/fsharp/testdata/elmish_counter.fs` | Elmish MVU: the Model record re-kinded SCOPE.Model (elmish_model), the Msg DU re-kinded SCOPE.Event (elmish_msg) with its cases as messages, and init/update/view tagged Properties[elmish_role]. Program.mkProgram/mkSimple bootstrap flagged (elmish_program). Cmd dispatch (Cmd.ofMsg/OfAsync/OfPromise/batch/none) emits USES edges stamped Properties[dispatch]. |

### Navigation

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Router pattern | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Type System

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Enum extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Interface extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Type alias extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Lifecycle

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| State setter emission | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Testing

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Tests linkage | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

### Substrate

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Confidence overlay | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Config consumption | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Constant propagation | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| DB effect | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Dead code detection | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Def use chain extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Env fallback recognition | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Error flow | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Feature flag gating | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Fs effect | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| HTTP effect | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Import resolution quality | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Module cycle detection | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Mutation effect | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Pure function tagging | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Reachability analysis | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Request shape extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Response shape extraction | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Sanitizer recognition | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Schema drift detection | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Taint sink detection | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Taint source detection | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Template pattern catalog | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |
| Vulnerability finding | 🔴 `missing` | — | backfill:dictionary-completeness | — | — |

## Framework-specific

### Fable Elmish/Feliz

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Command dispatch extraction | ✅ `full` | `2026-06-14` | [link](https://github.com/cajasmota/archigraph/issues/5129) | `internal/extractors/fsharp/elmish_feliz.go`<br>`internal/extractors/fsharp/elmish_feliz_test.go`<br>`internal/extractors/fsharp/testdata/elmish_counter.fs` | Cmd.ofMsg / Cmd.OfAsync.* / Cmd.OfPromise.* / Cmd.OfFunc.* / Cmd.batch / Cmd.none / Cmd.map dispatch helpers inside update/init emit USES edges stamped Properties[dispatch]=<helper>, elmish_command=true. |
| Elmish subscription extraction | 🔴 `missing` | — | [link](https://github.com/cajasmota/archigraph/issues/5129) | — | Elmish subscriptions (Program.withSubscription / Sub.batch) and message-source wiring are not yet modelled. |
| Feliz component extraction | ✅ `full` | `2026-06-14` | [link](https://github.com/cajasmota/archigraph/issues/5129) | `internal/extractors/fsharp/elmish_feliz.go`<br>`internal/extractors/fsharp/elmish_feliz_test.go`<br>`internal/extractors/fsharp/testdata/elmish_counter.fs` | Feliz/Fable.React DSL bodies (Html./prop./React./ReactElement) re-kinded SCOPE.UIComponent (feliz_component) with RENDERS edges to nested child components, mirroring the React #610 composition convention. |
| Feliz prop extraction | 🔴 `missing` | — | [link](https://github.com/cajasmota/archigraph/issues/5129) | — | Per-prop extraction of Feliz attribute lists (prop.text/prop.onClick/style props) into structured HAS_PROPS cells is deferred; only component-level recognition + RENDERS composition is implemented. |
| Mvu triad extraction | ✅ `full` | `2026-06-14` | [link](https://github.com/cajasmota/archigraph/issues/5129) | `internal/extractors/fsharp/elmish_feliz.go`<br>`internal/extractors/fsharp/elmish_feliz_test.go`<br>`internal/extractors/fsharp/testdata/elmish_counter.fs` | init/update/view operations tagged Properties[elmish_role]=init|update|view (name-convention recognition incl. init*/update*/view*); Model record re-kinded SCOPE.Model and Msg DU re-kinded SCOPE.Event so the MVU data triad is queryable. |
| Program bootstrap detection | ✅ `full` | `2026-06-14` | [link](https://github.com/cajasmota/archigraph/issues/5129) | `internal/extractors/fsharp/elmish_feliz.go`<br>`internal/extractors/fsharp/elmish_feliz_test.go`<br>`internal/extractors/fsharp/testdata/elmish_counter.fs` | Program.mkProgram / Program.mkSimple recognised; the host operation flagged Properties[elmish_program]=true. Program.withReactSynchronous/hydration wiring beyond the bootstrap flag is deferred. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.fsharp.framework.elmish ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
