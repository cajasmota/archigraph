<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.c-cpp.driver.mysql-connector-cpp` — MySQL Connector/C++

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C/C++](../by-language/c-cpp.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/cpp/driver_schema.go` | Regex: CREATE TABLE in exec()/mysql_query() string literals → table entity with table_name |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | ✅ `full` | `2026-05-30` | — | `internal/custom/cpp/driver_schema.go` | Regex: paren-balanced CREATE TABLE body → columns with column_name/column_type/parent_table |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | raw/NoSQL driver — no ORM relationship/association/FK/lazy-load layer |
| Foreign key extraction | — `not_applicable` | — | — | — | raw/NoSQL driver — no ORM relationship/association/FK/lazy-load layer |
| Lazy loading recognition | — `not_applicable` | — | — | — | raw/NoSQL driver — no ORM relationship/association/FK/lazy-load layer |
| Relationship extraction | — `not_applicable` | — | — | — | raw/NoSQL driver — no ORM relationship/association/FK/lazy-load layer |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-05-30` | — | `internal/custom/cpp/driver_schema.go` | Regex: SQL verb from exec()/exec_params() + mysql_query()/mysql_real_query() free-function literals |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | — | — | — | raw driver — no built-in migration system |
| Migration schema ops | 🔴 `missing` | — | 3628 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | — |

## Related extraction records

This record provides code-level coverage for the
[`db.mysql`](./db.mysql.md) hub record (MySQL / MariaDB (schema)),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.c-cpp.driver.mysql-connector-cpp ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
