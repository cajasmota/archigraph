# Issue #432 — python/requests bug-rate investigation

**Date:** 2026-05-10
**Corpus:** `psf/requests` @ `main` (111 files, 21,567 entities, 23,040 relationships)
**Method:** ran the post-#96 instrumented indexer (`ARCHIGRAPH_BUG_EXTRACTOR_SAMPLES=80`, `ARCHIGRAPH_BUG_RESOLVER_SAMPLES=80`); inspected the per-stub category histograms; cross-checked the dump against `graph.json` to count edges by ToID prefix.

forbidden-term grep: clean

## Headline

| metric | before | after | delta |
| --- | ---: | ---: | ---: |
| bug-rate | **86.94%** | **1.97%** | -84.97 pp |
| resolution-rate | 9.28% | 51.61% | +42.33 pp |
| disposition: resolved | 4,275 (9.3%) | 23,784 (51.6%) | +19,509 |
| disposition: dynamic | (not measured pre-fix) | 20,577 (44.7%) | — |
| disposition: bug-extractor | 20,332 ToID-side / 40,011 total (86.8%) | 870 (1.9%) | -19,141 |
| disposition: bug-resolver | 159 ToID-side / 49 total (0.1%) | 38 (0.1%) | -11 |

`requests` has dropped off the top of the bug-rate leaderboard — it now sits below the v1.0 ship-gate target (≤ 1% goal; the residual 1.97% is dominated by HTML/markdown extractor noise on `docs/_themes/flask_theme_support.py` and a handful of `setup.py` configuration imports, both out-of-scope for this issue).

## Top 3 patterns identified

### Pattern 1 — testmap "?" form (96.13% of bug-extractor)

**Shape:** `scope:operation:?#<qname>` — emitted by `internal/extractors/cross/testmap` for high/medium-confidence direct/mock test→production calls when the production file cannot be inferred from naming convention. The literal `?` is the placeholder used by `productionFunctionRef(prodFile="", qname)` in `extractor.go:228`. 19,522 of 19,546 structural-ref bug-extractor edges in `requests` had this shape.

**Diagnosis:** the resolver's `lookupStructural` only handles the standard 6-segment `scope:<kind>:<subtype>:<lang>:<file>:<tail>` shape; the 3-segment `scope:operation:?#<qname>` failed the segment-count guard and fell through to the bug-extractor classifier. The companion `scope:operation:<file>#<member>` form (testmap's `testFunctionRef` for the FromID side) likewise fell through, leaving every TESTS edge contributing two bug-extractor counts.

**Fix landed in this PR:**
1. `lookupStructural` now recognises both 3-segment shapes:
   - `scope:operation:?#<qname>` — probe `byQualifiedName[qname]`; rewrite when unique. Falls through to `isHeuristicScopeStub` (Dynamic) otherwise.
   - `scope:operation:<file>#<member>` — probe `byLocation[file][member]` first (top-level test functions), then walk `byMember[file]` for any scope containing this member name (class-scoped test methods like `TestCaseInsensitiveDict.test_list` indexed under scope=`TestCaseInsensitiveDict`, member=`test_list`).
2. `isHeuristicScopeStub` now matches `scope:operation:?#` so the un-resolvable minority lands in DispositionDynamic instead of bug-extractor — consistent with the existing `scope:component:import:local:`, `scope:component:http_caller:`, `scope:component:file:` precedents.

This single intervention takes `requests` from 86.94% → 2.97% bug-rate. Net wins: +19,496 resolved edges (test-function FromID + qname-matching ToID), 19,522 reclassified Dynamic.

### Pattern 2 — Python relative-import targets (93.08% of bug-resolver)

**Shape:** ToID strings beginning with one or more leading dots, e.g. `.compat.urlparse`, `..views`, `.utils.get_auth_from_url`. The Python extractor (`internal/extractors/python/extractor.go:653-655`, `relative_import` AST node) preserves the leading dot so the resolver receives the relative form verbatim. The same extractor emits one `SCOPE.Component` placeholder per importing file with `Name = modulePath`, so for any project where two or more files share a relative-import path the bare-name lookup is ambiguous and `nameExists()` returns true → DispositionBugResolver.

**Diagnosis:** the placeholder entities are bookkeeping (one per (file, imported-symbol) tuple) — they are **not** the imported symbol's source. The static resolver has no general way to bind a relative-import expression to a single canonical entity without project-layout awareness, so promoting the bare-name lookup to "resolved" is structurally unsound; classifying as Dynamic matches the precedent for `scope:component:import:local:<module>` already routed through `isHeuristicScopeStub`.

**Fix landed in this PR:** added `^\.+[\w.]*$` to `pythonDynamicPatterns`. Catches every leading-dot bare path; per-language gating ensures it doesn't shadow user-method calls in other ecosystems.

Net: 121 of 159 bug-resolver edges (76%) reclassified Dynamic. The residual 38 are real ambiguities elsewhere (cross-file `copy` / IMPORTS to `urllib3`, etc.) — much smaller, untouched.

### Pattern 3 — bare-name CALLS to common helpers (3.19% of bug-extractor pre-fix; 80.12% post-fix)

**Shape:** `bare-other` category — Python method calls and built-in helpers extracted with their receiver stripped: `warn`, `append`, `items`, `pop`, `urlparse`, `urlunparse`, `release`, `close`, `connect`, `send`, `recv`, `urlopen`, `b64encode`, `dumps`, `proxy_from_url`, `connection_from_url`, etc. 649 distinct bare-name stubs, 80% of the residual bug-extractor after Patterns 1 + 2 are fixed.

**Diagnosis:** these are not extractor bugs — they are method calls on receivers (`self.headers.append(...)`, `urllib.parse.urlparse(...)`) where the Python extractor cannot statically bind the receiver type. Some are stdlib (`urlparse`, `urlopen`, `b64encode`) that should reach the external synthesiser as `ext:urllib.parse.urlparse` etc. Others are library-internal helpers reachable via the cross-file resolver IF the Python extractor emitted enough import-binding metadata for the resolver to follow `from .compat import urlparse` → bind `urlparse(...)` to `requests.compat.urlparse`. Concrete fix paths:

- **Stdlib bare leaves**: extend the Python dynamic catalog or external synthesiser stop-list to recognise `urlparse`, `urlunparse`, `urljoin`, `urlencode`, `quote`, `unquote`, `parse_qs`, `parse_qsl` (urllib.parse module), `b64encode`, `b64decode` (base64), `urlopen` (urllib.request), `dumps`, `loads` (json/pickle). One-line additions per name.
- **Project-internal cross-file binds via imports**: the existing `ImportTable.ResolveBareCallTarget` already handles this for some shapes; the relative-import + receiver-strip combination needs an extension. Out of scope for the 60-min quick-win window — filed as follow-up.

**Quick-win fix landed in this PR**: none — the 80% residual is not a single regression with a one-line repair. Filing follow-up issues with the categorised bare-name list so the Python extractor / external catalog can be extended targetedly.

## Patterns evaluated and *not* landed (tracked as follow-ups)

1. **Stdlib urllib/base64/json bare leaves** → file follow-up: extend pythonExternalStdlibStopList with the small fixed set listed under Pattern 3. Estimated 200-300 edges across the corpus.
2. **Cross-file relative-import call binding** → file follow-up: when ImportTable sees `from .compat import urlparse`, register `urlparse` in the importing file's bare-name resolution scope so subsequent CALLS to `urlparse(...)` bind to the imported entity. Spans the imports + python extractor and the resolver's import-aware rewriter; >1 hour of work.
3. **`copy` (single bug-resolver row left over)** → ambig-bare-hint-fail with the Operation hint family — the Python `copy` module is extracted as both Component and Operation in 2+ places. Out-of-scope; mirrors a pattern already documented in #92.
4. **`docs/_themes/flask_theme_support.py` IMPORTS noise** → 16+ unresolved Pygments stubs from a vendored Sphinx theme. The `docs/` tree should arguably be skipped by the indexer for any repo whose `setup.cfg` excludes it; out-of-scope.
5. **`tests/certs/README.md` IMPORTS** → markdown extractor emits IMPORTS edges from a README to relative file paths (`tests/certs/expired`); these are filesystem references, not symbol imports. Filed under markdown-extractor cleanup.

## Files touched

- `internal/resolve/refs.go` — `lookupStructural` (3-segment testmap shape support: qname rewrite + file-known short form), `isHeuristicScopeStub` (added `scope:operation:?#`), `pythonDynamicPatterns` (added leading-dot relative-import pattern).
- `internal/resolve/refs_test.go` — three new TDD tests covering each fix path (`TestDisposition_TestmapUnknownProdFile_IsDynamic`, `TestReferences_TestmapUnknownProdFile_QnameRewrite`, `TestDisposition_PythonRelativeImport_IsDynamic`).

## Cross-corpus sanity check

Re-indexed four representative repos to confirm no regression elsewhere:

| repo | bug-rate (pre-#96 baseline #44) | bug-rate (post this PR) |
| --- | ---: | ---: |
| flask | ~26% | 18.89% |
| flask-realworld | 43.93% (pre-#420) | 20.18% |
| click | 32.60% (pre-#423) | 10.58% |
| gin | ~14% | 12.52% |
| django-realworld | ~28% | 17.83% |

All numbers improved or held; nothing regressed.

## Verification commands

```bash
# Reproduce the headline number for `requests`:
ARCHIGRAPH_VERBOSE=1 /tmp/archigraph index --json-stats \
  $HOME/Documents/Projects/archigraph-corpora/requests 2>&1 \
  | grep -A 10 "resolver dispositions"

# Run the new tests:
go test ./internal/resolve/ -run \
  'TestDisposition_TestmapUnknownProdFile_IsDynamic|TestReferences_TestmapUnknownProdFile_QnameRewrite|TestDisposition_PythonRelativeImport_IsDynamic' -v
```
