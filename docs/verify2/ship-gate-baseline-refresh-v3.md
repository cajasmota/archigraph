# Ship-gate baseline refresh v3 — 2026-05-18 (Refs #44)

Re-runs the same 32-repo VERIFY-2 corpus measured in
`docs/verify2/ship-gate-baseline-refresh-2026-05-10-v2.md` after the DSL-wave-2
batch landed on `main`:

- #446 Python Flask extensions + Marshmallow DSL classified
- #447 Python Django ORM + DRF DSL classified
- #448 Ruby Rails ActionPack internals classified
- #449 Ruby Sidekiq DSL + Redis pipeline classified

## Headline

| metric | refresh-2 (post-#435..#441) | refresh-3 (post-#446..#449) | delta |
| --- | ---: | ---: | ---: |
| repos measured | 32 | 32 | +0 |
| total files | 6,275 | 6,275 | +0 |
| total relationships | 246,010 | 246,010 | +0 |
| total endpoints | 492,020 | 492,020 | +0 |
| **aggregate bug_rate** | **16.50 %** | **15.95 %** | **-0.55 pp** |
| #44 ship-gate target (≤ 1 %) | NOT MET | NOT MET | — |

Total file/relationship/endpoint counts are unchanged because none of
#446/#447/#448/#449 alter extraction or relationship emission — they
re-classify already-emitted endpoints out of `bug-extractor` /
`bug-resolver` into `resolved`. Net effect is a 0.55 pp improvement,
two repos crossing their per-repo bars, and no regressions.

## Per-repo comparison

| repo | lang | files | endpoints | OLD bug % | NEW bug % | delta pp | bar % | hit |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | :---: |
| actix-examples | rust | 460 | 12,200 | 19.41 | 19.40 | -0.01 | 25 | YES |
| aspnetcore-realworld | csharp | 97 | 2,576 | 17.93 | 17.93 | +0.00 | 35 | YES |
| chi | go | 93 | 7,498 | 12.74 | 12.74 | -0.00 | 35 | YES |
| click | python | 138 | 14,948 | 10.58 | 10.16 | -0.42 | 10 | NO |
| django-realworld | python | 48 | 1,060 | 17.83 | 13.96 | -3.87 | 15 | YES |
| etcd | go | 424 | 59,010 | 19.42 | 19.42 | -0.00 | 35 | YES |
| exposed | kotlin | 117 | 8,472 | 13.09 | 13.09 | +0.00 | 25 | YES |
| express | javascript | 208 | 2,136 | 19.62 | 19.62 | -0.00 | 25 | YES |
| express-realworld | javascript | 66 | 692 | 19.65 | 19.65 | +0.00 | 25 | YES |
| flask | python | 225 | 11,564 | 18.89 | 16.88 | -2.01 | 10 | NO |
| flask-realworld | python | 43 | 1,868 | 20.18 | 16.70 | -3.48 | 15 | NO |
| gin | go | 121 | 22,654 | 12.52 | 12.52 | +0.00 | 35 | YES |
| kafka | java | 489 | 56,394 | 21.09 | 21.09 | +0.00 | 35 | YES |
| ktor | kotlin | 245 | 9,462 | 21.33 | 21.33 | -0.00 | 25 | YES |
| ktor-samples | kotlin | 509 | 9,230 | 31.66 | 31.66 | -0.00 | 25 | NO |
| laravel-quickstart | php | 83 | 382 | 24.35 | 24.35 | -0.00 | 25 | YES |
| laravel-routing | php | 90 | 4,950 | 20.10 | 20.10 | +0.00 | 25 | YES |
| mini-redis | rust | 33 | 2,094 | 16.67 | 16.67 | -0.00 | 25 | YES |
| nestjs | typescript | 289 | 15,514 | 24.91 | 24.91 | +0.00 | 25 | YES |
| nestjs-starter | typescript | 16 | 114 | 16.67 | 16.67 | -0.00 | 25 | YES |
| pandas | python | 197 | 60,682 | 14.65 | 14.41 | -0.24 | — | — |
| phoenix-todo-list | elixir | 69 | 1,428 | 12.75 | 12.75 | -0.00 | 25 | YES |
| rails-actionpack | ruby | 541 | 63,422 | 20.02 | 16.84 | -3.18 | 15 | NO |
| rails-realworld | ruby | 105 | 526 | 6.84 | 6.65 | -0.19 | 15 | YES |
| requests | python | 111 | 46,080 | 1.97 | 1.88 | -0.09 | 10 | YES |
| sidekiq | ruby | 85 | 9,466 | 15.24 | 13.85 | -1.39 | 15 | YES |
| spring-petclinic | java | 120 | 4,580 | 8.73 | 8.73 | +0.00 | 35 | YES |
| symfony-demo | php | 241 | 2,998 | 23.38 | 23.38 | +0.00 | 25 | YES |
| symfony-routing | php | 375 | 18,174 | 15.50 | 15.50 | +0.00 | 25 | YES |
| tokio | rust | 389 | 36,740 | 16.18 | 16.18 | -0.00 | 25 | YES |
| vapor | swift | 227 | 5,012 | 18.75 | 18.75 | +0.00 | 25 | YES |
| vapor-api-template | swift | 21 | 94 | 21.28 | 21.28 | -0.00 | 25 | YES |

## Aggregate disposition breakdown (new run, 32 repos incl. pandas)

| disposition | count | pct |
| --- | ---: | ---: |
| resolved | 288,650 | 58.67 % |
| external-known | 22,546 | 4.58 % |
| external-unknown | 25,311 | 5.14 % |
| dynamic | 77,045 | 15.66 % |
| bug-extractor | 62,040 | 12.61 % |
| bug-resolver | 16,428 | 3.34 % |
| unclassified | 0 | 0.00 % |
| **total** | **492,020** | **100.00 %** |

Resolved count is byte-identical to refresh-2 (288,650 / 58.67 %); the
DSL-wave-2 batch moved endpoints from the `bug-extractor` / `bug-resolver`
buckets into `dynamic` and `external-unknown` (DSL invocations resolved
to dynamic dispatch or external-symbol references rather than to concrete
entity IDs):

| disposition | refresh-2 | refresh-3 | delta |
| --- | ---: | ---: | ---: |
| resolved | 288,650 | 288,650 | +0 |
| external-known | 22,546 | 22,546 | +0 |
| external-unknown | 24,625 | 25,311 | +686 |
| dynamic | 74,992 | 77,045 | +2,053 |
| bug-extractor | 62,715 | 62,040 | -675 |
| bug-resolver | 18,492 | 16,428 | -2,064 |
| unclassified | 0 | 0 | +0 |

`bug-extractor + bug-resolver` drops by 2,739 endpoints
(81,207 → 78,468), driving the headline `bug_rate` from 16.50 % to 15.95 %.

## Repos crossing their per-repo bar (2 wins)

| repo | lang | OLD % | NEW % | bar % | source PR |
| --- | --- | ---: | ---: | ---: | --- |
| django-realworld | python | 17.83 | 13.96 | 15 | #447 |
| sidekiq | ruby | 15.24 | 13.85 | 15 | #449 |

Repos at their per-repo bar increased from **24/31** (refresh-2) to
**26/31** (refresh-3).

## Repos still above their bar (5)

| repo | lang | NEW % | bar % | gap pp |
| --- | --- | ---: | ---: | ---: |
| ktor-samples | kotlin | 31.66 | 25 | +6.66 |
| flask | python | 16.88 | 10 | +6.88 |
| flask-realworld | python | 16.70 | 15 | +1.70 |
| rails-actionpack | ruby | 16.84 | 15 | +1.84 |
| click | python | 10.16 | 10 | +0.16 |

Down from 7 (refresh-2) → 5 (refresh-3). `django-realworld` and
`sidekiq` cleared their bars; `flask`, `flask-realworld`, and
`rails-actionpack` improved substantially but did not fully cross.
`ktor-samples` was untouched by this batch (it was a refresh-2 target).

## Repos now at ≤ 8 %

| repo | lang | NEW % | bar % |
| --- | --- | ---: | ---: |
| requests | python | 1.88 | 10 |
| rails-realworld | ruby | 6.65 | 15 |

No new repos crossed the 8 % line in this refresh; only the same two
from refresh-2 sit below it (both improved marginally:
`requests` 1.97 → 1.88, `rails-realworld` 6.84 → 6.65).

## Repos now at ≤ 1 % (#44 ship-gate target)

None. The #44 ship-gate target remains **NOT MET**. The closest is
`requests` (1.88 %, ↑0.88 pp above target).

## Top movers (by improvement)

| repo | lang | OLD % | NEW % | delta pp | source PR |
| --- | --- | ---: | ---: | ---: | --- |
| django-realworld | python | 17.83 | 13.96 | -3.87 | #447 |
| flask-realworld | python | 20.18 | 16.70 | -3.48 | #446 |
| rails-actionpack | ruby | 20.02 | 16.84 | -3.18 | #448 |
| flask | python | 18.89 | 16.88 | -2.01 | #446 |
| sidekiq | ruby | 15.24 | 13.85 | -1.39 | #449 |
| click | python | 10.58 | 10.16 | -0.42 | #446 (transitive) |
| pandas | python | 14.65 | 14.41 | -0.24 | #446/#447 (transitive) |
| rails-realworld | ruby | 6.84 | 6.65 | -0.19 | #448 (transitive) |
| requests | python | 1.97 | 1.88 | -0.09 | #446/#447 (transitive) |
| actix-examples | rust | 19.41 | 19.40 | -0.01 | noise |

## Regressions

**None.** All 32 repos either improved or stayed flat. Untouched
languages (Go, Java, JavaScript/TypeScript, Kotlin, PHP, Swift, C#,
Elixir) all registered 0.00 pp delta — confirms the wave-2 batch is
non-regressive elsewhere.

## Methodology / reproducibility

- Worktree: `/Users/jorgecajas/Documents/Projects/archigraph-worktrees/baseline-4`
- Branch: `investigate/baseline-4` off `origin/main` @ `14d16f0` (post-#453
  merge, i.e. all four wave-2 DSL fixes landed).
- Binary: `go build -o /tmp/archigraph-baseline-4 ./cmd/archigraph` from
  worktree HEAD.
- Indexer: `archigraph index -json-stats <repo>` per repo, 600 s `gtimeout`
  cap, 6-way parallel via `xargs -P6`.
- Same 32 repos as refresh-2 and the prior baselines. Three
  framework-source repos (`django`, `nextjs`, `aspnetcore-mvc`) remain
  excluded under #96 policy.
- All 32 repos succeeded; no timeouts, no failures.
- Per-repo JSON stats: `/tmp/baseline-4-stats/<repo>.json`.
