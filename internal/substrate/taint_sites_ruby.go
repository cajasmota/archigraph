// Ruby taint-sites sniffer (#2773 Phase 2B T2).
//
// Recognises Ruby source / sink / sanitizer primitives across Rails,
// Sinatra, Hanami, Grape, Roda, Cuba, Padrino, and dry-rb.
//
// Sources:
//   - params / params[:name] / request.params (Rails / Sinatra / Grape)
//   - request.body.read / request.raw_post
//   - request.headers[...] / request.cookies[...] / cookies[...]
//   - ENV["..."] / ENV.fetch / ENV[...]
//   - JSON.parse / YAML.load / Marshal.load of a non-literal
//     (YAML.safe_load / JSON.parse on a literal-only string are not
//     considered taint sources)
//
// Sinks:
//   - SQL injection: ActiveRecord raw find_by_sql / where("col = #{v}")
//     style string interpolation, connection.execute / .exec_query with
//     a non-literal arg
//   - Command injection: system(...), exec(...), `backticks`, %x{...},
//     Open3.* with shell=true, IO.popen, Kernel#eval / instance_eval /
//     class_eval / send / public_send with a non-literal arg
//   - Path traversal: File.write / File.read / File.open / IO.write /
//     IO.read / FileUtils.* with a non-literal first arg
//   - XSS: ERB::Util.html_safe + html_safe on a non-literal,
//     raw(<non-literal>) inside an erb / haml block
//   - Deserialisation: Marshal.load is RCE-capable
//
// Sanitizers:
//   - Parameterised SQL: ActiveRecord where(hash) form, where("col = ?",
//     val) placeholder form, sanitize_sql_array
//   - HTML escape: CGI.escapeHTML, ERB::Util.html_escape, h(...) in an
//     ERB template, Rack::Utils.escape_html
//   - Strong parameters: params.require(...).permit(...) — Rails canonical
//   - Validation: ActiveModel::Validations declarations (`validates :x,
//     presence: true`) — HARD RULE per #2772: a declared validator counts
//     as a sanitizer; bare model.valid? does not
package substrate

import "regexp"

func init() { RegisterTaintSniffer("ruby", sniffTaintRuby) }

// rbSourceParamsRe matches Rails / Sinatra / Grape params access.
// `params[:x]` is the canonical Rails handler input shape.
var rbSourceParamsRe = regexp.MustCompile(
	`\bparams\s*\[\s*[:'"][A-Za-z_][\w]*[:'"]?\s*\]` +
		`|\brequest\s*\.\s*(?:params|body|raw_post|headers|cookies|env)\b` +
		`|\bcookies\s*\[\s*[:'"][A-Za-z_][\w]*[:'"]?\s*\]`,
)

// rbSourceEnvRe matches ENV reads.
var rbSourceEnvRe = regexp.MustCompile(
	`\bENV\s*(?:\[\s*['"][A-Z_][A-Z0-9_]*['"]\s*\]|\.\s*(?:fetch|[\[]))`,
)

// rbSourceDeserializeRe matches deserialisation primitives. Marshal.load
// is RCE-capable. YAML.load (the unsafe form, distinct from
// YAML.safe_load) similarly. JSON.parse on a non-literal is a milder
// source — flagged at lower confidence.
var rbSourceDeserializeRe = regexp.MustCompile(
	`\bMarshal\s*\.\s*load\s*\(` +
		`|\bYAML\s*\.\s*(?:unsafe_load|load)\s*\(` +
		`|\bJSON\s*\.\s*parse\s*\(\s*[a-z_][\w]*\s*[,)]`,
)

// rbSinkSQLRe matches AR raw SQL exec or interpolated where strings.
// The parameterised forms (where(hash) and where("...?", val)) are
// caught by the sanitizer regex below.
var rbSinkSQLRe = regexp.MustCompile(
	`\b(?:find_by_sql|exec_query|execute)\s*\(\s*(?:"[^"]*#\{|[a-z_][\w]*\s*\))` +
		`|\.\s*where\s*\(\s*"[^"]*#\{`,
)

// rbSinkExecRe matches command-injection primitives. system / exec /
// backticks / %x{...} / Open3 / IO.popen / Kernel.eval.
// (?:^|[^.\w]) prevents matching `obj.system` style methods.
var rbSinkExecRe = regexp.MustCompile(
	`(?:^|[^.\w])(?:system|exec|eval|instance_eval|class_eval|module_eval)\s*\(` +
		`|\bOpen3\s*\.\s*(?:popen3|capture2|capture3|capture2e)\s*\(` +
		`|\bIO\s*\.\s*popen\s*\(` +
		"|`[^`]*#\\{",
)

// rbSinkFSRe matches destructive / write file operations with a
// non-literal first arg. Literal-path opens are benign.
var rbSinkFSRe = regexp.MustCompile(
	`\bFile\s*\.\s*(?:write|read|open|delete|unlink|rename)\s*\(\s*[a-z_][\w]*\s*[,)]` +
		`|\bIO\s*\.\s*(?:write|read)\s*\(\s*[a-z_][\w]*\s*[,)]` +
		`|\bFileUtils\s*\.\s*(?:rm|rm_rf|mv|cp|cp_r|rmtree)\s*\(\s*[a-z_][\w]*\s*[,)]`,
)

// rbSinkXSSRe matches html_safe / raw on a non-literal. ERB's <%= raw
// var %> bypasses escape; html_safe on a tainted string is the Rails
// XSS vector.
var rbSinkXSSRe = regexp.MustCompile(
	`\b[a-z_][\w]*\s*\.\s*html_safe\b` +
		`|(?:^|[^.\w])raw\s*\(\s*[a-z_][\w]*\s*\)`,
)

// rbSinkReDoSRe matches Regexp.new of a non-literal.
var rbSinkReDoSRe = regexp.MustCompile(
	`\bRegexp\s*\.\s*new\s*\(\s*[a-z_][\w]*\s*[,)]`,
)

// rbSanitizerSQLRe matches the parameterised-AR form: where with a
// placeholder string + value, where with a hash, sanitize_sql_array.
var rbSanitizerSQLRe = regexp.MustCompile(
	`\.\s*where\s*\(\s*(?:\{|"[^"]*\?[^"]*"\s*,|:[A-Za-z_][\w]*\s*=>)` +
		`|\bsanitize_sql_array\s*\(` +
		`|\bsanitize_sql\s*\(`,
)

// rbSanitizerHTMLRe matches HTML-escape primitives.
var rbSanitizerHTMLRe = regexp.MustCompile(
	`\bCGI\s*\.\s*escapeHTML\s*\(` +
		`|\bERB::Util\s*\.\s*html_escape\s*\(` +
		`|\bRack::Utils\s*\.\s*escape_html\s*\(` +
		`|(?:^|[^.\w])h\s*\(\s*[a-z_][\w]*\s*\)`,
)

// rbSanitizerStrongParamsRe matches the Rails strong-parameters idiom:
// params.require(:x).permit(:a, :b). HARD RULE per #2772 — this counts
// because it declares an explicit allow-list of attributes.
var rbSanitizerStrongParamsRe = regexp.MustCompile(
	`\bparams\s*\.\s*require\s*\([^)]*\)\s*\.\s*permit\s*\(` +
		`|\bparams\s*\.\s*permit\s*\(`,
)

// rbSanitizerValidatesRe matches ActiveModel-style schema declarations.
// HARD RULE per #2772: the declaration (`validates :x, presence: true`)
// is what counts, not bare model.valid? calls.
var rbSanitizerValidatesRe = regexp.MustCompile(
	`(?m)^\s*validates\s+:[A-Za-z_][\w]*\s*,`,
)

func sniffTaintRuby(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanRubyFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, rbSourceParamsRe, TaintKindSource, TaintCategoryGeneric, "params/request.body/cookies", 1.0)
	out = appendTaintMatches(out, content, headers, rbSourceEnvRe, TaintKindSource, TaintCategoryGeneric, "ENV[...]/fetch", 0.85)
	out = appendTaintMatches(out, content, headers, rbSourceDeserializeRe, TaintKindSource, TaintCategoryDeserialization, "Marshal.load/YAML.load/JSON.parse(ident)", 0.9)
	// Sanitizers before sinks.
	out = appendTaintMatches(out, content, headers, rbSanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "AR where(hash/?,val)/sanitize_sql", 1.0)
	out = appendTaintMatches(out, content, headers, rbSanitizerHTMLRe, TaintKindSanitizer, TaintCategoryXSS, "CGI.escapeHTML/ERB::Util/h()", 1.0)
	out = appendTaintMatches(out, content, headers, rbSanitizerStrongParamsRe, TaintKindSanitizer, TaintCategoryGeneric, "params.require.permit", 0.95)
	out = appendTaintMatches(out, content, headers, rbSanitizerValidatesRe, TaintKindSanitizer, TaintCategoryGeneric, "ActiveModel validates", 0.85)
	// Sinks.
	out = appendTaintMatches(out, content, headers, rbSinkSQLRe, TaintKindSink, TaintCategorySQL, "find_by_sql/where(\"...#{...}\")", 0.9)
	out = appendTaintMatches(out, content, headers, rbSinkExecRe, TaintKindSink, TaintCategoryCommand, "system/exec/eval/backticks/Open3", 1.0)
	out = appendTaintMatches(out, content, headers, rbSinkFSRe, TaintKindSink, TaintCategoryPath, "File.write/IO.write/FileUtils(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, rbSinkXSSRe, TaintKindSink, TaintCategoryXSS, "html_safe/raw(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, rbSinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "Regexp.new(non-literal)", 0.9)
	return out
}
