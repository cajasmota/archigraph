// Java taint-sites sniffer (#2772 Phase 2B T1).
//
// Recognises Java source / sink / sanitizer primitives.
//
// Sources:
//   - Servlet: request.getParameter / getHeader / getCookies /
//     getReader / getInputStream / getQueryString
//   - Spring MVC: @RequestParam / @PathVariable / @RequestBody —
//     parameter-level annotations on handler methods
//   - System.getenv / System.getProperty
//   - ObjectInputStream.readObject (untrusted-input deserialisation)
//
// Sinks:
//   - SQL injection: Statement.execute / executeQuery / executeUpdate
//     with a concatenated string; raw JdbcTemplate.query(sql + ...)
//   - Command injection: Runtime.exec / ProcessBuilder.start with a
//     non-literal first arg
//   - Path traversal: new File(<non-literal>), Files.* with a Path
//     built from untrusted input
//   - XSS: response.getWriter().print(<non-literal>),
//     ResponseEntity.ok(<non-literal-html>)
//   - ReDoS: Pattern.compile(<non-literal>)
//   - Deserialisation: ObjectInputStream.readObject when the input
//     stream came from a request body
//
// Sanitizers:
//   - PreparedStatement.setX bound to a parameterised SQL string,
//     NamedParameterJdbcTemplate.queryForObject(sql, paramMap, ...)
//   - HTML escape: HtmlUtils.htmlEscape, Encode.forHtml,
//     ESAPI.encoder().encodeForHTML, OWASP Java Encoder
//   - Validation libs (schema-declaration required): @Valid /
//     @Validated on a handler arg whose type carries jakarta /
//     javax @NotNull|@Size|@Pattern annotations.
package substrate

import "regexp"

func init() { RegisterTaintSniffer("java", sniffTaintJava) }

// javaSourceServletRe matches the canonical servlet-API access shape.
var javaSourceServletRe = regexp.MustCompile(
	`\brequest\s*\.\s*(?:getParameter|getHeader|getHeaders|getCookies|getReader|getInputStream|getQueryString|getParameterValues|getParameterMap)\s*\(`,
)

// javaSourceSpringAnnotRe matches Spring MVC parameter annotations on
// method signatures. These are inputs by definition; the propagation
// pass marks the method's parameters as tainted.
var javaSourceSpringAnnotRe = regexp.MustCompile(
	`@(?:RequestParam|PathVariable|RequestBody|RequestHeader|CookieValue|RequestPart|ModelAttribute)\b`,
)

// javaSourceEnvRe matches System.getenv / System.getProperty.
var javaSourceEnvRe = regexp.MustCompile(
	`\bSystem\s*\.\s*(?:getenv|getProperty)\s*\(`,
)

// javaSourceDeserialRe matches ObjectInputStream.readObject — a
// well-known unrestricted-deserialisation primitive.
var javaSourceDeserialRe = regexp.MustCompile(
	`\.\s*readObject\s*\(\s*\)` +
		`|\bnew\s+ObjectInputStream\s*\(`,
)

// javaSinkSQLRe matches raw Statement / JdbcTemplate execution with a
// concatenated SQL string (`+`) or a non-literal first argument.
// The PreparedStatement path is excluded by requiring the receiver to
// be `statement|stmt|jdbcTemplate` and the first arg to contain a `+`
// or be a bare identifier.
var javaSinkSQLRe = regexp.MustCompile(
	`\b(?:statement|stmt|jdbcTemplate|connection)\s*\.\s*(?:execute|executeQuery|executeUpdate|query|queryForObject|queryForList|update)\s*\(\s*(?:[A-Za-z_$][\w$]*\s*\+|[A-Za-z_$][\w$]*\s*\))`,
)

// javaSinkExecRe matches Runtime.exec / ProcessBuilder.start. Always
// shell-evaluated on the underlying OS; tainted input is RCE.
var javaSinkExecRe = regexp.MustCompile(
	`\bRuntime\s*\.\s*getRuntime\s*\(\s*\)\s*\.\s*exec\s*\(` +
		`|\bnew\s+ProcessBuilder\s*\(`,
)

// javaSinkFSRe matches `new File(<non-literal>)` and Files.* with a
// path built from a non-literal.
var javaSinkFSRe = regexp.MustCompile(
	`\bnew\s+File\s*\(\s*[A-Za-z_$][\w$]*\s*[,)]` +
		`|\bFiles\s*\.\s*(?:readAllBytes|readString|write|writeString|newInputStream|newOutputStream|delete|deleteIfExists)\s*\(\s*[A-Za-z_$][\w$]*\b`,
)

// javaSinkXSSRe matches response.getWriter() output of a non-literal.
var javaSinkXSSRe = regexp.MustCompile(
	`\bresponse\s*\.\s*getWriter\s*\(\s*\)\s*\.\s*(?:print|println|write)\s*\(\s*[A-Za-z_$][\w$]*\s*\)`,
)

// javaSinkReDoSRe matches Pattern.compile(<non-literal>).
var javaSinkReDoSRe = regexp.MustCompile(
	`\bPattern\s*\.\s*compile\s*\(\s*[A-Za-z_$][\w$]*\s*[,)]`,
)

// javaSanitizerSQLRe matches PreparedStatement creation +
// NamedParameterJdbcTemplate with a paramMap. These represent
// parameterised execution and are the standard Java SQL safety
// pattern.
var javaSanitizerSQLRe = regexp.MustCompile(
	`\.\s*prepareStatement\s*\(` +
		`|\bnamedParameterJdbcTemplate\s*\.\s*(?:queryForObject|queryForList|query|update)\s*\(` +
		`|\bNamedParameterJdbcTemplate\b`,
)

// javaSanitizerHTMLRe matches HTML-escape libraries.
var javaSanitizerHTMLRe = regexp.MustCompile(
	`\bHtmlUtils\s*\.\s*htmlEscape\s*\(` +
		`|\bEncode\s*\.\s*forHtml(?:Content|Attribute)?\s*\(` +
		`|\bESAPI\s*\.\s*encoder\s*\(\s*\)\s*\.\s*encodeForHTML\s*\(`,
)

// javaSanitizerSchemaRe matches @Valid / @Validated on a handler
// parameter. HARD RULE per #2772: the annotation MUST appear next to
// a parameter that itself carries validation constraints (@NotNull,
// @Size, etc.) — the propagation pass enforces the constraint-presence
// check by counting @Valid only when paired with a parameter type
// referenced as a record / class with jakarta-validation annotations
// in the same file. The sniffer emits the @Valid match; binding
// happens in the pass.
var javaSanitizerSchemaRe = regexp.MustCompile(
	`@(?:Valid|Validated)\b`,
)

func sniffTaintJava(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanJavaFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, javaSourceServletRe, TaintKindSource, TaintCategoryGeneric, "request.getParameter/Header", 1.0)
	out = appendTaintMatches(out, content, headers, javaSourceSpringAnnotRe, TaintKindSource, TaintCategoryGeneric, "@RequestParam/@PathVariable/@RequestBody", 0.95)
	out = appendTaintMatches(out, content, headers, javaSourceEnvRe, TaintKindSource, TaintCategoryGeneric, "System.getenv/getProperty", 0.85)
	out = appendTaintMatches(out, content, headers, javaSourceDeserialRe, TaintKindSource, TaintCategoryDeserialization, "ObjectInputStream.readObject", 1.0)
	out = appendTaintMatches(out, content, headers, javaSanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "PreparedStatement/NamedParameterJdbcTemplate", 1.0)
	out = appendTaintMatches(out, content, headers, javaSanitizerHTMLRe, TaintKindSanitizer, TaintCategoryXSS, "HtmlUtils.htmlEscape/Encode.forHtml/ESAPI", 1.0)
	out = appendTaintMatches(out, content, headers, javaSanitizerSchemaRe, TaintKindSanitizer, TaintCategoryGeneric, "@Valid/@Validated", 0.85)
	out = appendTaintMatches(out, content, headers, javaSinkSQLRe, TaintKindSink, TaintCategorySQL, "Statement.execute(non-literal)", 0.9)
	out = appendTaintMatches(out, content, headers, javaSinkExecRe, TaintKindSink, TaintCategoryCommand, "Runtime.exec/ProcessBuilder", 1.0)
	out = appendTaintMatches(out, content, headers, javaSinkFSRe, TaintKindSink, TaintCategoryPath, "new File(non-literal)/Files.*", 0.85)
	out = appendTaintMatches(out, content, headers, javaSinkXSSRe, TaintKindSink, TaintCategoryXSS, "response.getWriter().print(non-literal)", 0.9)
	out = appendTaintMatches(out, content, headers, javaSinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "Pattern.compile(non-literal)", 0.9)
	return out
}
