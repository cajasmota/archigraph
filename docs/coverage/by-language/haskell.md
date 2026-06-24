<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# haskell

**Frameworks**: 3 · **Tools**: 2 · **ORMs**: 1 · **Other**: 0

Back to [summary](../summary.md).

### Legend

Each group column shows `glyph covered/applicable` — **covered** = capabilities with extraction, **applicable** = covered + missing (not-applicable capabilities are excluded from both). The glyph is the group's **support level**:

| Glyph | Level | Meaning |
|---|---|---|
| ✅ | **Comprehensive** | every applicable capability is `full` — fixture-proven, resolves the general case |
| 🟢 | **Supported** | every applicable capability is extracted; some only *heuristically* (detected by pattern, not full AST/data-flow resolution) |
| 🟡 | **Partial** | some capabilities extracted, some still missing |
| 🔴 | **Not extracted** | nothing extracted yet |
| — | **N/A** | capability does not apply to this framework |

Examples: `🟢 20/20` = fully supported, some capabilities heuristic · `🟡 12/20` = 8 not yet extracted. Detail pages use the same palette **per cell** (✅ full · 🟢 heuristic/partial · 🔴 missing · — n/a).

## Frameworks


### Backend HTTP

| Name | Routing | Auth | Type System | Testing | Substrate | Other capabilities | Notes |
|---|---|---|---|---|---|---|---|
| [Scotty (Haskell HTTP)](../detail/lang.haskell.framework.scotty.md) | 🟡 3/7 | 🔴 0/1 | 🔴 0/4 | 🔴 0/1 | 🔴 0/24 | 🔴 0/13 | |
| [Servant (Haskell type-level API)](../detail/lang.haskell.framework.servant.md) | 🟡 3/7 | 🔴 0/1 | 🔴 0/4 | 🔴 0/1 | 🔴 0/24 | 🔴 0/13 | |
| [Yesod (Haskell web framework)](../detail/lang.haskell.framework.yesod.md) | 🟡 3/7 | 🔴 0/1 | 🔴 0/4 | 🔴 0/1 | 🔴 0/24 | 🔴 0/13 | |


## Tools

| Name | Dependency graph | Dependency usage status | Lockfile parsing | Manifest parsing | Target extraction | Notes |
|---|---|---|---|---|---|---|
| [Cabal / Stack / hpack (Haskell build)](../detail/lang.haskell.tool.cabal.md) | — | — | — | ✅ | — | |
| [hspec (Haskell testing)](../detail/test.hspec.md) | 🟢 | — | — | — | ✅ | |

## ORMs


### ORM / Data Mapper

| Name | Other capabilities | Notes |
|---|---|---|
| [persistent (Haskell ORM)](../detail/lang.haskell.orm.persistent.md) | 🟡 3/11 | |
