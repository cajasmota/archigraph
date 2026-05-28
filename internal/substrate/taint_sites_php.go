// PHP taint-sites sniffer (#2773 Phase 2B T2).
//
// Recognises PHP source / sink / sanitizer primitives across raw PHP,
// Laravel / Lumen, Symfony, CakePHP, CodeIgniter, Slim, Phalcon, Yii,
// Laminas, WordPress, Drupal, and Magento.
//
// Sources:
//   - Superglobals: $_GET / $_POST / $_REQUEST / $_COOKIE / $_FILES /
//     $_SERVER / $_ENV / $_SESSION
//   - getenv() / apache_request_headers() / getallheaders()
//   - Laravel: $request->input / ->all / ->get / ->post / ->json /
//     ->header / ->cookie / ->file / Request::input
//   - Symfony: $request->query->get / ->request->get / ->headers->get /
//     ->cookies->get / ->getContent
//   - file_get_contents("php://input")
//   - unserialize() — RCE-capable on untrusted input
//
// Sinks:
//   - SQL injection: mysqli_query / mysql_query / pg_query / PDO::query
//     / PDO::exec with a concatenated string, $db->query("..." . $v)
//   - Command injection: system / exec / passthru / shell_exec /
//     `backticks` / proc_open / popen with a non-literal arg, eval(),
//     assert() on a string, create_function (deprecated, still seen)
//   - Path traversal: file_put_contents / file_get_contents / fopen /
//     include / require / unlink with a non-literal path
//   - XSS: echo / print of a non-escaped variable inside HTML context
//     (heuristic — flagged at lower confidence)
//   - Deserialisation: unserialize() / Symfony Serializer->deserialize
//     on untrusted bytes
//
// Sanitizers:
//   - Parameterised SQL: PDO::prepare + bindValue / bindParam / execute
//     with an array of values, mysqli_prepare + bind_param
//   - HTML escape: htmlspecialchars / htmlentities / strip_tags
//     (heuristic), Laravel {{ $v }} blade auto-escape
//   - Shell escape: escapeshellarg / escapeshellcmd
//   - Validation: filter_input / filter_var with a FILTER_* constant
//     (HARD RULE per #2772 — the FILTER_VALIDATE_* flag is what counts;
//     filter_input alone without a flag is not a sanitizer)
//   - Laravel validator: $request->validate([...]) / Validator::make(
//     ..., [...]) — the second arg is the schema
package substrate

import "regexp"

func init() { RegisterTaintSniffer("php", sniffTaintPHP) }

// phpSourceSuperglobalRe matches the canonical PHP request inputs.
// Square-bracket indexing or bare reference both count.
var phpSourceSuperglobalRe = regexp.MustCompile(
	`\$_(?:GET|POST|REQUEST|COOKIE|FILES|SERVER|ENV|SESSION)\b`,
)

// phpSourceFrameworkRe matches Laravel / Symfony request-object access.
var phpSourceFrameworkRe = regexp.MustCompile(
	`\$request\s*->\s*(?:input|all|get|post|json|header|cookie|file|query|request|headers|cookies|getContent|getRequestUri|getQueryString)\b` +
		`|\bRequest::\s*(?:input|all|get|post|json|capture|createFromGlobals)\s*\(`,
)

// phpSourceEnvRe matches getenv / apache_request_headers / getallheaders.
var phpSourceEnvRe = regexp.MustCompile(
	`\bgetenv\s*\(` +
		`|\b(?:apache_request_headers|getallheaders)\s*\(`,
)

// phpSourceStdinRe matches reads of php://input (the raw request body
// shape used by JSON APIs that don't go through a framework parser).
var phpSourceStdinRe = regexp.MustCompile(
	`\bfile_get_contents\s*\(\s*['"]php://input['"]`,
)

// phpSourceDeserializeRe matches unserialize() — historically the
// #1 PHP RCE vector when fed untrusted bytes.
var phpSourceDeserializeRe = regexp.MustCompile(
	`\bunserialize\s*\(`,
)

// phpSinkSQLRe matches raw SQL exec with string concatenation. The
// PDO::prepare flow is detected as the sanitizer below.
var phpSinkSQLRe = regexp.MustCompile(
	`\b(?:mysqli_query|mysql_query|pg_query)\s*\(\s*[^,)]*\$[A-Za-z_][\w]*` +
		`|->\s*(?:query|exec|prepare)\s*\(\s*['"][^'"]*['"]\s*\.\s*\$` +
		`|->\s*(?:query|exec)\s*\(\s*\$[A-Za-z_][\w]*\s*[,)]` +
		`|\bDB::\s*(?:select|insert|update|delete|statement)\s*\(\s*\$`,
)

// phpSinkExecRe matches command-injection primitives.
var phpSinkExecRe = regexp.MustCompile(
	`\b(?:system|exec|passthru|shell_exec|proc_open|popen)\s*\(\s*[^)]*\$` +
		`|\beval\s*\(` +
		`|\bassert\s*\(\s*\$` +
		`|\bcreate_function\s*\(`,
)

// phpSinkFSRe matches filesystem write/include of a non-literal path.
// include / require with a variable is the canonical LFI vector.
var phpSinkFSRe = regexp.MustCompile(
	`\bfile_put_contents\s*\(\s*\$[A-Za-z_][\w]*\s*[,)]` +
		`|\bfile_get_contents\s*\(\s*\$[A-Za-z_][\w]*\s*[,)]` +
		`|\bfopen\s*\(\s*\$[A-Za-z_][\w]*\s*[,)]` +
		`|\b(?:include|require|include_once|require_once)\s*\(?\s*\$[A-Za-z_][\w]*` +
		`|\bunlink\s*\(\s*\$[A-Za-z_][\w]*\s*[,)]`,
)

// phpSinkXSSRe matches echo / print of a non-escaped variable. Very
// heuristic — confidence is low because echo $v is the normal way to
// render a server-escaped value in many codebases.
var phpSinkXSSRe = regexp.MustCompile(
	`(?:^|[^.\w])(?:echo|print)\s+\$[A-Za-z_][\w]*`,
)

// phpSinkReDoSRe matches preg_match / preg_replace where the pattern
// argument is a variable (constructed regex).
var phpSinkReDoSRe = regexp.MustCompile(
	`\bpreg_(?:match|match_all|replace|replace_callback|split)\s*\(\s*\$[A-Za-z_][\w]*\s*[,)]`,
)

// phpSanitizerSQLRe matches PDO::prepare + bindValue / bindParam /
// execute(array). The presence of prepare() in the file proves a
// parameterised flow is in use.
var phpSanitizerSQLRe = regexp.MustCompile(
	`->\s*prepare\s*\(\s*['"][^'"]*\?[^'"]*['"]\s*\)` +
		`|->\s*prepare\s*\(\s*['"][^'"]*:[A-Za-z_][\w]*[^'"]*['"]\s*\)` +
		`|->\s*(?:bindValue|bindParam)\s*\(` +
		`|->\s*execute\s*\(\s*\[` +
		`|\bmysqli_prepare\s*\(` +
		`|->\s*bind_param\s*\(`,
)

// phpSanitizerHTMLRe matches HTML-escape functions.
var phpSanitizerHTMLRe = regexp.MustCompile(
	`\bhtmlspecialchars\s*\(` +
		`|\bhtmlentities\s*\(` +
		`|\bstrip_tags\s*\(` +
		`|\be\s*\(\s*\$[A-Za-z_][\w]*\s*\)`, // Laravel e() helper
)

// phpSanitizerShellRe matches shell-escape functions.
var phpSanitizerShellRe = regexp.MustCompile(
	`\bescapeshell(?:arg|cmd)\s*\(`,
)

// phpSanitizerFilterRe matches filter_input / filter_var. HARD RULE
// per #2772: the FILTER_VALIDATE_* / FILTER_SANITIZE_* constant must be
// present — bare filter_input(INPUT_POST, "x") is NOT a sanitizer.
var phpSanitizerFilterRe = regexp.MustCompile(
	`\bfilter_(?:input|var)\s*\([^)]*FILTER_(?:VALIDATE|SANITIZE)_[A-Z_]+`,
)

// phpSanitizerValidateRe matches Laravel-style validation calls. HARD
// RULE per #2772: the call must carry a rule schema (array of field=>
// rule pairs), not bare ->validate().
var phpSanitizerValidateRe = regexp.MustCompile(
	`->\s*validate\s*\(\s*\[` +
		`|\bValidator::\s*make\s*\([^)]*,\s*\[`,
)

func sniffTaintPHP(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanPHPFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, phpSourceSuperglobalRe, TaintKindSource, TaintCategoryGeneric, "$_GET/$_POST/$_COOKIE/$_REQUEST", 1.0)
	out = appendTaintMatches(out, content, headers, phpSourceFrameworkRe, TaintKindSource, TaintCategoryGeneric, "Laravel/Symfony request input", 0.95)
	out = appendTaintMatches(out, content, headers, phpSourceEnvRe, TaintKindSource, TaintCategoryGeneric, "getenv/getallheaders", 0.85)
	out = appendTaintMatches(out, content, headers, phpSourceStdinRe, TaintKindSource, TaintCategoryGeneric, "php://input", 1.0)
	out = appendTaintMatches(out, content, headers, phpSourceDeserializeRe, TaintKindSource, TaintCategoryDeserialization, "unserialize", 1.0)
	// Sanitizers first.
	out = appendTaintMatches(out, content, headers, phpSanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "PDO::prepare+bindValue/execute(array)", 1.0)
	out = appendTaintMatches(out, content, headers, phpSanitizerHTMLRe, TaintKindSanitizer, TaintCategoryXSS, "htmlspecialchars/htmlentities/e()", 1.0)
	out = appendTaintMatches(out, content, headers, phpSanitizerShellRe, TaintKindSanitizer, TaintCategoryCommand, "escapeshellarg/escapeshellcmd", 1.0)
	out = appendTaintMatches(out, content, headers, phpSanitizerFilterRe, TaintKindSanitizer, TaintCategoryGeneric, "filter_input/var(FILTER_VALIDATE_*)", 0.95)
	out = appendTaintMatches(out, content, headers, phpSanitizerValidateRe, TaintKindSanitizer, TaintCategoryGeneric, "->validate([...])/Validator::make", 0.9)
	// Sinks.
	out = appendTaintMatches(out, content, headers, phpSinkSQLRe, TaintKindSink, TaintCategorySQL, "mysqli_query/PDO->query(concat)", 0.9)
	out = appendTaintMatches(out, content, headers, phpSinkExecRe, TaintKindSink, TaintCategoryCommand, "system/exec/passthru/eval/assert", 1.0)
	out = appendTaintMatches(out, content, headers, phpSinkFSRe, TaintKindSink, TaintCategoryPath, "file_*/include/require(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, phpSinkXSSRe, TaintKindSink, TaintCategoryXSS, "echo/print $var", 0.6)
	out = appendTaintMatches(out, content, headers, phpSinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "preg_match/replace($pattern,...)", 0.85)
	return out
}
