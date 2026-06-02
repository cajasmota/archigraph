<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# `db.cache` — Caching (regions/keys)

Auto-generated. Back to [summary](../summary.md).

- **Language:** [multi](../by-language/multi.md)
- **Category:** [databases](../by-category/databases.md)
- **Capability cells:** 2

## Capabilities

| Capability | Status | Verified at | Issue | Cites | Notes |
|------------|--------|-------------|-------|-------|-------|
| Dependency attribution | 🟢 `partial` | — | 3692 | `internal/custom/java/caching.go`<br>`internal/custom/javascript/caching.go`<br>`internal/custom/python/caching.go`<br>`internal/custom/ruby/caching.go` | CACHES (read-through/write) + INVALIDATES (evict) edges from function/method to cache region/key. Cross-language consistent target ref cache:<fw>:<region>. |
| Resource extraction | 🟢 `partial` | — | 3692 | `internal/custom/java/caching.go`<br>`internal/custom/javascript/caching.go`<br>`internal/custom/python/caching.go`<br>`internal/custom/ruby/caching.go` | Cache-region/key SCOPE.Datastore nodes from Spring @Cacheable/@CacheEvict/@CachePut, Python lru_cache/Flask-Caching/cachetools, Rails.cache.fetch/delete, NestJS @CacheKey + cache-manager. Dynamic keys honest-partial. |

## Provenance

This record is sourced from `docs/coverage/registry.json`. To update it, edit the JSON
(or use `go run ./tools/coverage update db.cache ...`) then regenerate:

```
go run ./tools/coverage validate
go run ./tools/coverage gen
```
