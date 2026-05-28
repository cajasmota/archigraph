// Elixir taint-sites sniffer (#2773 Phase 2B T2).
//
// Recognises Elixir source / sink / sanitizer primitives across
// Phoenix, Phoenix LiveView, Plug, Cowboy, Bandit, Absinthe, Ash, Oban.
//
// Sources:
//   - Phoenix / Plug: conn.params / conn.body_params / conn.query_params
//     / conn.req_headers / conn.cookies
//   - Plug.Conn fetched params via fetch_query_params / read_body
//   - System.get_env / System.fetch_env! / System.cmd inputs from outside
//   - :erlang.binary_to_term on untrusted bytes (RCE-capable)
//
// Sinks:
//   - SQL injection: Ecto.Adapters.SQL.query / query! with a string
//     interpolation, Repo.query("..." <> v), raw fragment in Ecto's
//     fragment("CAST(? AS ...)") with a user-controlled string is safe
//     (parameterised) — flagged only when fragment is built via Kernel.
//     binary concat
//   - Command injection: System.cmd(<non-literal>, args), Port.open
//     {:spawn, ...} with user input, :os.cmd
//   - Path traversal: File.write / read / open / rm with a non-literal
//     path, Path.join with a user-controlled segment
//   - XSS: Phoenix.HTML.raw on a non-literal, EEx render with @raw_html
//     assign, Phoenix.HTML.html_escape skipped (safe template flow)
//   - Deserialisation: :erlang.binary_to_term, :erlang.binary_to_atom
//     (atom-table exhaustion DoS — flagged at lower confidence)
//
// Sanitizers:
//   - Parameterised SQL: Ecto.Query DSL (from p in Post, where: p.id ==
//     ^id), Ecto.Adapters.SQL.query with [params] list, Repo.all /
//     Repo.get / Repo.get_by — these always parameterise via Ecto's
//     compile-time query builder
//   - HTML escape: Phoenix.HTML.html_escape, Plug.HTML.html_escape,
//     HtmlSanitizeEx.strip_tags / sanitize
//   - Validation: Ecto changesets — cast(struct, params, [allowed]) +
//     validate_required / validate_format / validate_length is the
//     schema declaration (HARD RULE per #2772: the cast / validate_*
//     pipeline is what counts; a bare changeset call without validators
//     is weaker but we still mark it because cast filters by allow-list)
//   - Phoenix.Token / Plug.Crypto.MessageVerifier: signed-message
//     verification, which proves origin (treated as a generic sanitizer)
package substrate

import "regexp"

func init() { RegisterTaintSniffer("elixir", sniffTaintElixir) }

// exSourceConnRe matches Plug.Conn / Phoenix.Controller input access.
// `conn.params` is the canonical Phoenix handler input.
var exSourceConnRe = regexp.MustCompile(
	`\bconn\s*\.\s*(?:params|body_params|query_params|path_params|req_headers|cookies|host|remote_ip|request_path)\b` +
		`|\bfetch_query_params\s*\(\s*conn\b` +
		`|\bread_body\s*\(\s*conn\b`,
)

// exSourceEnvRe matches System.get_env / fetch_env / System.cmd.
var exSourceEnvRe = regexp.MustCompile(
	`\bSystem\s*\.\s*(?:get_env|fetch_env!?|get_env!?)\s*\(`,
)

// exSourceDeserializeRe matches the canonical Elixir RCE / DoS
// deserialisation primitives.
var exSourceDeserializeRe = regexp.MustCompile(
	`\b:erlang\s*\.\s*binary_to_term\s*\(` +
		`|\b:erlang\s*\.\s*binary_to_atom\s*\(` +
		`|\bString\s*\.\s*to_atom\s*\(\s*[a-z_][\w]*\s*\)`,
)

// exSinkSQLRe matches Ecto's raw query forms with string interpolation
// or concatenation. The compile-time Ecto.Query DSL (`from p in ...`)
// is the sanitizer.
var exSinkSQLRe = regexp.MustCompile(
	`\b(?:Ecto\s*\.\s*Adapters\s*\.\s*SQL|Repo)\s*\.\s*query!?\s*\(\s*"[^"]*#\{` +
		`|\b(?:Ecto\s*\.\s*Adapters\s*\.\s*SQL|Repo)\s*\.\s*query!?\s*\(\s*[a-z_][\w]*\s*<>\s*` +
		`|\bfragment\s*\(\s*[a-z_][\w]*\s*\)`,
)

// exSinkExecRe matches command-injection primitives.
var exSinkExecRe = regexp.MustCompile(
	`\bSystem\s*\.\s*cmd\s*\(\s*[a-z_][\w]*\s*[,)]` +
		`|\bPort\s*\.\s*open\s*\(\s*\{\s*:spawn\s*,\s*[a-z_][\w]*` +
		`|\b:os\s*\.\s*cmd\s*\(` +
		`|\bCode\s*\.\s*eval_string\s*\(` +
		`|\bCode\s*\.\s*eval_quoted\s*\(`,
)

// exSinkFSRe matches File / Path operations with a non-literal arg.
var exSinkFSRe = regexp.MustCompile(
	`\bFile\s*\.\s*(?:write!?|read!?|open!?|rm!?|rm_rf!?|rename!?|cp!?|cp_r!?|stream!|mkdir!?|mkdir_p!?)\s*\(\s*[a-z_][\w]*\s*[,)]` +
		`|\bPath\s*\.\s*join\s*\(\s*[a-z_][\w]*\s*,\s*[a-z_][\w]*\s*\)`,
)

// exSinkXSSRe matches Phoenix.HTML.raw on a non-literal.
var exSinkXSSRe = regexp.MustCompile(
	`\bPhoenix\.HTML\s*\.\s*raw\s*\(\s*[a-z_][\w]*\s*\)` +
		`|\b\{\s*:safe\s*,\s*[a-z_][\w]*\s*\}`,
)

// exSinkReDoSRe matches Regex.compile / ~r runtime construction.
var exSinkReDoSRe = regexp.MustCompile(
	`\bRegex\s*\.\s*compile!?\s*\(\s*[a-z_][\w]*\s*[,)]`,
)

// exSanitizerSQLRe matches Ecto's parameterised query forms. `from p
// in Post, where: p.id == ^var` is the canonical safe shape; pinned
// (`^`) values are bound parameters. Repo.get* always parameterise.
var exSanitizerSQLRe = regexp.MustCompile(
	`\bfrom\s+[a-z_][\w]*\s+in\s+[A-Z]` +
		`|\bRepo\s*\.\s*(?:get|get!|get_by|get_by!|all|one|one!|insert|insert!|update|update!|delete|delete!|preload)\s*\(` +
		`|\bEcto\s*\.\s*Adapters\s*\.\s*SQL\s*\.\s*query!?\s*\(\s*[^,]+,\s*"[^"]*\$[0-9]+[^"]*"\s*,\s*\[` +
		`|\^[a-z_][\w]*\b`,
)

// exSanitizerHTMLRe matches HTML-escape primitives.
var exSanitizerHTMLRe = regexp.MustCompile(
	`\bPhoenix\.HTML\s*\.\s*html_escape\s*\(` +
		`|\bPlug\.HTML\s*\.\s*html_escape\s*\(` +
		`|\bHtmlSanitizeEx\s*\.\s*(?:strip_tags|sanitize|basic_html|html5)\s*\(`,
)

// exSanitizerChangesetRe matches the Ecto.Changeset pipeline. HARD
// RULE per #2772: cast/3 with an allow-list of fields counts as a
// sanitiser because it filters input by an explicit allow-list;
// validate_* further constrains the values.
var exSanitizerChangesetRe = regexp.MustCompile(
	`\bcast\s*\(\s*[^,]+,\s*[^,]+,\s*\[\s*:[A-Za-z_][\w]*` +
		`|\bvalidate_(?:required|format|length|number|inclusion|exclusion|change|confirmation|subset|acceptance)\s*\(` +
		`|\bPlug\.Crypto\.MessageVerifier\s*\.\s*verify\s*\(` +
		`|\bPhoenix\.Token\s*\.\s*verify\s*\(`,
)

func sniffTaintElixir(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanElixirFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, exSourceConnRe, TaintKindSource, TaintCategoryGeneric, "conn.params/body_params/req_headers", 1.0)
	out = appendTaintMatches(out, content, headers, exSourceEnvRe, TaintKindSource, TaintCategoryGeneric, "System.get_env/fetch_env", 0.85)
	out = appendTaintMatches(out, content, headers, exSourceDeserializeRe, TaintKindSource, TaintCategoryDeserialization, ":erlang.binary_to_term/atom/String.to_atom(ident)", 0.95)
	// Sanitizers first.
	out = appendTaintMatches(out, content, headers, exSanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "from..in../Repo.get/^pinned", 1.0)
	out = appendTaintMatches(out, content, headers, exSanitizerHTMLRe, TaintKindSanitizer, TaintCategoryXSS, "Phoenix.HTML.html_escape/HtmlSanitizeEx", 1.0)
	out = appendTaintMatches(out, content, headers, exSanitizerChangesetRe, TaintKindSanitizer, TaintCategoryGeneric, "cast([:fields])/validate_*/Plug.Crypto/Phoenix.Token", 0.95)
	// Sinks.
	out = appendTaintMatches(out, content, headers, exSinkSQLRe, TaintKindSink, TaintCategorySQL, "Repo.query(\"#{...}\"/concat/fragment(ident))", 0.9)
	out = appendTaintMatches(out, content, headers, exSinkExecRe, TaintKindSink, TaintCategoryCommand, "System.cmd/Port.open/:os.cmd/Code.eval_*", 1.0)
	out = appendTaintMatches(out, content, headers, exSinkFSRe, TaintKindSink, TaintCategoryPath, "File.write/read/rm/Path.join(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, exSinkXSSRe, TaintKindSink, TaintCategoryXSS, "Phoenix.HTML.raw/{:safe,ident}", 0.85)
	out = appendTaintMatches(out, content, headers, exSinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "Regex.compile(non-literal)", 0.9)
	return out
}
