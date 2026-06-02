<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.python.driver.mongodb` — pymongo / motor

Auto-generated. Back to [summary](../summary.md).

- **Language:** [python](../by-language/python.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | — `not_applicable` | — | — | — | NoSQL driver — no relational schema/ORM model in user code |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | — `not_applicable` | — | — | — | NoSQL driver — no relational schema/ORM model in user code |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | raw driver — no ORM relationship/lazy-load layer |
| Foreign key extraction | — `not_applicable` | — | — | — | raw driver — no ORM relationship/lazy-load layer |
| Lazy loading recognition | — `not_applicable` | — | — | — | raw driver — no ORM relationship/lazy-load layer |
| Relationship extraction | — `not_applicable` | — | — | — | raw driver — no ORM relationship/lazy-load layer |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-06-02` | [link](https://github.com/cajasmota/archigraph/issues/3645) | `internal/engine/orm_queries.go`<br>`internal/engine/orm_queries_drivers_other.go`<br>`internal/engine/orm_queries_drivers_other_test.go`<br>`internal/engine/orm_queries_python_mongo_agg.go`<br>`internal/engine/orm_queries_python_mongo_agg_test.go` | Driver topology: db.get_collection('x') / db['x'] subscript captured via scanPythonDrivers (pyMongoCollGetRe/pyMongoSubscriptRe); QUERIES edge to Class:<Collection>; dynamic names honest-skipped. Aggregation pipelines (scanPythonMongoAggregation): each $lookup/$graphLookup stage emits a JOINS_COLLECTION edge aggregating-collection -> `from` collection plus a SCOPE.DataAccess stage entity (#3440). Pipeline resolved from an inline list literal, a same-function `pipeline = [...]` binding, OR a same-file builder function (#3866): `coll.aggregate(build_fn())` and `pipeline = build_fn(); coll.aggregate(pipeline)` follow build_fn's definition and scan its body for the returned literal (`return [...]` / `pipeline = [...]; return pipeline`), attributing the joins to the aggregating collection at the executor site (bounded 1-2 hop, same-file). The aggregating collection also resolves `get_collection(CONST)` where CONST is a module-level `NAME = "value"` literal or an UPPER_SNAKE collection-name constant, so the anchor + JOINS_COLLECTION FromID land on the named collection node (Class:Inspection) rather than the shared ext:get_collection node or an all-caps phantom. Honest-partial: cross-module/dynamic builders, non-literal builder returns, and lowercase-local-var get_collection args stay unresolved (no fabricated edge). |

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
[`db.mongodb`](./db.mongodb.md) hub record (MongoDB (collections)),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.python.driver.mongodb ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
