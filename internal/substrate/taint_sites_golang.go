// Go taint-sites sniffer (#2772 Phase 2B T1).
//
// Recognises Go source / sink / sanitizer primitives across the
// net/http stdlib, popular routers (gin, chi, echo, fiber), the
// database/sql package, and the os/exec command-runner.
//
// Sources:
//   - r.URL.Query() / r.Form / r.PostForm / r.MultipartForm
//   - r.Header.Get / r.Cookie
//   - r.Body (io.Reader of request body)
//   - gin: c.Query / c.Param / c.PostForm / c.GetHeader / c.GetRawData
//   - chi: chi.URLParam
//   - echo: c.QueryParam / c.Param / c.FormValue
//   - fiber: c.Query / c.Params / c.FormValue / c.Body
//   - os.Getenv
//   - json.Unmarshal of a non-literal byte slice
//
// Sinks:
//   - SQL injection: db.Query / Exec / QueryRow with a fmt.Sprintf or
//     concatenated string
//   - Command injection: exec.Command / exec.CommandContext with a
//     non-literal first arg (program name) or with `/bin/sh -c <var>`
//   - Path traversal: os.Open / os.ReadFile / os.Create /
//     os.WriteFile / filepath.Join with a non-literal segment
//   - XSS: w.Write of an unescaped []byte built from input;
//     template/html non-escaped use (text/template — html/template
//     auto-escapes)
//   - ReDoS: regexp.Compile / MustCompile with a non-literal
//   - SSRF: http.Get / http.Post with a non-literal URL
//
// Sanitizers:
//   - Parameterised SQL: db.Query / Exec with `?` or `$1`
//     placeholders AND a trailing parameter slice — pattern: the SQL
//     literal contains a placeholder character and the call has more
//     than one argument
//   - html.EscapeString / template.HTMLEscapeString
//   - go-playground/validator: a struct field carrying a `validate:`
//     tag counts as a schema declaration
package substrate

import "regexp"

func init() { RegisterTaintSniffer("go", sniffTaintGo) }

// goSourceHTTPReqRe matches the canonical net/http request access
// patterns. The leading `\b` keeps it from matching `xr.URL.Query`.
var goSourceHTTPReqRe = regexp.MustCompile(
	`\br\s*\.\s*(?:URL\s*\.\s*Query|Form|PostForm|MultipartForm|Header\s*\.\s*Get|Cookie|Body)\b` +
		`|\breq\s*\.\s*(?:URL\s*\.\s*Query|Form|PostForm|Body|Header\s*\.\s*Get)\b`,
)

// goSourceGinRe matches gin's context input methods.
var goSourceGinRe = regexp.MustCompile(
	`\bc\s*\.\s*(?:Query|QueryArray|Param|PostForm|GetHeader|GetRawData|ShouldBind(?:JSON|Query|Header)?|Bind(?:JSON|Query|Header)?)\s*\(`,
)

// goSourceChiEchoFiberRe matches chi/echo/fiber input getters.
var goSourceChiEchoFiberRe = regexp.MustCompile(
	`\bchi\s*\.\s*URLParam\s*\(` +
		`|\bc\s*\.\s*(?:QueryParam|FormValue|Params|Body)\s*\(`,
)

// goSourceEnvRe matches os.Getenv / os.LookupEnv.
var goSourceEnvRe = regexp.MustCompile(
	`\bos\s*\.\s*(?:Getenv|LookupEnv)\s*\(`,
)

// goSourceDeserialRe matches json.Unmarshal of a non-literal.
var goSourceDeserialRe = regexp.MustCompile(
	`\bjson\s*\.\s*Unmarshal\s*\(\s*[A-Za-z_][\w]*\s*,`,
)

// goSinkSQLRe matches db.Query/Exec/QueryRow when the first arg is a
// fmt.Sprintf call or a concatenated string. The parameterised case
// (`db.Query("SELECT ... WHERE id = ?", id)`) is detected as the
// sanitizer; here we deliberately require fmt.Sprintf or `+`.
var goSinkSQLRe = regexp.MustCompile(
	`\b(?:db|tx|stmt|conn)\s*\.\s*(?:Query|QueryContext|QueryRow|QueryRowContext|Exec|ExecContext)\s*\(\s*(?:fmt\s*\.\s*Sprintf|[A-Za-z_][\w]*\s*\+)`,
)

// goSinkExecRe matches exec.Command / CommandContext with a non-
// literal first arg or a shell-invocation pattern.
var goSinkExecRe = regexp.MustCompile(
	`\bexec\s*\.\s*(?:Command|CommandContext)\s*\(\s*(?:[A-Za-z_][\w]*\s*[,)]|"[^"]*sh"\s*,\s*"-c"\s*,\s*[A-Za-z_][\w]*\b)`,
)

// goSinkFSRe matches os.Open / ReadFile / Create / WriteFile when the
// path argument is a non-literal identifier. filepath.Join with a
// non-literal segment is included separately.
var goSinkFSRe = regexp.MustCompile(
	`\bos\s*\.\s*(?:Open|OpenFile|ReadFile|Create|WriteFile|Remove|RemoveAll)\s*\(\s*[A-Za-z_][\w]*\s*[,)]` +
		`|\bfilepath\s*\.\s*Join\s*\([^)]*[A-Za-z_][\w]*\s*\)`,
)

// goSinkXSSRe matches w.Write of a non-literal byte slice. Many false
// positives here (a typical response body write); we tag it lower
// confidence to let the propagation pass weight it accordingly.
var goSinkXSSRe = regexp.MustCompile(
	`\bw\s*\.\s*Write\s*\(\s*\[\]byte\s*\(\s*[A-Za-z_][\w]*\s*\)` +
		`|\btext/template\b` + // text/template never escapes
		`|\.Execute\s*\(\s*w\s*,\s*[A-Za-z_][\w]*\s*\)`,
)

// goSinkReDoSRe matches regexp.Compile / MustCompile with a
// non-literal pattern.
var goSinkReDoSRe = regexp.MustCompile(
	`\bregexp\s*\.\s*(?:Compile|MustCompile|CompilePOSIX|MustCompilePOSIX)\s*\(\s*[A-Za-z_][\w]*\s*[,)]`,
)

// goSinkSSRFRe matches outbound HTTP with a non-literal URL.
var goSinkSSRFRe = regexp.MustCompile(
	`\bhttp\s*\.\s*(?:Get|Post|Head|PostForm)\s*\(\s*[A-Za-z_][\w]*\s*[,)]`,
)

// goSanitizerSQLRe matches the parameterised-query convention: the
// SQL literal contains a placeholder (`?` or `$N`) AND there is at
// least one comma-separated arg after it. database/sql passes the
// remaining args as values bound to placeholders, which is safe.
var goSanitizerSQLRe = regexp.MustCompile(
	`\b(?:db|tx|stmt|conn)\s*\.\s*(?:Query|QueryContext|QueryRow|QueryRowContext|Exec|ExecContext)\s*\(\s*(?:ctx\s*,\s*)?"[^"]*(?:\?|\$[0-9]+)[^"]*"\s*,`,
)

// goSanitizerHTMLRe matches html.EscapeString /
// template.HTMLEscapeString.
var goSanitizerHTMLRe = regexp.MustCompile(
	`\bhtml\s*\.\s*EscapeString\s*\(` +
		`|\btemplate\s*\.\s*HTMLEscapeString\s*\(` +
		`|\bhtml/template\b`,
)

// goSanitizerValidateRe matches the go-playground/validator schema-
// declaration pattern: a struct field carrying a `validate:` tag.
// HARD RULE per #2772: bare validator.New().Struct(...) without a
// tag is not detectable as a sanitizer; we require the tag presence
// in the file to count the validator as installed.
var goSanitizerValidateRe = regexp.MustCompile(
	"`[^`]*\\bvalidate:\"[^\"]+\"[^`]*`",
)

func sniffTaintGo(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanGoFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, goSourceHTTPReqRe, TaintKindSource, TaintCategoryGeneric, "r.URL.Query/Form/Body", 1.0)
	out = appendTaintMatches(out, content, headers, goSourceGinRe, TaintKindSource, TaintCategoryGeneric, "gin.Context input", 0.95)
	out = appendTaintMatches(out, content, headers, goSourceChiEchoFiberRe, TaintKindSource, TaintCategoryGeneric, "chi/echo/fiber input", 0.95)
	out = appendTaintMatches(out, content, headers, goSourceEnvRe, TaintKindSource, TaintCategoryGeneric, "os.Getenv", 0.85)
	out = appendTaintMatches(out, content, headers, goSourceDeserialRe, TaintKindSource, TaintCategoryDeserialization, "json.Unmarshal(ident, ...)", 0.7)
	out = appendTaintMatches(out, content, headers, goSanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "db.Query(sql,?args)", 1.0)
	out = appendTaintMatches(out, content, headers, goSanitizerHTMLRe, TaintKindSanitizer, TaintCategoryXSS, "html.EscapeString/html/template", 1.0)
	out = appendTaintMatches(out, content, headers, goSanitizerValidateRe, TaintKindSanitizer, TaintCategoryGeneric, "struct `validate:` tag", 0.85)
	out = appendTaintMatches(out, content, headers, goSinkSQLRe, TaintKindSink, TaintCategorySQL, "db.Query(fmt.Sprintf|concat)", 0.9)
	out = appendTaintMatches(out, content, headers, goSinkExecRe, TaintKindSink, TaintCategoryCommand, "exec.Command(non-literal)", 1.0)
	out = appendTaintMatches(out, content, headers, goSinkFSRe, TaintKindSink, TaintCategoryPath, "os.Open/WriteFile(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, goSinkXSSRe, TaintKindSink, TaintCategoryXSS, "w.Write/text/template", 0.7)
	out = appendTaintMatches(out, content, headers, goSinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "regexp.Compile(non-literal)", 0.9)
	out = appendTaintMatches(out, content, headers, goSinkSSRFRe, TaintKindSink, TaintCategorySSRF, "http.Get(non-literal)", 0.85)
	return out
}
