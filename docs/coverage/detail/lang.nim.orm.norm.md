<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.nim.orm.norm` — Norm (Nim ORM)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [nim](../by-language/nim.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-06-12` | 4904 | `internal/custom/nim/extractors_test.go`<br>`internal/custom/nim/norm_orm.go` | Norm is the de-facto Nim ORM; a persisted model is a `T* = ref object of Model` declaration. nimNormModelRe recognises each such type and emits one SCOPE.Schema/model + one SCOPE.Schema/table (table identity = the model type name) per model, carrying framework=norm + provenance. Pre-filtered by nimNormHasModel so plain Nim objects are ignored. Proven by TestNimNormORM_ModelTableColumns + TestNimNormORM_NonModelNoop. |
| Model lifecycle extraction | — `not_applicable` | — | 4991 | — | Norm has no model lifecycle/callback DSL — unlike ActiveRecord/Granite (before_save/after_create) or TypeORM (@BeforeInsert), Norm exposes no before/after model hooks. Persistence side-effects are expressed as ordinary imperative code around `db.insert/update/delete` calls (see query_attribution + transaction_function_stamping), not as declarative hooks on the model, so there is no lifecycle annotation to extract. Confirmed against Norm's model API (norm/model) during #4991. |
| Schema extraction | ✅ `full` | `2026-06-12` | 4932 | `internal/custom/nim/extractors_test.go`<br>`internal/custom/nim/norm_orm.go` | Each public object field of a model becomes a SCOPE.Schema/column carrying column_type (Option[T]/seq[T] generic wrappers unwrapped to the inner type) and the owning model name. #4932 deepened: a model-header `{.tableName: "x".}` / `{.dbName: "x".}` pragma keys the table entity by the override name and stamps table_name on the model (table identity is no longer forced to the Nim type name); field-level pragmas are read — `{.unique.}` -> unique=true, `{.dbType: "TEXT".}` -> db_type=TEXT on the column. Proven by TestNimNormORM_Deepen_4932 (tableName override + unique + dbType asserted) + TestNimNormORM_ModelTableColumns. Honest remainder: index pragmas beyond unique are not modelled (follow-up #4932). |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | Norm models relations as plain typed object fields, not a declarative association DSL — there is no association macro to extract. |
| Foreign key extraction | 🟢 `partial` | `2026-06-14` | 4904 | `internal/custom/nim/extractors_test.go`<br>`internal/custom/nim/norm_orm.go` | A field typed as another model declared in the same file yields a REFERENCES edge model->referenced-model (fk_field + to_model props) and stamps foreign_key=true / column_type on the column; Option[Model]/seq[Model] wrappers are unwrapped first. #4932 deepened: an explicit `{.fk: Other.}` pragma on a scalar-typed field (e.g. `authorId* {.fk: User.}: int64`) now yields a REFERENCES edge (fk_pragma=true) + a foreign_key=true/fk_target column even though the field type is not itself a Model. Proven by TestNimNormORM_ModelTableColumns + TestNimNormORM_OptionWrappedFK + TestNimNormORM_Deepen_4932. Partial (honest): cross-file FK targets emit a REFERENCES edge to the bare type name but are not resolved to the concrete entity here. Cross-file resolution is intentionally delegated to the shared cross-file resolver (binding a REFERENCES edge whose target is a bare type name to the concrete model entity declared in another file is NOT file-local and cannot be done honestly inside the per-file extractor); it remains a shared-resolver follow-up, not a Norm-extractor gap (#4991). |
| Lazy loading recognition | — `not_applicable` | — | — | — | Norm loads related rows via explicit `db.select` calls, not a lazy-loading proxy layer — no lazy-load annotation to recognise. |
| Relationship extraction | 🟢 `partial` | — | 4932 | `internal/custom/nim/norm_orm.go` | Field-typed-as-model relationships surface as REFERENCES edges (see foreign_key_extraction). Norm has no separate declarative association DSL (relationships are plain typed fields), so association_extraction/lazy_loading are not_applicable; full bidirectional relationship modelling is follow-up #4932. |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-06-14` | 4991 | `internal/custom/nim/extractors_test.go`<br>`internal/custom/nim/norm_orm.go` | A `db.select/insert/update/delete(...)` call site emits a QUERIES edge from the model entity to its table (operation/table/model props), one edge per distinct operation. #4991 generalised first-argument resolution to THREE forms: (1) a model TYPE — `db.select(User, ...)`; (2) a variable handle bound to a model — `var u = User()` ... `db.select(u, ...)` resolved via nimNormHandleBindModelRe; (3) a raw-SQL query naming a table — `db.select(objs, sql"... FROM posts ...")` / `db.rawSelect(...)` whose FROM/INTO/UPDATE table is matched back to its model via tableToModel (nimNormRawSelectRe). collectNormQueries + nimNormQueryRe. Proven by TestNimNormORM_Deepen_4932 (model-typed) + TestNimNormQuery_VariableHandleAndRawSQL_4991 (handle: User select+update; raw-SQL: Post select via FROM posts). |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | ✅ `full` | `2026-06-14` | 4991 | `internal/custom/nim/extractors_test.go`<br>`internal/custom/nim/norm_migrations.go` | Norm has no declarative migration DSL; schema is created/evolved imperatively against a DbConn handle. norm_migrations.go (custom_nim_norm_migrations) parses the two real shapes: model-typed schema ops `<db>.createTables(Model())` / `<db>.dropTables(Model())` (and the receiver-less `createTables(Model())` form), plus a variable handle `var u = User()` ... `createTables(u)` resolved via nimNormHandleBindRe; and raw-DDL `db.exec(sql"CREATE/DROP/ALTER TABLE <name> ...")` strings (nimNormRawDDLRe). Pre-filtered by nimNormHasMigration. Proven by TestNimNormMigrations_4991 + TestNimNormMigrations_NoMatchNoop + TestNimNormMigrations_WrongLanguageNoop. Honest: a truly dynamic (unbound) createTables handle or an interpolated raw-DDL table name is skipped (no fabricated op). |
| Migration schema ops | ✅ `full` | `2026-06-14` | 4991 | `internal/custom/nim/extractors_test.go`<br>`internal/custom/nim/norm_migrations.go`<br>`internal/engine/migration_schema_ops.go`<br>`internal/engine/migration_schema_ops_test.go` | Each Norm migration op is emitted as a shared SCOPE.Evolution entity (framework=norm, migration_op, table, provenance INFERRED_FROM_NORM_MIGRATION) with the normalised op subtype (create_table|drop_table|alter_table) — the same Kind the JS knex/typeorm and Nim Allographer migration extractors use. The engine migration-schema-ops pass (internal/engine/migration_schema_ops.go, `case "norm"` in evolutionOp) derives a MODIFIES_TABLE edge op→table convergence node, unifying migration→table evolution with query→table access on one logical table (model-typed ops target the model NAME, bound to its table node by the shared resolver/normTable). Proven by TestNormCreateDropMigration (engine: model-typed create + raw-DDL alter both converge to the `user` table node). |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | ✅ `full` | `2026-06-14` | 4991 | `internal/custom/nim/extractors_test.go`<br>`internal/custom/nim/norm_orm.go` | A Norm `db.transaction:` / `<conn>.transaction:` block header emits a SCOPE.Pattern/transaction_boundary entity (transactional=true, framework=norm, db_handle, provenance INFERRED_FROM_NORM_TRANSACTION), mirroring the Kotlin/Java @Transactional boundary shape. #4991 deepened: the boundary is now stamped with its enclosing_proc (resolved by an indent-tracking proc map over proc/func/method headers, buildNormProcMap) and with the write ops issued inside the block (writes=insert,update,delete in stable order + has_writes=true, via normTxWrites scanning the indented body). collectNormTransactions + nimNormTxRe + nimNormProcRe. Proven by TestNimNormORM_Deepen_4932 + TestNimNormTransaction_EnclosingProcAndWrites_4991 (enclosing_proc=savePost, writes=insert,update). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.nim.orm.norm ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
