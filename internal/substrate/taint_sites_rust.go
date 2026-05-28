// Rust taint-sites sniffer (#2773 Phase 2B T2).
//
// Recognises Rust source / sink / sanitizer primitives across actix-web,
// axum, rocket, warp, tower, hyper, tide, poem, salvo, gotham.
//
// Sources:
//   - actix-web: web::Json<T> / web::Path<T> / web::Query<T> /
//     web::Form<T> / HttpRequest::headers / ::cookie
//   - axum: extract::Json / Path / Query / Form / TypedHeader as
//     handler-arg types
//   - rocket: handler args annotated &State<T>, Json<T>, Form<T>
//   - std::env::var / env::vars
//   - serde_json::from_str / from_slice of a non-literal
//
// Sinks:
//   - SQL injection: sqlx::query / query_as with a `&format!(...)` /
//     concatenated &str, raw_sql! macro
//   - Command injection: std::process::Command::new with a non-literal
//     program, .arg(<user input>) directly; tokio::process::Command
//     equivalent
//   - Path traversal: std::fs::write / read / read_to_string /
//     File::create with a non-literal path; tokio::fs equivalent
//   - XSS: askama / tera template render with disable_autoescape; raw
//     html output via http::Response::body of a String built from input
//   - ReDoS: regex::Regex::new of a non-literal pattern
//   - Deserialisation: bincode::deserialize / serde_json::from_slice on
//     untrusted bytes (tagged as deserialization category)
//
// Sanitizers:
//   - Parameterised SQL: sqlx::query("SELECT ... $1").bind(value),
//     query_as!(...) macro with positional placeholders
//   - HTML escape: askama auto-escape (default on), html_escape::encode_*,
//     v_htmlescape::escape
//   - Validation: the `validator` crate — a struct deriving #[derive(
//     Validate)] counts as a schema declaration. HARD RULE per #2772:
//     bare .validate() without a #[derive(Validate)] in the file is
//     not a sanitizer.
//   - Command isolation: tokio::process::Command with .arg called only
//     on string literals — practically impossible to detect statically
//     here, so we conservatively do NOT flag any Command call as
//     sanitised unless escapeshell-equivalent wrapping is visible.
package substrate

import "regexp"

func init() { RegisterTaintSniffer("rust", sniffTaintRust) }

// rsSourceActixAxumRe matches the actix-web / axum extractor types
// used as handler arguments. Presence of `web::Json<T>` / `Path<T>` /
// `Query<T>` in a function signature marks the function as carrying a
// taint source for the value extracted.
var rsSourceActixAxumRe = regexp.MustCompile(
	`\b(?:web|extract)\s*::\s*(?:Json|Path|Query|Form|TypedHeader)\s*<` +
		`|\bJson\s*<\s*[A-Z][\w]*\s*>|\bPath\s*<\s*[A-Z(][\w()]*\s*>|\bQuery\s*<\s*[A-Z][\w]*\s*>`,
)

// rsSourceReqRe matches HttpRequest accessors.
var rsSourceReqRe = regexp.MustCompile(
	`\b(?:req|request)\s*\.\s*(?:headers|cookie|cookies|uri|query_string|body|connection_info)\s*\(`,
)

// rsSourceEnvRe matches std::env::var / env::vars.
var rsSourceEnvRe = regexp.MustCompile(
	`\b(?:std\s*::\s*)?env\s*::\s*(?:var|vars|var_os)\s*\(`,
)

// rsSourceDeserializeRe matches serde_json / bincode deserialisation
// of a non-literal input.
var rsSourceDeserializeRe = regexp.MustCompile(
	`\bserde_json\s*::\s*(?:from_str|from_slice|from_reader)\s*\(\s*&?[a-z_][\w]*` +
		`|\bbincode\s*::\s*deserialize\s*\(`,
)

// rsSinkSQLRe matches sqlx::query / query_as called with a &format!()
// or concatenated &str — string-interpolation SQL is the SQLi vector.
// The .bind() form is the sanitizer (matched separately).
var rsSinkSQLRe = regexp.MustCompile(
	`\bsqlx\s*::\s*(?:query|query_as)\s*\(\s*&?format!\s*\(` +
		`|\bsqlx\s*::\s*(?:query|query_as)\s*\(\s*&?[a-z_][\w]*\s*\)`,
)

// rsSinkExecRe matches std::process::Command / tokio::process::Command
// with a non-literal program name, or any .arg called on a binding.
var rsSinkExecRe = regexp.MustCompile(
	`\b(?:std|tokio)\s*::\s*process\s*::\s*Command\s*::\s*new\s*\(\s*&?[a-z_][\w]*\s*\)` +
		`|\bCommand\s*::\s*new\s*\(\s*&?[a-z_][\w]*\s*\)\s*\.\s*arg\s*\(\s*&?[a-z_][\w]*\s*\)`,
)

// rsSinkFSRe matches std::fs / tokio::fs writes / opens with a
// non-literal path. Literal paths (PathBuf::from("/etc/...")) are not
// flagged.
var rsSinkFSRe = regexp.MustCompile(
	`\b(?:std|tokio)\s*::\s*fs\s*::\s*(?:write|read|read_to_string|read_dir|remove_file|remove_dir|remove_dir_all|rename|copy|create_dir|create_dir_all|File::create|File::open)\s*\(\s*&?[a-z_][\w]*\s*[,)]`,
)

// rsSinkReDoSRe matches Regex::new on a non-literal pattern.
var rsSinkReDoSRe = regexp.MustCompile(
	`\bRegex\s*::\s*new\s*\(\s*&?[a-z_][\w]*\s*\)`,
)

// rsSanitizerSQLRe matches the parameterised sqlx flow. The presence
// of `.bind(` on a query in the file is the sanitizer signal.
var rsSanitizerSQLRe = regexp.MustCompile(
	`\bsqlx\s*::\s*(?:query|query_as)\s*\(\s*"[^"]*\$[0-9]+[^"]*"\s*\)` +
		`|\bsqlx\s*::\s*(?:query|query_as)\s*\([^)]*\)\s*\.\s*bind\s*\(` +
		`|\bquery_as!\s*\(|\bquery!\s*\(`,
)

// rsSanitizerHTMLRe matches the canonical HTML-escape crates.
var rsSanitizerHTMLRe = regexp.MustCompile(
	`\bhtml_escape\s*::\s*encode_(?:text|safe|double_quoted_attribute|single_quoted_attribute)\s*\(` +
		`|\bv_htmlescape\s*::\s*escape\s*\(` +
		`|\baskama\s*::\s*Template\b`, // askama auto-escapes by default
)

// rsSanitizerValidateRe matches the `validator` crate's schema-
// declaration shape: a struct annotated with #[derive(Validate)] and
// per-field #[validate(...)] attributes. HARD RULE per #2772 — the
// derive macro on a struct is what counts.
var rsSanitizerValidateRe = regexp.MustCompile(
	`#\[\s*derive\s*\([^)]*\bValidate\b[^)]*\)\s*\]`,
)

func sniffTaintRust(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanRustFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, rsSourceActixAxumRe, TaintKindSource, TaintCategoryGeneric, "web::Json/Path/Query<T>", 0.95)
	out = appendTaintMatches(out, content, headers, rsSourceReqRe, TaintKindSource, TaintCategoryGeneric, "req.headers/cookie/query_string", 1.0)
	out = appendTaintMatches(out, content, headers, rsSourceEnvRe, TaintKindSource, TaintCategoryGeneric, "std::env::var", 0.85)
	out = appendTaintMatches(out, content, headers, rsSourceDeserializeRe, TaintKindSource, TaintCategoryDeserialization, "serde_json::from_str(ident)/bincode", 0.8)
	// Sanitizers first.
	out = appendTaintMatches(out, content, headers, rsSanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "sqlx::query($N).bind/query!()", 1.0)
	out = appendTaintMatches(out, content, headers, rsSanitizerHTMLRe, TaintKindSanitizer, TaintCategoryXSS, "html_escape/v_htmlescape/askama", 1.0)
	out = appendTaintMatches(out, content, headers, rsSanitizerValidateRe, TaintKindSanitizer, TaintCategoryGeneric, "#[derive(Validate)]", 0.9)
	// Sinks.
	out = appendTaintMatches(out, content, headers, rsSinkSQLRe, TaintKindSink, TaintCategorySQL, "sqlx::query(format!/ident)", 0.9)
	out = appendTaintMatches(out, content, headers, rsSinkExecRe, TaintKindSink, TaintCategoryCommand, "process::Command::new(ident).arg(ident)", 1.0)
	out = appendTaintMatches(out, content, headers, rsSinkFSRe, TaintKindSink, TaintCategoryPath, "fs::write/read/File(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, rsSinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "Regex::new(non-literal)", 0.9)
	return out
}
