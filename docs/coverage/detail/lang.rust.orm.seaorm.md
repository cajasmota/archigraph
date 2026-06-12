<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.rust.orm.seaorm` — SeaORM

Auto-generated. Back to [summary](../summary.md).

- **Language:** [rust](../by-language/rust.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/rules/rust/orms/seaorm.yaml` | — |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/rust/orm_props_test.go`<br>`internal/custom/rust/seaorm.go`<br>`internal/custom/rust/testdata/seaorm_entity.rs` | — |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/rust/seaorm.go`<br>`internal/custom/rust/testdata/seaorm_entity.rs` | — |
| Foreign key extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/rust/diesel_seaorm_test.go`<br>`internal/custom/rust/seaorm.go` | Detects #[sea_orm(belongs_to=..., from=..., to=...)] with explicit FK column references |
| Lazy loading recognition | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/rust/diesel_seaorm_test.go`<br>`internal/custom/rust/seaorm.go` | Detects .find_related(), .find_linked(), LoaderTrait::load_many/load_one patterns |
| Relationship extraction | 🟢 `partial` | `2026-05-30` | backfill:dictionary-completeness | `internal/custom/rust/seaorm.go`<br>`internal/custom/rust/testdata/seaorm_entity.rs` | DeriveRelation variants + impl Related resolved to target Entity path string; linking 'super::user::Entity' to its concrete Model entity needs cross-module resolution. |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-05-28` | — | `internal/engine/rules/rust/orms/seaorm.yaml` | — |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | ✅ `full` | `2026-06-12` | 5022 | `internal/custom/rust/diesel_seaorm_test.go`<br>`internal/custom/rust/seaorm.go`<br>`internal/custom/rust/testdata/seaorm_migration.rs` | #5022: impl MigrationTrait for M emits a migration component; the human migration id is resolved from the adjacent MigrationName::name impl (migration_id prop). Proven by TestSeaORM_Migration + TestSeaORM_MigrationSchemaOps_FromFixture. |
| Migration schema ops | 🟢 `partial` | `2026-06-12` | 5022 | `internal/custom/rust/diesel_seaorm_test.go`<br>`internal/custom/rust/seaorm.go`<br>`internal/custom/rust/testdata/seaorm_migration.rs` | #5022: up()/down() schema-builder calls manager.{create,alter,drop,truncate,rename}_table and {create,drop}_index are parsed via balanced-paren spans into migration components carrying migration_op + the resolved .table(<Iden>::Table) target (or index .name); ColumnDef::new(<Iden>::<Col>) inside create/alter ops emits a schema_column per column with table_name+migration_op. Column SQL types (.integer()/.string() builder chain), default/not-null modifiers, and cross-file Iden->table-name resolution are deferred. Proven by TestSeaORM_MigrationSchemaOps_FromFixture/_AlterTable/_CreateIndex. |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | ✅ `full` | `2026-06-12` | 5021 | `internal/custom/rust/transactions.go`<br>`internal/custom/rust/transactions_test.go` | #5021: sea_orm `db.begin()` and `db.transaction(|txn| ...)` (import-disambiguated) emit one SCOPE.Pattern/transaction_boundary stamping transactional=true, framework=sea_orm, transaction_api=sea_orm_begin|sea_orm_transaction_closure, the db_handle receiver, and the enclosing fn via `function`. No transitive propagation. Proven by TestRustTx_SeaOrmBegin + TestRustTx_SeaOrmClosure. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.rust.orm.seaorm ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
