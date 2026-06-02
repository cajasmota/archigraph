<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.jsts.ogm.grafeo` — grafeo-ogm (Neo4j TS OGM, GraphQL-SDL-driven)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [JS/TS](../by-language/jsts.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | — `not_applicable` | — | — | — | graph DB OGM — no relational table/model concept; the SCOPE.Schema/node is the unit (see schema_extraction). |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | 🟢 `partial` | `2026-06-02` | 3628-grafeo | `internal/custom/javascript/grafeo.go`<br>`internal/custom/javascript/grafeo_test.go` | grafeo-ogm declares its graph model in GraphQL SDL (standalone .graphql file or inline `new OGM({ typeDefs })`). Each `type|interface X @node` is extracted as a SCOPE.Schema/node keyed on its Neo4j label (the type name, or the first entry of @node(labels:[...])). `@relationshipProperties` edge-property types are correctly EXCLUDED (negative case, value-asserted). Regex over the balanced type body; partial (scalar field types not individually emitted as property entities). |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | 🟢 `partial` | — | 3628-grafeo | `internal/custom/javascript/grafeo.go` | grafeo @relationship fields are extracted as SCOPE.Component/relationship entities carrying relation_type, direction, field_name, owner_node, target_type, relationship_properties (the @relationshipProperties edge type) and target_node (when resolved). |
| Foreign key extraction | — `not_applicable` | — | — | — | graph DB — no foreign-key concept |
| Lazy loading recognition | — `not_applicable` | — | — | — | graph DB — no lazy-loading concept |
| Relationship extraction | ✅ `full` | `2026-06-02` | 3628-grafeo | `internal/custom/javascript/grafeo.go`<br>`internal/custom/javascript/grafeo_test.go` | grafeo-ogm @relationship fields (`field: T @relationship(type:"REL", direction: OUT|IN, properties:"Props")`) are extracted AND emitted as traversable GRAPH_RELATES graph-schema edges owner-node -> target-node (mirrors the neomodel #3609 / neogma #3610 sibling OGMs and the Java SDN template #3663); the domain graph topology is a navigable subgraph rather than opaque string props. Full when the @relationship field's target GraphQL type resolves to a same-document @node type (value-asserting tests TestGrafeoGraphRelatesOutgoing: Book -GRAPH_RELATES(WRITTEN_BY,OUTGOING)-> Author; direction IN -> INCOMING; @node(labels:[...]) primary-label owner; self-edge). Cross-document / non-@node targets are honest-partial (kept as the target_type prop only, no edge). Works on both standalone .graphql schema files and inline `new OGM({ typeDefs })` TS template literals. |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | — `not_applicable` | — | — | — | grafeo issues no hand-written Cypher/driver calls in user code — queries are compiled internally by the OGM from typed find/create/update/delete + select trees, so there is no user-authored query string to attribute. The graph node-label query attribution for the raw neo4j-driver is tracked on lang.jsts.driver.neo4j. |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | — | — | — | grafeo has no migration system — the GraphQL SDL schema IS the source of truth; there are no versioned migration files to parse. |
| Migration schema ops | — `not_applicable` | — | — | — | no migration files — schema changes are made directly in the .graphql SDL. |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | 🔴 `missing` | — | 3628-transaction-function-stamping | — | grafeo executes via the neo4j-driver session/transaction API under the hood; explicit transaction-function stamping is not yet modelled (shared with the sibling OGMs). |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.jsts.ogm.grafeo ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
