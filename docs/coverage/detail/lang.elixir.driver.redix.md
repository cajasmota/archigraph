<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `lang.elixir.driver.redix` — Redix

Auto-generated. Back to [summary](../summary.md).

- **Language:** [elixir](../by-language/elixir.md)
- **Category:** [orm](../by-category/orm.md)
- **Subcategory:** ORM / Data Mapper
- **Capability cells:** 11

## Capabilities


### Models

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Model extraction | — `not_applicable` | — | — | — | Raw Redis key-value driver; commands carry only keys, not a schema/model definition. No model entity to extract. |
| Model lifecycle extraction | 🔴 `missing` | — | 3628 | — | — |
| Schema extraction | — `not_applicable` | — | — | — | Raw Elixir DB driver; no schema/model definition. Schema belongs to Ecto ORM layer, not the driver. |

### Relationships

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Association extraction | — `not_applicable` | — | — | — | Raw driver has no association/relationship concept; Ecto handles associations independently. |
| Foreign key extraction | — `not_applicable` | — | — | — | Foreign key awareness lives in Ecto schema layer, not the raw driver. |
| Lazy loading recognition | — `not_applicable` | — | — | — | Raw driver; no lazy loading concept. |
| Relationship extraction | — `not_applicable` | — | — | — | Raw driver protocol; relationship modelling is Ecto's responsibility. |

### Queries

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Query attribution | ✅ `full` | `2026-06-06` | [link](https://github.com/cajasmota/archigraph/issues/4271) | `internal/engine/orm_queries.go`<br>`internal/engine/orm_queries_drivers_other.go`<br>`internal/engine/orm_queries_drivers_other_test.go` | scanElixirDrivers attributes Redix data-access commands to the KEYSPACE they touch (Redis is key-value: no table/collection). emitRedisTargets + elixirRedixCmdKeyRe match Redix.command/command!/noreply_command(conn, [CMD, key, ...]) where CMD is the first list element and the key is the second; the keyspace = the key prefix before the first ':' (else the whole key), via redisKeyspaceFromLiteral + capitalisedSingular. QUERIES edge caller->Class:<keyspace>, orm=redis, op from the command verb (GET/HGET->find, SET/HSET->create, INCR/EXPIRE->update, DEL->delete). PUBLISH/SUBSCRIBE are pub/sub (synthesizeElixirRedisPubSub) and excluded. Interpolated (#{}) and bare-variable keys honest-skipped. Value-asserting tests TestDriver_ElixirRedixHGet (load->Class:User find), TestDriver_ElixirRedixSet (save->Class:Session create), TestDriver_ElixirRedixDel (evict->Class:User delete), TestDriver_ElixirRedixBareKey (Class:Flag); negatives TestDriver_ElixirRedixInterpolatedKeySkipped, TestDriver_ElixirRedixVariableKeySkipped, TestDriver_ElixirRedixPublishNotAttributed. |

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
[`db.redis`](./db.redis.md) hub record (Redis (keys)),
which tracks the same technology at a higher level.

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update lang.elixir.driver.redix ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
