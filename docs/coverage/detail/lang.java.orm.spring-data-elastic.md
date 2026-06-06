<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.java.orm.spring-data-elastic` — Spring Data Elasticsearch

Auto-generated. Back to [summary](../summary.md).

- **Language:** [java](../by-language/java.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | 🔴 `missing` | `2026-06-05` | [link](https://github.com/cajasmota/archigraph/issues/4271) | — | HONESTLY LEFT MISSING this round (#4271 was query_attribution-scoped). @Document(indexName=..) is now consumed by scanJavaSpringDataElastic for query topology (index attribution), but it is NOT yet emitted as a standalone schema/model entity: no Go pass extracts the @Document/@Field/@Id property schema into a SCOPE.Schema/model record (the only such code, internal/custom/java/spring_ecosystem.go, is the dead custom_extractor that never runs in Go). Model extraction needs a separate native extractor pass. |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | 🔴 `missing` | `2026-05-29` | [link](https://github.com/cajasmota/archigraph/issues/3586) | `internal/custom/java/extractors_test.go`<br>`internal/custom/java/spring_ecosystem.go` | — |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | 3095 | — | NoSQL store has no relational join/FK/lazy-load concept; not_applicable by design. |
| Foreign key extraction | — `not_applicable` | — | 3095 | — | NoSQL store has no relational join/FK/lazy-load concept; not_applicable by design. |
| Lazy loading recognition | — `not_applicable` | — | 3095 | — | NoSQL store has no relational join/FK/lazy-load concept; not_applicable by design. |
| Relationship extraction | — `not_applicable` | — | 3095 | — | NoSQL store has no relational join/FK/lazy-load concept; not_applicable by design. |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-06-05` | [link](https://github.com/cajasmota/archigraph/issues/4271) | `internal/engine/orm_queries_drivers_other.go`<br>`internal/engine/orm_queries_drivers_other_test.go` | Native Go query-topology pass (scanJavaSpringDataElastic, #4271): @Document(indexName="products") entity emits a QUERIES edge class -> Class:<index> (the @Document indexName IS the index attribution), and @Query("{...}") on an ElasticsearchRepository method emits method -> Class:<index> where the index is resolved from the file's @Document entity (the extended-JSON @Query body is a query, not an index name). The shared index:"x"/.Index("x") literal forms (emitElasticTargets) cover ElasticsearchOperations search-request builders. Gated on org.springframework.data.elasticsearch / ElasticsearchRepository / ElasticsearchOperations. Honest limit: dynamic index names (IndexCoordinates.of(var)) -> no edge. |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | `2026-05-29` | — | — | ORM model-definition layer; database migration files are owned by Flyway/Liquibase, not the ORM itself. Same rationale as lang.java.orm.jooq and lang.java.orm.neo4j N/A. |
| Migration schema ops | 🔴 `missing` | — | 3628 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | — |

## Related extraction records

This record provides code-level coverage for the
[`db.elasticsearch`](./db.elasticsearch.md) hub record (Elasticsearch (indices)),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.java.orm.spring-data-elastic ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
