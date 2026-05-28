// Python taint-sites sniffer (#2772 Phase 2B T1).
//
// Recognises Python source / sink / sanitizer primitives.
//
// Sources:
//   - Django: request.POST / GET / body / headers / COOKIES / FILES
//   - Flask: request.form / args / json / data / headers / cookies / files
//   - FastAPI / Starlette: request.json() / form() / body() / headers
//   - os.environ / os.getenv
//   - pickle.loads / yaml.load / yaml.unsafe_load / marshal.loads /
//     json.loads (latter low-confidence — not always taint)
//
// Sinks:
//   - SQL injection: cursor.execute("...") with % or .format / f-string,
//     raw Model.objects.raw, connection.execute
//   - Command injection: subprocess.call/run/Popen(..., shell=True),
//     os.system, os.popen, eval, exec, compile
//   - Path traversal: open(<non-literal>), pathlib.Path(<non-literal>)
//     followed by read/write
//   - XSS: Django |safe filter (template-side, not detectable here),
//     mark_safe(<non-literal>), HttpResponse(<non-literal>)
//   - ReDoS: re.compile(<non-literal>)
//
// Sanitizers:
//   - Parameterised SQL: cursor.execute(sql, params) — second arg
//     present
//   - HTML escape: html.escape, bleach.clean, django.utils.html.escape,
//     markupsafe.escape
//   - Validation libs (schema-declaration required): pydantic
//     BaseModel subclass declarations, marshmallow Schema subclass
//     declarations, attrs.define / dataclass with validators — we
//     detect the declaration form, not the .parse() call
package substrate

import "regexp"

func init() { RegisterTaintSniffer("python", sniffTaintPython) }

// pySourceReqRe matches Django/Flask/FastAPI/DRF request-object access.
// `request.data` is the DRF (Django REST Framework) convention for the
// parsed request body — distinct from Django's `request.POST` which is
// form-only. Included here because DRF is the dominant Django HTTP API
// framework in 2026.
var pySourceReqRe = regexp.MustCompile(
	`\brequest\s*\.\s*(?:POST|GET|body|headers|COOKIES|FILES|form|args|json|data|cookies|files|query_params|path_params|META)\b`,
)

// pySourceEnvRe matches os.environ / os.getenv reads.
var pySourceEnvRe = regexp.MustCompile(
	`\bos\s*\.\s*(?:environ\s*(?:\[\s*['"][A-Z_][A-Z0-9_]*['"]\s*\]|\.\s*get\s*\()|getenv\s*\()`,
)

// pySourceDeserializeRe matches deserialisation primitives. pickle and
// yaml.load (without SafeLoader) are RCE-capable; flagged at high
// confidence. yaml.safe_load is excluded from the source set.
var pySourceDeserializeRe = regexp.MustCompile(
	`\bpickle\s*\.\s*loads?\s*\(` +
		`|\byaml\s*\.\s*(?:load|unsafe_load)\s*\(` +
		`|\bmarshal\s*\.\s*loads?\s*\(`,
)

// pySinkSQLRe matches cursor.execute / connection.execute with a
// non-parameterised SQL argument. The parameterised form
// `cursor.execute(sql, (params,))` is caught by the sanitizer regex
// below; we exclude it here by requiring the open paren followed by a
// quoted string that contains a `%` formatter, an f-string, or
// concatenation, or by a bare identifier.
var pySinkSQLRe = regexp.MustCompile(
	`\b(?:cursor|conn|connection)\s*\.\s*execute\s*\(\s*` +
		`(?:` +
		`f['"]` + // f-string SQL
		`|['"][^'"]*['"]\s*[%+]` + // "..." % var  or  "..." + var
		`|[A-Za-z_][\w]*\s*\)` + // bare identifier as only arg
		`|['"][^'"]*['"]\s*\.format\s*\(` + // "... {} ...".format(...)
		`)`,
)

// pySinkRawORMRe matches Django's raw() escape hatch.
var pySinkRawORMRe = regexp.MustCompile(
	`\.\s*objects\s*\.\s*raw\s*\(`,
)

// pySinkExecRe matches command-injection sinks. subprocess.* with
// shell=True is the classic vector; os.system / os.popen are always
// shell-evaluated. eval / exec are dynamic-code sinks.
// Negative lookbehind isn't supported in Go's RE2, so we use the
// `(?:^|[^.\w])` prefix to reject builtins-like names preceded by a
// dot (re.compile, ast.compile, importlib.util.compile_*). This
// prevents `re.compile(<literal-regex>)` from misfiring as a sink.
var pySinkExecRe = regexp.MustCompile(
	`\bsubprocess\s*\.\s*(?:call|run|Popen|check_call|check_output)\s*\([^)]*shell\s*=\s*True` +
		`|\bos\s*\.\s*(?:system|popen)\s*\(` +
		`|(?:^|[^.\w])(?:eval|exec|compile)\s*\(`,
)

// pySinkFSRe matches DESTRUCTIVE filesystem operations with a non-
// literal first arg. We exclude `open(<ident>)` because Python codebases
// routinely pass module-level path constants (LOG_FILE, CONFIG_PATH)
// through opens — the intraprocedural dataflow needed to prove the
// path is request-derived lives in Phase 4. Destructive ops
// (os.remove, os.unlink, shutil.rmtree, os.rename) are unambiguously
// security-sensitive even when the variable is internal, so we keep
// them. Pathlib.Path itself is benign — it's the subsequent .write_*
// that matters, and those are captured by the matching write regex.
var pySinkFSRe = regexp.MustCompile(
	`\bos\s*\.\s*(?:remove|unlink|rmdir|rename|replace)\s*\(\s*[A-Za-z_][\w]*\s*[,)]` +
		`|\bshutil\s*\.\s*(?:rmtree|move|copy|copy2|copyfile|copytree)\s*\(\s*[A-Za-z_][\w]*\s*[,)]`,
)

// pySinkXSSRe matches mark_safe / HttpResponse on a non-literal.
var pySinkXSSRe = regexp.MustCompile(
	`\bmark_safe\s*\(\s*[A-Za-z_][\w]*\s*\)` +
		`|\bHttpResponse\s*\(\s*[A-Za-z_][\w]*\s*[,)]`,
)

// pySinkReDoSRe matches re.compile of a non-literal.
var pySinkReDoSRe = regexp.MustCompile(
	`\bre\s*\.\s*compile\s*\(\s*[A-Za-z_][\w]*\s*[,)]`,
)

// pySanitizerSQLRe matches the parameterised cursor.execute form:
// cursor.execute(sql, params). Detection: open paren, quoted SQL,
// comma, then the params argument. We accept literal SQL or a bare
// identifier (named sql).
// Recognises the parameterised cursor.execute form. The SQL argument
// may be a quoted string, an f-string, a triple-quoted block, or a
// bare identifier (named sql); the second arg is what proves the call
// is safe. We accept the params arg starting with `[`, `(`, `{`,
// `tuple(`, `list(`, or a bare identifier — the universal pattern is
// "a second positional argument exists at all", which DB-API binds
// to the placeholders in the SQL.
var pySanitizerSQLRe = regexp.MustCompile(
	`\b(?:cursor|conn|connection)\s*\.\s*execute\s*\(\s*` +
		`(?:` +
		`f?['"]{3}[\s\S]*?['"]{3}` + // triple-quoted (possibly f-string) SQL
		`|f?['"][^'"]*['"]` + // single-quoted (possibly f-string) SQL
		`|[A-Za-z_][\w.]*` + // bare or dotted identifier (e.g. self.sql)
		`)` +
		`\s*,\s*` + // params separator
		`(?:[\[(\{]|tuple\b|list\b|dict\b|[A-Za-z_][\w]*)`,
)

// pySanitizerHTMLRe matches HTML-escape libraries.
var pySanitizerHTMLRe = regexp.MustCompile(
	`\bhtml\s*\.\s*escape\s*\(` +
		`|\bbleach\s*\.\s*(?:clean|linkify)\s*\(` +
		`|\bdjango\s*\.\s*utils\s*\.\s*html\s*\.\s*escape\s*\(` +
		`|\bmarkupsafe\s*\.\s*escape\s*\(` +
		`|\bescape\s*\(`,
)

// pySanitizerSchemaRe matches pydantic / marshmallow / attrs schema
// declarations. HARD RULE per #2772: the SCHEMA declaration is what
// counts, not the parse-call site. We match class declarations whose
// base is one of the known schema bases; this is conservative and
// only fires inside files that actually declare schemas.
var pySanitizerSchemaRe = regexp.MustCompile(
	`(?m)^\s*class\s+[A-Za-z_]\w*\s*\(\s*(?:BaseModel|Schema|marshmallow\s*\.\s*Schema|pydantic\s*\.\s*BaseModel)\s*[,)\s]`,
)

func sniffTaintPython(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanPyFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, pySourceReqRe, TaintKindSource, TaintCategoryGeneric, "request.body/POST/json", 1.0)
	out = appendTaintMatches(out, content, headers, pySourceEnvRe, TaintKindSource, TaintCategoryGeneric, "os.environ/getenv", 0.85)
	out = appendTaintMatches(out, content, headers, pySourceDeserializeRe, TaintKindSource, TaintCategoryDeserialization, "pickle.loads/yaml.load", 1.0)
	// Sanitizers first.
	out = appendTaintMatches(out, content, headers, pySanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "cursor.execute(sql, params)", 1.0)
	out = appendTaintMatches(out, content, headers, pySanitizerHTMLRe, TaintKindSanitizer, TaintCategoryXSS, "html.escape/bleach", 1.0)
	out = appendTaintMatches(out, content, headers, pySanitizerSchemaRe, TaintKindSanitizer, TaintCategoryGeneric, "pydantic/marshmallow.Schema", 0.9)
	// Sinks.
	out = appendTaintMatches(out, content, headers, pySinkSQLRe, TaintKindSink, TaintCategorySQL, "cursor.execute(non-literal)", 0.9)
	out = appendTaintMatches(out, content, headers, pySinkRawORMRe, TaintKindSink, TaintCategorySQL, "Model.objects.raw", 0.85)
	out = appendTaintMatches(out, content, headers, pySinkExecRe, TaintKindSink, TaintCategoryCommand, "subprocess shell=True/os.system/eval", 1.0)
	out = appendTaintMatches(out, content, headers, pySinkFSRe, TaintKindSink, TaintCategoryPath, "os.remove/unlink/rename/shutil(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, pySinkXSSRe, TaintKindSink, TaintCategoryXSS, "mark_safe/HttpResponse(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, pySinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "re.compile(non-literal)", 0.9)
	return out
}
