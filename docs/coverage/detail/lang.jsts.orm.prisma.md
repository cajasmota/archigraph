<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.jsts.orm.prisma` — Prisma

Auto-generated. Back to [summary](../summary.md).

- **Language:** [JS/TS](../by-language/jsts.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | ✅ `full` | `2026-05-28` | — | `internal/engine/rules/javascript_typescript/orms/prisma.yaml`<br>`internal/engine/rules/javascript_typescript/orms/prisma_client_js.yaml`<br>`internal/engine/rules/prisma/_manifest.yaml` | — |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | ✅ `full` | `2026-06-24` | 3067 | `internal/custom/javascript/orm_build_3067_test.go`<br>`internal/custom/javascript/prisma.go`<br>`internal/custom/javascript/prisma_modular_test.go` | #5489: modular split schema (prismaSchemaFolder) — prisma/schema/*.prisma (one domain per file) is resolved as ONE logical schema. prismaModularSiblings unions model/enum names across every .prisma file in the schema folder, so cross-file @relation targets, relation-field types, and enum references resolve. Models keep their real source file. Single-schema.prisma is the union of one (no regression). Test: TestPrismaModularSplitSchema/TestPrismaSingleFileSchemaNoRegression. |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | ✅ `full` | `2026-05-29` | 3067 | `internal/custom/javascript/orm_build_3067_test.go`<br>`internal/custom/javascript/prisma.go` | — |
| Foreign key extraction | ✅ `full` | `2026-05-29` | 3067 | `internal/custom/javascript/orm_build_3067_test.go`<br>`internal/custom/javascript/prisma.go` | — |
| Lazy loading recognition | — `not_applicable` | — | — | — | Prisma uses explicit include/select — eager-only per Prisma docs; no transparent lazy loading (#3184) |
| Relationship extraction | ✅ `full` | `2026-06-24` | — | `internal/custom/javascript/orm_relationship_edges_test.go`<br>`internal/custom/javascript/prisma.go`<br>`internal/custom/javascript/prisma_modular_test.go` | Model↔model GRAPH_RELATES edges with cardinality from relation fields: Order[]→one_to_many, Type @relation(fields:...)→many_to_one, Type?→one_to_one; scalar/composite types emit no edge. #5489: under the modular split schema (prismaSchemaFolder, prisma/schema/*.prisma) the model symbol table is the UNION across all .prisma files in the schema folder, so a relation whose target model lives in a sibling file resolves cross-file. Test: TestPrismaGraphRelatesEdges/TestPrismaScalarFieldNoEdge/TestPrismaModularSplitSchema. |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-06-11` | — | `internal/engine/orm_queries_jsts.go`<br>`internal/substrate/effect_sinks_jsts.go`<br>`internal/substrate/effect_sinks_querybuilder_4335_4336_test.go` | #4335 Prisma fluent delegate data-access effects: prisma.<model>.findMany/findUnique/findFirst -> db_read; create/createMany/update/upsert/delete -> db_write; $queryRaw/$queryRawUnsafe -> db_read, $executeRaw/$executeRawUnsafe -> db_write. find*/create* distinctive (bare); raw escape-hatches matched on the prisma delegate. effect_sinks_jsts.go. |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | ✅ `full` | `2026-05-28` | — | `internal/custom/javascript/extractors_coverage_test.go`<br>`internal/custom/javascript/prisma.go` | — |
| Migration schema ops | 🔴 `missing` | — | 3628 | — | — |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | ✅ `full` | `2026-06-02` | — | `internal/extractors/javascript/extractor.go`<br>`internal/extractors/javascript/transaction_boundary_test.go`<br>`internal/txscope/txscope.go` | #3628: Prisma prisma.$transaction(async tx => ...) interactive transaction stamps transactional=true + tx_source=prisma_transaction on the enclosing fn. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.jsts.orm.prisma ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
