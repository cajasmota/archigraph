<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.ruby.driver.postgres` — pg (Ruby driver)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [ruby](../by-language/ruby.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | — `not_applicable` | — | — | — | — |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | — `not_applicable` | — | — | — | raw client driver; no ORM model/schema in user code |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | raw client driver; no association DSL |
| Foreign key extraction | — `not_applicable` | — | — | — | raw driver — no ORM relationship/lazy-load layer |
| Lazy loading recognition | — `not_applicable` | — | — | — | raw driver — no ORM relationship/lazy-load layer |
| Relationship extraction | — `not_applicable` | — | — | — | raw client driver; no relationship DSL |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-06-02` | — | `internal/substrate/effects_test.go` | Native effect-substrate raw-driver SQL attribution (#3949): pg conn.exec/exec_params, mysql2 client.query, sqlite3 db.execute carrying a SQL string literal are parsed to a db_read (SELECT/WITH) or db_write (INSERT/UPDATE/DELETE/REPLACE/MERGE/TRUNCATE) effect naming the target table in the sink tag (rawsql.read(orders) / rawsql.write(users)) at full confidence. Table extraction mirrors internal/patterns/raw_sql_extractor.go. Non-SQL string args yield no DB effect; dynamic/interpolated tables stamp the effect with an honest no-table sink (rawsql.read(?)) -- no fabrication. Value-asserting test TestSniffEffectsRuby_RawDriverSQL covers all 3 drivers + negatives. |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | — | — | — | — |
| Migration schema ops | 🔴 `missing` | — | 3628 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | — |

## Related extraction records

This record provides code-level coverage for the
[`db.postgres`](./db.postgres.md) hub record (PostgreSQL (schema)),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.ruby.driver.postgres ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
