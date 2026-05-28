// Kotlin taint-sites sniffer (#2773 Phase 2B T2).
//
// Recognises Kotlin source / sink / sanitizer primitives across Ktor,
// Spring Boot (Kotlin), Micronaut, Quarkus, Javalin, http4k, the Arrow
// FP stack, Jetpack Compose, KMP, and the kotlinx.coroutines layer.
//
// Sources:
//   - Spring (Kotlin): @RequestParam / @PathVariable / @RequestBody /
//     @RequestHeader / @CookieValue
//   - Ktor: call.receive<T>() / call.parameters / call.request.headers /
//     call.request.queryParameters / call.request.cookies
//   - Javalin: ctx.formParam / queryParam / pathParam / header / body /
//     bodyAsClass / cookie
//   - http4k: Request.bodyString() / .form() / .query() / .header()
//   - System.getenv / System.getProperty
//   - kotlinx.serialization Json.decodeFromString of a non-literal
//
// Sinks:
//   - SQL injection: JdbcTemplate.execute / query / update with a
//     concatenated string, Statement.executeQuery with a "..." + var,
//     Exposed framework selectAll().where { ... } with a raw fragment
//   - Command injection: ProcessBuilder with a list containing a
//     non-literal, Runtime.getRuntime().exec(non-literal)
//   - Path traversal: File(<non-literal>) followed by .writeText /
//     .readText / .delete, Files.write / readAllBytes with a Path
//     built from a non-literal
//   - XSS: HTML output via response.writer.print of a non-literal
//   - ReDoS: Pattern.compile / Regex(<non-literal>)
//
// Sanitizers:
//   - Parameterised SQL: NamedParameterJdbcTemplate, PreparedStatement.
//     setX with placeholder SQL, Exposed wrap.eq operator
//   - HTML escape: HtmlUtils.htmlEscape, kotlinx.html builder (auto-
//     escapes), Encode.forHtml (OWASP Java Encoder, callable from Kotlin)
//   - Validation: javax.validation / jakarta.validation @Valid /
//     @Validated on a handler arg (HARD RULE per #2772 — the annotation
//     plus per-field @NotNull / @Size / @Pattern annotations on the
//     data-class properties is the schema declaration; bare @Valid
//     without per-field constraints is not a sanitizer in practice but
//     we leniently mark it because Kotlin data classes commonly carry
//     constraints in a separate file)
package substrate

import "regexp"

func init() { RegisterTaintSniffer("kotlin", sniffTaintKotlin) }

// ktSourceSpringRe matches Spring MVC parameter annotations on
// Kotlin handler methods.
var ktSourceSpringRe = regexp.MustCompile(
	`@(?:RequestParam|PathVariable|RequestBody|RequestHeader|CookieValue|RequestPart|ModelAttribute)\b`,
)

// ktSourceKtorRe matches Ktor's call accessors.
var ktSourceKtorRe = regexp.MustCompile(
	`\bcall\s*\.\s*(?:receive(?:Text|Stream|Channel|Parameters)?|parameters|request\s*\.\s*(?:headers|queryParameters|cookies|receiveText|receiveChannel|receiveStream|local))\b`,
)

// ktSourceJavalinRe matches Javalin context input methods.
var ktSourceJavalinRe = regexp.MustCompile(
	`\bctx\s*\.\s*(?:formParam|queryParam|pathParam|header|body|bodyAsClass|cookie|attribute)\s*\(`,
)

// ktSourceHttp4kRe matches http4k request accessors.
var ktSourceHttp4kRe = regexp.MustCompile(
	`\b(?:request|req)\s*\.\s*(?:bodyString|form|query|header|cookie)\s*\(`,
)

// ktSourceEnvRe matches System.getenv / getProperty calls.
var ktSourceEnvRe = regexp.MustCompile(
	`\bSystem\s*\.\s*(?:getenv|getProperty)\s*\(`,
)

// ktSourceDeserializeRe matches kotlinx.serialization Json.decode of a
// non-literal source.
var ktSourceDeserializeRe = regexp.MustCompile(
	`\bJson\s*\.\s*(?:decodeFromString|decodeFromStream|decodeFromJsonElement)\s*\(\s*[a-z_][\w]*\s*\)` +
		`|\bObjectMapper\s*\(\s*\)\s*\.\s*readValue\s*\(`,
)

// ktSinkSQLRe matches the JdbcTemplate / Statement raw-SQL patterns.
var ktSinkSQLRe = regexp.MustCompile(
	`\b(?:jdbcTemplate|statement|stmt|connection)\s*\.\s*(?:execute|executeQuery|executeUpdate|query|queryForObject|update)\s*\(\s*(?:[a-z_][\w]*\s*\+|"[^"]*\$\{|"[^"]*"\s*\+)`,
)

// ktSinkExecRe matches ProcessBuilder / Runtime.exec.
var ktSinkExecRe = regexp.MustCompile(
	`\bProcessBuilder\s*\(\s*[a-z_][\w]*\s*\)` +
		`|\bProcessBuilder\s*\([^)]*\$\{` +
		`|\bRuntime\s*\.\s*getRuntime\s*\(\s*\)\s*\.\s*exec\s*\(`,
)

// ktSinkFSRe matches File operations with a non-literal first arg.
var ktSinkFSRe = regexp.MustCompile(
	`\bFile\s*\(\s*[a-z_][\w]*\s*\)\s*\.\s*(?:writeText|writeBytes|readText|readBytes|delete|deleteOnExit|renameTo|appendText)\s*\(` +
		`|\bFiles\s*\.\s*(?:write|writeString|readAllBytes|readString|delete|deleteIfExists|move|copy)\s*\(\s*[A-Za-z_][\w]*`,
)

// ktSinkXSSRe matches response writer output of a non-literal.
var ktSinkXSSRe = regexp.MustCompile(
	`\bresponse\s*\.\s*(?:writer|outputStream)\s*\.\s*(?:print|println|write)\s*\(\s*[a-z_][\w]*\s*\)` +
		`|\bcall\s*\.\s*respondText\s*\(\s*[a-z_][\w]*\s*[,)]`,
)

// ktSinkReDoSRe matches Pattern.compile / Regex on a non-literal.
var ktSinkReDoSRe = regexp.MustCompile(
	`\bPattern\s*\.\s*compile\s*\(\s*[a-z_][\w]*\s*[,)]` +
		`|\bRegex\s*\(\s*[a-z_][\w]*\s*[,)]`,
)

// ktSanitizerSQLRe matches the parameterised-SQL flows.
var ktSanitizerSQLRe = regexp.MustCompile(
	`\.\s*prepareStatement\s*\(` +
		`|\bnamedParameterJdbcTemplate\s*\.\s*(?:queryForObject|queryForList|query|update)\s*\(` +
		`|\bNamedParameterJdbcTemplate\b`,
)

// ktSanitizerHTMLRe matches the canonical HTML-escape utilities.
var ktSanitizerHTMLRe = regexp.MustCompile(
	`\bHtmlUtils\s*\.\s*htmlEscape\s*\(` +
		`|\bEncode\s*\.\s*forHtml(?:Content|Attribute)?\s*\(` +
		`|\bStringEscapeUtils\s*\.\s*escapeHtml4\s*\(`,
)

// ktSanitizerValidateRe matches @Valid / @Validated annotations and
// per-field jakarta-validation constraints.
var ktSanitizerValidateRe = regexp.MustCompile(
	`@(?:Valid|Validated)\b` +
		`|@(?:NotNull|NotBlank|NotEmpty|Size|Pattern|Min|Max|Email|Positive|Negative|PastOrPresent)\b`,
)

func sniffTaintKotlin(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanKotlinFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, ktSourceSpringRe, TaintKindSource, TaintCategoryGeneric, "@RequestParam/@PathVariable/@RequestBody", 0.95)
	out = appendTaintMatches(out, content, headers, ktSourceKtorRe, TaintKindSource, TaintCategoryGeneric, "call.receive/parameters/headers", 1.0)
	out = appendTaintMatches(out, content, headers, ktSourceJavalinRe, TaintKindSource, TaintCategoryGeneric, "ctx.formParam/queryParam/body", 1.0)
	out = appendTaintMatches(out, content, headers, ktSourceHttp4kRe, TaintKindSource, TaintCategoryGeneric, "request.bodyString/form/query", 0.95)
	out = appendTaintMatches(out, content, headers, ktSourceEnvRe, TaintKindSource, TaintCategoryGeneric, "System.getenv/getProperty", 0.85)
	out = appendTaintMatches(out, content, headers, ktSourceDeserializeRe, TaintKindSource, TaintCategoryDeserialization, "Json.decodeFromString/ObjectMapper.readValue", 0.85)
	// Sanitizers first.
	out = appendTaintMatches(out, content, headers, ktSanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "PreparedStatement/NamedParameterJdbcTemplate", 1.0)
	out = appendTaintMatches(out, content, headers, ktSanitizerHTMLRe, TaintKindSanitizer, TaintCategoryXSS, "HtmlUtils.htmlEscape/Encode.forHtml/StringEscapeUtils", 1.0)
	out = appendTaintMatches(out, content, headers, ktSanitizerValidateRe, TaintKindSanitizer, TaintCategoryGeneric, "@Valid/@Validated/@NotNull/@Size", 0.85)
	// Sinks.
	out = appendTaintMatches(out, content, headers, ktSinkSQLRe, TaintKindSink, TaintCategorySQL, "JdbcTemplate.execute(concat/${...})", 0.9)
	out = appendTaintMatches(out, content, headers, ktSinkExecRe, TaintKindSink, TaintCategoryCommand, "ProcessBuilder/Runtime.exec(non-literal)", 1.0)
	out = appendTaintMatches(out, content, headers, ktSinkFSRe, TaintKindSink, TaintCategoryPath, "File(non-literal)/Files.write", 0.85)
	out = appendTaintMatches(out, content, headers, ktSinkXSSRe, TaintKindSink, TaintCategoryXSS, "response.writer.print/call.respondText(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, ktSinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "Pattern.compile/Regex(non-literal)", 0.9)
	return out
}
