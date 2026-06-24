<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.jsts.orm.kysely` — Kysely (type-safe query builder)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [JS/TS](../by-language/jsts.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | — `not_applicable` | — | — | — | Kysely is a type-safe SQL query builder, not an ORM — its `Database` interface is a compile-time TypeScript type with no runtime model/entity layer to extract. The schema is defined imperatively in migrations, not as decorated model classes. |
| Model lifecycle extraction | — `not_applicable` | — | — | — | No model/entity layer; Kysely has no lifecycle hooks (no save/beforeCreate equivalents) — queries are explicit chains, not active-record instances. |
| Schema extraction | 🔴 `missing` | — | 5491 | — | Kysely schema is declared imperatively via the migration schema-builder DSL (db.schema.createTable(...).addColumn(...)). Not yet parsed; tracked separately from the query/effect extraction landed in #5491. |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | Kysely is a SQL query builder with no ORM model layer; associations are expressed ad-hoc per query via .innerJoin()/.leftJoin(), not declared on a model — there is no static association to extract. |
| Foreign key extraction | 🔴 `missing` | — | 5491 | — | Foreign keys are declared imperatively in the migration schema-builder DSL (.addForeignKeyConstraint(...)); not yet parsed — part of the deferred schema_extraction work. |
| Lazy loading recognition | — `not_applicable` | — | — | — | Kysely is a SQL query builder with no ORM model layer; there is no relation or lazy-loading concept to recognise — every join is explicit in the query chain. |
| Relationship extraction | 🔴 `missing` | — | 5491 | — | Table relationships are only declared via migration foreign-key constraints; not yet parsed — part of the deferred schema_extraction work. |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-06-24` | 5491 | `internal/substrate/effect_sinks_jsts.go`<br>`internal/substrate/effect_sinks_kysely_5491_test.go` | #5491 Kysely query-builder data-access effects: the chain ROOT method on a db/kysely/trx receiver determines read vs write — selectFrom("t") -> db_read; insertInto/updateTable/deleteFrom/replaceInto("t") -> db_write — terminating in .execute()/.executeTakeFirst()/.executeTakeFirstOrThrow()/.stream(). The string-literal table arg is captured in a model/table-bearing sink tag (kysely.read:user / kysely.write:post) attributed to the enclosing function, mirroring the #5490 Prisma model-bearing uplift. Raw sql`…`.execute(db) is classified by the leading SQL keyword (SELECT/WITH -> read; INSERT/UPDATE/DELETE/REPLACE -> write; undeterminable -> generic db_read, sink kysely.raw); (db|kysely|trx).executeQuery(...) -> generic db_read. The distinctive chain-root + db/kysely/trx receiver gate (trx = transaction-callback handle) stops an unrelated .execute() from being misread. effect_sinks_jsts.go. Proven by TestKyselyReadEffects_5491 / TestKyselyWriteEffects_5491 / TestKyselyReplaceInto_5491 / TestKyselyRawSQL_5491 / TestKyselyTrxAndKyselyReceiver_5491 / TestKyselyNonKyselyExecuteNotCredited_5491. |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | 🔴 `missing` | — | 5491 | — | Kysely migrations are plain TS modules exporting up()/down() that call the imperative schema-builder DSL; not yet parsed. Tracked alongside schema_extraction. |
| Migration schema ops | 🔴 `missing` | — | 5491 | — | db.schema.createTable/alterTable/dropTable migration ops are not yet converged to MODIFIES_TABLE (the #3628 engine pass that already covers knex/typeorm/sequelize). |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 5491 | — | db.transaction().execute(async (trx) => ...) interactive-transaction boundary stamping (transactional=true / tx_source) is not yet emitted; the trx handle IS receiver-gated for query/effect attribution (#5491). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.jsts.orm.kysely ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
