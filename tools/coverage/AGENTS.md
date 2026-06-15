# Coverage tooling — agent guide

The `coverage` command maintains the grafel capabilities registry at `docs/coverage/registry.json` and regenerates the per-language / per-category markdown views. It is the source of truth for "what does grafel extract today?"

## Hard rules

- **Standalone dev tool.** No imports from `internal/*` are allowed — pure file I/O + YAML/JSON. This keeps the tool buildable independent of the indexer and lets it run from any worktree without a daemon.
- **Determinism.** Every `gen` invocation must produce byte-identical output for the same input. The pre-commit gen workflow + CI (`.github/workflows/coverage-docs.yml`) compare regenerated docs against the committed copy.
- **Schema is data-driven.** The capability taxonomy lives in `capability-dictionary.yaml` (post-#2752). Do not hardcode capability keys in Go; load them from the dictionary.

## Files

- `main.go` — CLI dispatcher; subcommands: `list`, `get`, `add`, `update`, `gaps`, `stats`, `validate`, `gen`, `discover`, `map-status`, `parity`
- `schema.go` — registry + record shape
- `parity.go` — READ-ONLY coverage parity probe (#3876): flags flagship→sibling asymmetry (a capability credited on one framework but missing on same-language siblings in the same `(language, category, subcategory)` group). Uniform-scaffold (all-missing) cells are suppressed by design. `--strict` is a CI gate.
- `store.go` — load / save / canonical ordering of `registry.json`
- `validate.go` — schema invariants (referential integrity, status enum, dictionary key conformance)
- `capability_map.go` + `capability-map.yaml` — capability → file/function mapping for traceability
- `validate_map.go` — verifies `capability-map.yaml` references real files
- `generate.go` + `templates/` — markdown rendering of `docs/coverage/{summary.md,by-language/,by-category/,detail/}`
- `discover.go` / `map_status.go` — bootstrap helpers
- `buckets.go` / `languages.go` / `views.go` — projection helpers used by templates

## Extending the schema

Because the taxonomy is data-driven:

1. Open `capability-dictionary.yaml` and add the new capability key under the right group, with description + status enum if non-default.
2. Run `go run ./tools/coverage validate` — it will fail on any record that doesn't yet have a value for the new key (defaulting to `missing` is fine, but it must be explicit if the dictionary marks it required).
3. Update existing records in `docs/coverage/registry.json` via `go run ./tools/coverage update ...` rather than editing JSON by hand when possible — the tool guarantees canonical placement.
4. Regenerate: `go run ./tools/coverage gen`.
5. Commit the dictionary + registry + regenerated docs together. Splitting them across PRs breaks the CI gate.

## Templates

- Templates live in `templates/` and use Go's `text/template`.
- Keep them deterministic: sort every map / slice before iterating. The `gen_test.go` snapshot test will catch nondeterministic ordering.

## Errors vs warnings — the validate gate policy

`go run ./tools/coverage validate` distinguishes two severities, and the CI
gate (`.github/workflows/coverage-docs.yml`) treats them differently:

- **Errors fail the build** (`main.go` returns non-zero when `totalErrors > 0`).
  These are hard consistency violations: a capability-map citation pointing at a
  registry cell that doesn't exist (or whose shape/group doesn't match), a cited
  source file or function missing on disk, an invalid status enum, a stale
  dictionary key, a duplicate capability key. **Errors must stay at 0.**
- **Warnings never fail the build.** They are advisory nudges and are expected
  to number in the thousands at the group's current breadth. Do not "fix" them
  by deleting registry rows or capability-map content — that would silence
  reality rather than reflect it. The two dominant warning classes are
  structural and intentional:
  1. *"capability has no mapping entry"* (`validate_map.go`, the
     mapping-coverage nudge) — every registry cell with status `full`/`partial`
     ideally has a `capability-map.yaml` symbol/function mapping. The mapping
     section only enumerates grafel's own code-delivering records (~18); the
     hundreds of framework/language records (e.g. `lang.jsts.framework.*`,
     `test.pytest`) are descriptive coverage entries served by *shared*
     extractors, so per-record symbol mappings are deliberately absent. Adding
     mapping is incremental, never required.
  2. *"capability has no tracking issue"* (`validate.go`,
     `validateCapabilityCell`) — `partial`/`missing` cells without an `issue:`
     link get a nudge to file a ticket. These mark known framework-support gaps
     and are advisory, not blocking.

  If a warning is ever genuinely wrong (cell status misrepresents what the code
  does), fix it at the source — flip the registry cell or correct the citation —
  rather than suppressing the warning. (Ref: #2799.)

## Coverage matrix update rule

The root `AGENTS.md` "Coverage matrix update" section is **the** rule for capability-changing PRs across the repo. Tooling changes inside this directory generally do NOT require a matrix update unless they alter the schema (in which case the schema PR + record migration must ship together).
