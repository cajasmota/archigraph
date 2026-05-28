// Scala taint-sites sniffer (#2773 Phase 2B T2).
//
// Recognises Scala source / sink / sanitizer primitives across Play
// Framework, Akka HTTP, http4s, Finatra, Lagom, Cask, Scalatra,
// ZIO-HTTP, and Cats Effect.
//
// Sources:
//   - Play: request.body / request.queryString / request.headers /
//     request.cookies, action(parse.json) / parse.urlFormEncoded
//     extractors
//   - Akka HTTP: extract(...) directives — extractRequest,
//     extractRequestEntity, entity(as[T]), parameter, headerValueByName
//   - http4s: req.bodyAsText, req.params, req.headers, req.cookies
//   - sys.env / sys.props
//   - upickle / circe / play-json decoding of a non-literal payload
//
// Sinks:
//   - SQL injection: Slick sql"#${var}" raw-interpolation (the `#${...}`
//     splice bypasses parameterisation), plain JDBC Statement.execute
//     with a concatenated string
//   - Command injection: sys.process.Process / scala.sys.process."!"
//     with a non-literal command, ProcessBuilder
//   - Path traversal: java.io.File / java.nio.file.Files / Source.
//     fromFile with a non-literal path; os-lib os.read / os.write with
//     a Path built from input
//   - XSS: Twirl @Html(...) of a non-literal, raw output via Ok(...)
//     with a String body and Content-Type text/html
//   - ReDoS: scala.util.matching.Regex from a non-literal pattern
//
// Sanitizers:
//   - Parameterised SQL: Slick sql"..." with `${var}` interpolation —
//     the non-# interpolation IS parameterised by Slick. sqlu"...$var"
//     equivalent.
//   - HTML escape: Play HtmlFormat.escape, Twirl auto-escape (default
//     in @{} blocks), scala.xml.Utility.escape
//   - Validation: Play Form binding (Form(mapping(...))) with field-
//     level Constraints — the mapping declaration is the schema (HARD
//     RULE per #2772: a Form / mapping declaration counts; a bare
//     bindFromRequest without a typed mapping does not)
package substrate

import "regexp"

func init() { RegisterTaintSniffer("scala", sniffTaintScala) }

// scSourcePlayRe matches Play Framework request access.
var scSourcePlayRe = regexp.MustCompile(
	`\b(?:request|req)\s*\.\s*(?:body|queryString|headers|cookies|rawQueryString|method|path|getQueryString|target)\b` +
		`|\bparse\s*\.\s*(?:json|urlFormEncoded|text|raw|temporaryFile|multipartFormData|tolerantJson|tolerantText|tolerantXml|byteString|xml|anyContent)\b`,
)

// scSourceAkkaRe matches Akka HTTP directives that extract input.
var scSourceAkkaRe = regexp.MustCompile(
	`\b(?:extractRequest|extractRequestEntity|extractUri|entity\s*\(\s*as\[|parameter\s*\(|parameters\s*\(|headerValueByName\s*\(|formField\s*\(|formFields\s*\(|path\s*\(\s*Segment|cookie\s*\()`,
)

// scSourceHttp4sRe matches http4s request accessors.
var scSourceHttp4sRe = regexp.MustCompile(
	`\b(?:req|request)\s*\.\s*(?:bodyAsText|bodyText|params|multiParams|headers|cookies|uri)\b`,
)

// scSourceEnvRe matches sys.env / sys.props.
var scSourceEnvRe = regexp.MustCompile(
	`\bsys\s*\.\s*(?:env|props)\s*(?:\.\s*(?:get|getOrElse|apply)|\(|\[)` +
		`|\bSystem\s*\.\s*(?:getenv|getProperty)\s*\(`,
)

// scSourceDeserializeRe matches JSON / pickle decoding of a non-literal.
var scSourceDeserializeRe = regexp.MustCompile(
	`\bupickle\s*\.\s*default\s*\.\s*read\s*\(\s*[a-z_][\w]*` +
		`|\bdecode\s*\[\s*[A-Z][\w]*\s*\]\s*\(\s*[a-z_][\w]*\s*\)` +
		`|\bJson\s*\.\s*parse\s*\(\s*[a-z_][\w]*\s*\)`,
)

// scSinkSQLRe matches the unsafe Slick `#${...}` splice form and
// JDBC string-concat exec.
var scSinkSQLRe = regexp.MustCompile(
	`\bsql"{1,3}[\s\S]*?#\$\{` +
		`|\bsqlu"{1,3}[\s\S]*?#\$\{` +
		`|\b(?:statement|stmt|connection)\s*\.\s*(?:execute|executeQuery|executeUpdate)\s*\(\s*[a-z_][\w]*\s*\+`,
)

// scSinkExecRe matches scala.sys.process / java ProcessBuilder.
var scSinkExecRe = regexp.MustCompile(
	`\b(?:scala\s*\.\s*)?sys\s*\.\s*process\s*\.\s*Process\s*\(\s*[a-z_][\w]*\s*[,)]` +
		"|\\bProcess\\(`?[a-z_][\\w]*`?\\)\\s*\\.!" +
		`|\bnew\s+ProcessBuilder\s*\(\s*[a-z_][\w]*`,
)

// scSinkFSRe matches File / Files / Source / os-lib operations with a
// non-literal path.
var scSinkFSRe = regexp.MustCompile(
	`\bnew\s+java\.io\.File\s*\(\s*[a-z_][\w]*\s*[,)]` +
		`|\bnew\s+File\s*\(\s*[a-z_][\w]*\s*[,)]` +
		`|\bFiles\s*\.\s*(?:write|writeString|readAllBytes|readString|delete|deleteIfExists|move|copy)\s*\(\s*[A-Za-z_][\w]*` +
		`|\bSource\s*\.\s*fromFile\s*\(\s*[a-z_][\w]*\s*[,)]` +
		`|\bos\s*\.\s*(?:read|write|remove|move|copy)\s*\(\s*[a-z_][\w]*`,
)

// scSinkXSSRe matches Twirl @Html on a non-literal.
var scSinkXSSRe = regexp.MustCompile(
	`@Html\s*\(\s*[a-z_][\w]*\s*\)` +
		`|\bHtml\s*\(\s*[a-z_][\w]*\s*\)`,
)

// scSinkReDoSRe matches Regex / Pattern construction from a non-literal.
var scSinkReDoSRe = regexp.MustCompile(
	`\b[a-z_][\w]*\s*\.\s*r\b` + // String.r idiom — only flag when r is called on identifier
		`|\bPattern\s*\.\s*compile\s*\(\s*[a-z_][\w]*\s*[,)]` +
		`|\bnew\s+Regex\s*\(\s*[a-z_][\w]*\s*[,)]`,
)

// scSanitizerSQLRe matches the Slick parameterised forms (sql"...${var}"
// without the `#` splice) and the Slick query DSL (TableQuery / for-
// comprehensions).
var scSanitizerSQLRe = regexp.MustCompile(
	`\bsql"{1,3}[\s\S]*?(?:[^#])\$\{[a-z_]` +
		`|\bsqlu"{1,3}[\s\S]*?(?:[^#])\$\{[a-z_]` +
		`|\bTableQuery\s*\[\s*[A-Z][\w]*\s*\]` +
		`|\bquoted\s*\{`,
)

// scSanitizerHTMLRe matches Play / Twirl / xml HTML-escape utilities.
var scSanitizerHTMLRe = regexp.MustCompile(
	`\bHtmlFormat\s*\.\s*escape\s*\(` +
		`|\bscala\s*\.\s*xml\s*\.\s*Utility\s*\.\s*escape\s*\(` +
		`|\bStringEscapeUtils\s*\.\s*escapeHtml4\s*\(`,
)

// scSanitizerFormRe matches Play Form mapping declarations. HARD
// RULE per #2772: the mapping(...) declaration with typed field
// constraints is the schema; bare bindFromRequest is not.
var scSanitizerFormRe = regexp.MustCompile(
	`\bForm\s*\(\s*mapping\s*\(` +
		`|\bmapping\s*\([^)]*->\s*(?:nonEmptyText|text|number|email|longNumber|boolean|date)\b`,
)

func sniffTaintScala(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanScalaFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, scSourcePlayRe, TaintKindSource, TaintCategoryGeneric, "request.body/queryString/headers", 1.0)
	out = appendTaintMatches(out, content, headers, scSourceAkkaRe, TaintKindSource, TaintCategoryGeneric, "extractRequest/entity(as[T])/parameter", 0.95)
	out = appendTaintMatches(out, content, headers, scSourceHttp4sRe, TaintKindSource, TaintCategoryGeneric, "req.bodyAsText/params/headers", 0.95)
	out = appendTaintMatches(out, content, headers, scSourceEnvRe, TaintKindSource, TaintCategoryGeneric, "sys.env/sys.props", 0.85)
	out = appendTaintMatches(out, content, headers, scSourceDeserializeRe, TaintKindSource, TaintCategoryDeserialization, "upickle/circe.decode/Json.parse(ident)", 0.8)
	// Sanitizers first.
	out = appendTaintMatches(out, content, headers, scSanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "sql\"${v}\"/TableQuery/quoted", 1.0)
	out = appendTaintMatches(out, content, headers, scSanitizerHTMLRe, TaintKindSanitizer, TaintCategoryXSS, "HtmlFormat.escape/scala.xml.Utility.escape", 1.0)
	out = appendTaintMatches(out, content, headers, scSanitizerFormRe, TaintKindSanitizer, TaintCategoryGeneric, "Form(mapping(...))", 0.9)
	// Sinks.
	out = appendTaintMatches(out, content, headers, scSinkSQLRe, TaintKindSink, TaintCategorySQL, "sql\"#${v}\"/Statement.execute(concat)", 0.95)
	out = appendTaintMatches(out, content, headers, scSinkExecRe, TaintKindSink, TaintCategoryCommand, "sys.process.Process(non-literal)/ProcessBuilder", 1.0)
	out = appendTaintMatches(out, content, headers, scSinkFSRe, TaintKindSink, TaintCategoryPath, "new File/Files.write/Source.fromFile/os.read(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, scSinkXSSRe, TaintKindSink, TaintCategoryXSS, "@Html(non-literal)", 0.9)
	out = appendTaintMatches(out, content, headers, scSinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "ident.r/Pattern.compile/new Regex(non-literal)", 0.85)
	return out
}
