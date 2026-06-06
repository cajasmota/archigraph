<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.csharp.driver.mongodb` — MongoDB.Driver (C#)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C#](../by-language/csharp.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 12

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | — `not_applicable` | — | — | — | — |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | 🟢 `partial` | — | backfill:dictionary-completeness | `internal/custom/csharp/driver_schema.go` | MongoDB.Bson [BsonCollection]/[BsonElement]/[BsonIgnore] attrs detected via reBsonCollection/reBsonElement/reBsonIgnore |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | raw driver — no ORM relationship/association/FK/lazy-load layer |
| Foreign key extraction | — `not_applicable` | — | — | — | raw driver — no ORM relationship/association/FK/lazy-load layer |
| Lazy loading recognition | — `not_applicable` | — | — | — | raw driver — no ORM relationship/association/FK/lazy-load layer |
| Relationship extraction | — `not_applicable` | — | — | — | raw driver — no ORM relationship/association/FK/lazy-load layer |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-06-05` | [link](https://github.com/cajasmota/archigraph/issues/3645) | `internal/engine/orm_queries.go`<br>`internal/engine/orm_queries_drivers_other.go`<br>`internal/engine/orm_queries_drivers_other_test.go` | Driver topology: the `db.GetCollection<T>("x")` literal (csGetCollectionRe) is captured via scanCSharpDrivers (import-gated on mentionsCSharpMongo); QUERIES edge to Class:<Collection>, dynamic collection names honest-skipped (#3645). |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | — | — | — | — |
| Migration schema ops | 🔴 `missing` | — | 3628 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | — |

## Framework-specific

### Aggregation Joins

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Aggregation join extraction | ✅ `full` | — | 3848 | `internal/engine/orm_queries_csharp_mongo_agg.go`<br>`internal/engine/orm_queries_csharp_mongo_agg_test.go` | scanCSharpMongoAggregation (#3848, epic #3837) parses MongoDB.Driver aggregation $lookup into a JOINS_COLLECTION edge aggregating-collection -> from collection (Class:Book -> Class:Author) plus a SCOPE.DataAccess stage entity, matching the Python/Go/Java/Mongoose contract. Two idioms: the fluent positional db.GetCollection<Book>("books").Aggregate().Lookup("authors","authorId","_id","author") overload, and the new BsonDocument("$lookup", new BsonDocument { { "from", "authors" }, ... }) pipeline stage (C# collection-initialiser tuple form, the analogue of Go bson.D; the { "from": "authors" } colon map form is also accepted). Aggregating collection resolved file-scoped from GetCollection<T>("books") (string-literal first, else the <T> generic entity name) or an IMongoCollection<Book> field typing. Value-asserting tests TestMongoAggCSharp_FluentLookup_GetCollectionString + TestMongoAggCSharp_BsonDocumentLookup assert the joined-collection node ids. Honest-partial: dynamic from (.Lookup(var,..) / { "from", var }), unresolvable aggregating collection, and cross-file pipeline assembly stay unresolved (no fabricated edge). |

## Related extraction records

This record provides code-level coverage for the
[`db.mongodb`](./db.mongodb.md) hub record (MongoDB (collections)),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.csharp.driver.mongodb ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
