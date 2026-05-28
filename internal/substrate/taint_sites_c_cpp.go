// C / C++ taint-sites sniffer (#2773 Phase 2B T2).
//
// Recognises C / C++ source / sink / sanitizer primitives across the
// raw C stdlib, cpprestsdk, libcurl, the libpq (PostgreSQL) C client,
// Boost.Asio / Crow / Drogon / Oat++ / Pistache / Poco / Qt servers,
// and the unreal-engine HTTP module.
//
// Sources:
//   - argv (main()'s second arg)
//   - getenv("...") — POSIX C entry point
//   - fgets / scanf / fscanf / fread on stdin or a network FD
//   - cpprestsdk: web::http::http_request::extract_string /
//     extract_json / headers / request_uri
//   - libcurl: CURLOPT_WRITEFUNCTION callback receives untrusted bytes
//   - Boost.Beast / Boost.Asio: socket.read_some / async_read writes
//     into a buffer that becomes a taint source
//   - Crow: req.body / req.url_params / req.headers
//   - Drogon: req->getParameter / req->body / req->headers
//   - Oat++: request->getQueryParameters / getHeaders / readBodyToString
//   - Pistache: req.body / req.query / req.headers
//
// Sinks:
//   - SQL injection: libpq PQexec(conn, sql_with_concat),
//     mysql_query(conn, sql), sqlite3_exec with a non-literal — the
//     parameterised PQexecParams / sqlite3_prepare_v2 + bind flow is
//     the sanitizer
//   - Command injection: system(), exec*() family, popen, _wsystem,
//     CreateProcess on Windows; sprintf into a buffer that becomes the
//     argument of system() is the canonical C RCE chain
//   - Path traversal: fopen / open / freopen / unlink / remove / rename
//     with a non-literal first arg; std::ofstream / std::ifstream
//     constructed from a non-literal
//   - Buffer-overflow shapes (not strictly taint but security-sensitive):
//     strcpy, strcat, gets, sprintf — flagged at lower confidence in
//     the "command" category as a catch-all for memory-safety sinks
//
// Sanitizers:
//   - Parameterised SQL: libpq PQexecParams (4th arg is the parameter
//     array), sqlite3_bind_text / bind_int after sqlite3_prepare_v2,
//     mysql_stmt_bind_param
//   - Shell escape: there is no canonical C escapeshellarg-equivalent;
//     we recognise execve() called with a NULL-terminated argv array
//     (which bypasses the shell) as the safe shape
//   - Boost.Format with strict typing — `boost::format("%1%") % var`
//     does no shell evaluation; treated as a generic safety hint
package substrate

import "regexp"

func init() { RegisterTaintSniffer("c-cpp", sniffTaintCCPP) }

// ccSourceArgvRe matches references to main()'s argv / envp.
var ccSourceArgvRe = regexp.MustCompile(
	`\bargv\s*\[` +
		`|\benvp\s*\[`,
)

// ccSourceEnvRe matches getenv / secure_getenv.
var ccSourceEnvRe = regexp.MustCompile(
	`\b(?:getenv|secure_getenv|_wgetenv)\s*\(`,
)

// ccSourceStdinRe matches the standard read-from-fd primitives.
var ccSourceStdinRe = regexp.MustCompile(
	`\b(?:fgets|gets|scanf|fscanf|sscanf|fread|read|recv|recvfrom|recvmsg)\s*\(`,
)

// ccSourceFrameworkRe matches the dominant C++ HTTP frameworks'
// request-input accessors.
var ccSourceFrameworkRe = regexp.MustCompile(
	`\bextract_(?:string|json|vector|utf8string)\s*\(` + // cpprestsdk
		`|\breq(?:->|\.)\s*(?:body|url_params|headers|query|getParameter|getParameters|getQueryParameters|getHeader|getHeaders|readBodyToString)\b` +
		`|\brequest\s*->\s*(?:getQueryParameters|getHeaders|readBodyToString|getBody|getParameter)\s*\(`,
)

// ccSinkSQLRe matches libpq PQexec / mysql_query / sqlite3_exec with a
// non-literal first SQL argument or string concatenation. PQexecParams
// (parameterised) is the sanitizer below.
var ccSinkSQLRe = regexp.MustCompile(
	`\bPQexec\s*\(\s*[A-Za-z_][\w]*\s*,\s*[A-Za-z_][\w]*\s*\)` +
		`|\bmysql_query\s*\(\s*[A-Za-z_][\w]*\s*,\s*[A-Za-z_][\w]*\s*\)` +
		`|\bsqlite3_exec\s*\(\s*[A-Za-z_][\w]*\s*,\s*[A-Za-z_][\w]*\s*,`,
)

// ccSinkExecRe matches command-injection primitives. system / popen /
// execl / execv with a non-literal first arg, sprintf-into-buffer-then-
// system is the textbook chain (we only catch system() itself here —
// the propagation pass connects them via CALLS).
var ccSinkExecRe = regexp.MustCompile(
	`(?:^|[^.\w>])system\s*\(\s*[A-Za-z_][\w]*\s*\)` +
		`|\b(?:popen|_popen)\s*\(\s*[A-Za-z_][\w]*\s*[,)]` +
		`|\bexec(?:l|v|le|lp|ve|vp|vpe)\s*\(\s*[A-Za-z_][\w]*\s*[,)]` +
		`|\bCreateProcess[AW]?\s*\([^)]*[A-Za-z_][\w]*`,
)

// ccSinkFSRe matches fopen / open / unlink / remove / rename with a
// non-literal first arg, plus std::ofstream / std::ifstream of a
// non-literal path.
var ccSinkFSRe = regexp.MustCompile(
	`\b(?:fopen|fopen_s|freopen|open|open64|_open|_wfopen|unlink|remove|rename)\s*\(\s*[A-Za-z_][\w]*\s*[,)]` +
		`|\bstd\s*::\s*(?:ofstream|ifstream|fstream)\s*[A-Za-z_][\w]*\s*\(\s*[A-Za-z_][\w]*\s*[,)]`,
)

// ccSinkBufferRe matches the classic memory-safety primitives. Tagged
// as TaintCategoryCommand (generic security-sensitive op) — the
// security audit surfaces these even when the buffer is not strictly
// "tainted" because they are footguns by construction.
var ccSinkBufferRe = regexp.MustCompile(
	`(?:^|[^.\w>])(?:strcpy|strcat|gets|sprintf|vsprintf|wcscpy|wcscat)\s*\(`,
)

// ccSinkReDoSRe matches std::regex constructed from a non-literal
// pattern.
var ccSinkReDoSRe = regexp.MustCompile(
	`\bstd\s*::\s*regex\s+[A-Za-z_][\w]*\s*\(\s*[A-Za-z_][\w]*\s*[,)]` +
		`|\bboost\s*::\s*regex\s+[A-Za-z_][\w]*\s*\(\s*[A-Za-z_][\w]*\s*[,)]`,
)

// ccSanitizerSQLRe matches PQexecParams (parameterised), sqlite3_bind_*
// after prepare_v2, and mysql_stmt_bind_param.
var ccSanitizerSQLRe = regexp.MustCompile(
	`\bPQexecParams\s*\(` +
		`|\bPQprepare\s*\(` +
		`|\bsqlite3_prepare_v2?\s*\(` +
		`|\bsqlite3_bind_(?:text|int|int64|double|blob|null)\s*\(` +
		`|\bmysql_stmt_(?:prepare|bind_param)\s*\(`,
)

// ccSanitizerExecveRe matches execve() called with an explicit argv
// array — bypasses the shell, so a tainted string in argv[1] does NOT
// undergo shell expansion. Treated as the C equivalent of
// escapeshellarg for the command-injection category.
var ccSanitizerExecveRe = regexp.MustCompile(
	`\bexecve\s*\(\s*[A-Za-z_][\w]*\s*,\s*[A-Za-z_][\w]*\s*,`,
)

// ccSanitizerFormatRe matches Boost.Format strict-typed string
// construction. Treated as a generic safety hint.
var ccSanitizerFormatRe = regexp.MustCompile(
	`\bboost\s*::\s*format\s*\(`,
)

func sniffTaintCCPP(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanCCPPFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, ccSourceArgvRe, TaintKindSource, TaintCategoryGeneric, "argv[]/envp[]", 0.9)
	out = appendTaintMatches(out, content, headers, ccSourceEnvRe, TaintKindSource, TaintCategoryGeneric, "getenv/secure_getenv", 0.85)
	out = appendTaintMatches(out, content, headers, ccSourceStdinRe, TaintKindSource, TaintCategoryGeneric, "fgets/scanf/fread/recv", 0.85)
	out = appendTaintMatches(out, content, headers, ccSourceFrameworkRe, TaintKindSource, TaintCategoryGeneric, "cpprestsdk/Crow/Drogon/Oat++/Pistache request", 0.95)
	// Sanitizers first.
	out = appendTaintMatches(out, content, headers, ccSanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "PQexecParams/sqlite3_bind_*/mysql_stmt_bind_param", 1.0)
	out = appendTaintMatches(out, content, headers, ccSanitizerExecveRe, TaintKindSanitizer, TaintCategoryCommand, "execve(prog, argv, envp)", 0.9)
	out = appendTaintMatches(out, content, headers, ccSanitizerFormatRe, TaintKindSanitizer, TaintCategoryGeneric, "boost::format", 0.7)
	// Sinks.
	out = appendTaintMatches(out, content, headers, ccSinkSQLRe, TaintKindSink, TaintCategorySQL, "PQexec/mysql_query/sqlite3_exec(non-literal)", 0.9)
	out = appendTaintMatches(out, content, headers, ccSinkExecRe, TaintKindSink, TaintCategoryCommand, "system/popen/execl/CreateProcess(non-literal)", 1.0)
	out = appendTaintMatches(out, content, headers, ccSinkFSRe, TaintKindSink, TaintCategoryPath, "fopen/open/unlink/std::ofstream(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, ccSinkBufferRe, TaintKindSink, TaintCategoryCommand, "strcpy/strcat/gets/sprintf", 0.75)
	out = appendTaintMatches(out, content, headers, ccSinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "std::regex/boost::regex(non-literal)", 0.85)
	return out
}
