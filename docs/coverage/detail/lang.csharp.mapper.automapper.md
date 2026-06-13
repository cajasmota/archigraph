<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.csharp.mapper.automapper` — AutoMapper (.NET object-object mapper)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [C#](../by-language/csharp.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | — `not_applicable` | — | — | — | object-object mapper config (AutoMapper Profile / Mapster) — not a DB ORM: no models/queries/migrations/transactions of its own; the mapping is the source->dest relationship surfaced via relationship_extraction. |
| Model lifecycle extraction | — `not_applicable` | — | — | — | object-object mapper config (AutoMapper Profile / Mapster) — not a DB ORM: no models/queries/migrations/transactions of its own; the mapping is the source->dest relationship surfaced via relationship_extraction. |
| Schema extraction | — `not_applicable` | — | — | — | object-object mapper config (AutoMapper Profile / Mapster) — not a DB ORM: no models/queries/migrations/transactions of its own; the mapping is the source->dest relationship surfaced via relationship_extraction. |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | object-object mapper config (AutoMapper Profile / Mapster) — not a DB ORM: no models/queries/migrations/transactions of its own; the mapping is the source->dest relationship surfaced via relationship_extraction. |
| Foreign key extraction | — `not_applicable` | — | — | — | object-object mapper config (AutoMapper Profile / Mapster) — not a DB ORM: no models/queries/migrations/transactions of its own; the mapping is the source->dest relationship surfaced via relationship_extraction. |
| Lazy loading recognition | — `not_applicable` | — | — | — | object-object mapper config (AutoMapper Profile / Mapster) — not a DB ORM: no models/queries/migrations/transactions of its own; the mapping is the source->dest relationship surfaced via relationship_extraction. |
| Relationship extraction | ✅ `full` | `2026-06-13` | — | `internal/custom/csharp/object_mapping.go`<br>`internal/custom/csharp/object_mapping_test.go` | #5074: AutoMapper Profile subclass (class X : Profile) -> SCOPE.Pattern(mapping_profile); CreateMap<TSrc,TDest>() (+chained .ForMember member maps, .ReverseMap) -> SCOPE.Pattern(object_mapping) carrying a MAPS_TO edge Class:<TSrc> -> Class:<TDest> (resolvable to the real DTO/entity classes via the byName Class: fallback), with member_map_count and a reverse MAPS_TO for .ReverseMap(). Proven by TestObjectMappingAutoMapperMapster. Honest-partial: open generics / runtime-typed sources carry no fabricated edge. |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | — `not_applicable` | — | — | — | object-object mapper config (AutoMapper Profile / Mapster) — not a DB ORM: no models/queries/migrations/transactions of its own; the mapping is the source->dest relationship surfaced via relationship_extraction. |

### Migrations

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Migration parsing | — `not_applicable` | — | — | — | object-object mapper config (AutoMapper Profile / Mapster) — not a DB ORM: no models/queries/migrations/transactions of its own; the mapping is the source->dest relationship surfaced via relationship_extraction. |
| Migration schema ops | — `not_applicable` | — | — | — | object-object mapper config (AutoMapper Profile / Mapster) — not a DB ORM: no models/queries/migrations/transactions of its own; the mapping is the source->dest relationship surfaced via relationship_extraction. |

### Transactions

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Transaction function stamping | — `not_applicable` | — | — | — | object-object mapper config (AutoMapper Profile / Mapster) — not a DB ORM: no models/queries/migrations/transactions of its own; the mapping is the source->dest relationship surfaced via relationship_extraction. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.csharp.mapper.automapper ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
