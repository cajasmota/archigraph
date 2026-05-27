<!-- DO NOT EDIT — generated from docs/coverage.json by 'go run ./tools/coverage gen' -->
# Coverage — category: `security`

Auto-generated. Back to [summary](../summary.md).

- Records: **5**
- Valid capability keys: `auth_policy`, `secret_detection`, `sql_injection`

## Records

| ID | Language | Label | Capabilities |
|----|----------|-------|--------------|
| [security.auth-java](../detail/security.auth-java.md) | [java](../by-language/java.md) | Auth policy resolver (Java/Kotlin — Phase 1 of #1942) | auth_policy=full |
| [security.auth-other](../detail/security.auth-other.md) | [multi](../by-language/multi.md) | Auth policy resolver (Python / NestJS / Go / Ruby / ASP.NET — Phases 2-4 of #1942) | auth_policy=missing |
| [security.csrf](../detail/security.csrf.md) | [multi](../by-language/multi.md) | CSRF heuristic detector | auth_policy=full |
| [security.secrets](../detail/security.secrets.md) | [multi](../by-language/multi.md) | Secret material extraction (Phase 1 security audit) | secret_detection=full |
| [security.sql-injection](../detail/security.sql-injection.md) | [multi](../by-language/multi.md) | SQL injection heuristic (f-string / .format() / % interpolation into SQL) | sql_injection=full |
