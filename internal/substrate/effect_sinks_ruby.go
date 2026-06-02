// Ruby effect-sink sniffer (#2765 Phase 1A T2).
//
// Recognises Ruby sink primitives:
//
//   - http_out  : Net::HTTP.<get|post|start|...>, HTTParty.<verb>, RestClient
//     .<verb>, Faraday.<verb>, Excon.<verb>, open-uri's
//     URI.open / URI.parse(...).open
//   - db_read   : ActiveRecord `.where / .find / .find_by / .find_each /
//     .all / .first / .last / .pluck / .count / .exists?`,
//     and raw-driver SELECT/WITH queries — pg `conn.exec` /
//     `conn.exec_params`, mysql2 `client.query`, sqlite3 /
//     ActiveRecord `db.execute` — with the FROM table named in
//     the sink (e.g. rawsql.read(orders)) (#3949)
//   - db_write  : ActiveRecord `.create / .create! / .update / .update! /
//     .update_all / .destroy / .destroy_all / .delete /
//     .delete_all / .save / .save! / .insert / .insert_all /
//     .upsert / .upsert_all`, raw-driver INSERT/UPDATE/DELETE/…
//     queries via pg/mysql2/sqlite3 `exec|exec_params|query|
//     execute` with the target table named in the sink
//     (e.g. rawsql.write(users)) (#3949), and Sequel dataset
//     writes `DB[:table]…insert/update/delete/multi_insert/
//     import/truncate` (e.g. sequel.write(users))
//   - fs_read   : File.read / .open (default "r"), File.readlines, IO.read,
//     Dir.entries / .glob / .[], Pathname#read
//   - fs_write  : File.write / .open with "w"/"a"/"x"/binary modes,
//     File.delete / .unlink / .rename, FileUtils.cp / .mv /
//     .rm / .mkdir / .mkdir_p
//   - mutation  : `@ivar = ...` instance-variable assignment inside a
//     method body. Excludes `==` comparisons.
//
// Function attribution uses the nearest preceding `def` header — the
// same heuristic the other sniffers use. Ruby's indentation-free grammar
// makes this slightly less precise than Python but accurate enough for
// the substrate's needs.
package substrate

import (
	"regexp"
	"strings"
)

func init() { RegisterEffectSniffer("ruby", sniffEffectsRuby) }

// rubyFuncHeaderRe matches `def name` or `def self.name`. Capture group 1
// is the bare method name.
var rubyFuncHeaderRe = regexp.MustCompile(
	`(?m)^\s*def\s+(?:self\s*\.\s*)?([A-Za-z_][\w]*[!?=]?)\b`,
)

// rubyHTTPRe matches outbound HTTP primitives.
var rubyHTTPRe = regexp.MustCompile(
	`\bNet::HTTP\s*\.\s*(?:get|get_response|post|post_form|start|put|delete|head|patch|request)\b` +
		`|\bHTTParty\s*\.\s*(?:get|post|put|patch|delete|head|options)\s*\(` +
		`|\bRestClient\s*\.\s*(?:get|post|put|patch|delete|head)\s*\(` +
		`|\bFaraday\s*\.\s*(?:get|post|put|patch|delete|head|new)\b` +
		`|\bExcon\s*\.\s*(?:get|post|put|patch|delete|head|new)\b` +
		`|\bURI\s*\.\s*(?:open|parse)\s*\([^)]*\)\s*\.\s*(?:read|open)\b` +
		`|\bopen-uri\b`,
)

// rubyDBReadRe matches ActiveRecord and raw read primitives.
var rubyDBReadRe = regexp.MustCompile(
	`\.\s*(?:where|find|find_by|find_each|find_in_batches|all|first|last|pluck|count|exists\?|any\?|many\?|none\?|take|select|joins|includes|preload|eager_load|references|distinct|order|group|having|limit|offset)\s*[\(.]` +
		`|\.\s*find\s*\(\s*[A-Za-z_0-9:]` +
		`|\bconnection\s*\.\s*(?:execute|exec_query|select_all|select_one|select_value|select_values|select_rows)\s*\(`,
)

// rubyDBWriteRe matches ActiveRecord and raw write primitives.
var rubyDBWriteRe = regexp.MustCompile(
	`\.\s*(?:create|create!|update|update!|update_all|update_column|update_columns|destroy|destroy!|destroy_all|delete|delete_all|save|save!|insert|insert_all|insert_all!|upsert|upsert_all|increment!|decrement!|touch|toggle!)\s*[\(.!]?` +
		`|\.\s*save\s*$`,
)

// rubySequelDatasetWriteRe matches Sequel dataset writes anchored on the
// `DB[:table]` dataset literal so the table name is recoverable (#3950):
//
//	DB[:users].insert(name: 'x')
//	DB[:users].where(id: 1).update(name: 'x')
//	DB[:logs].where(stale: true).delete
//	DB[:users].multi_insert(rows)  /  .import(cols, rows)  /  .update_all(...)
//
// Capture group 1 is the table symbol/string so the sink can name the table.
// The terminal write verb may be separated from the dataset by an arbitrary
// chain of read-only refinements (.where/.exclude/.filter/.join/...), matched
// permissively as `[^\n;]*?` up to the write verb. Only mutating verbs are
// listed, so a pure read chain (e.g. `DB[:users].where(id: 1).all`) does NOT
// match — preserving the read/write split.
var rubySequelDatasetWriteRe = regexp.MustCompile(
	`\bDB\s*\[\s*:?["']?(\w+)["']?\s*\]` + // DB[:users] / DB['users']
		`[^\n;]*?` + // optional read-only refinement chain
		`\.\s*(?:insert|multi_insert|import|update|update_all|delete|truncate|insert_ignore)\b`,
)

// rubyRawDriverCallRe matches a raw-driver query call carrying a SQL string
// literal as its first argument (#3949). It anchors on the Rails-adjacent raw
// drivers' call shapes:
//
//	pg      : conn.exec("SELECT ...")        / conn.exec_params("INSERT ...", [])
//	mysql2  : client.query("SELECT ...")
//	sqlite3 : db.execute("UPDATE ...")       (also covers ActiveRecord
//	          connection.execute, kept for the table-naming upgrade)
//
// The verb set is restricted to {exec, exec_params, query, execute} so this
// does NOT fire on arbitrary `.foo("SELECT...")` strings. Capture group 1 is
// the SQL string body (single- or double-quoted, no escape unescaping needed
// for table extraction). Interpolated table names (`#{...}`) remain in the
// captured body and are deliberately NOT matched by the table regexes below,
// so a dynamic table yields an honest no-table effect (no fabrication).
var rubyRawDriverCallRe = regexp.MustCompile(
	`\.\s*(?:exec|exec_params|query|execute)\s*\(\s*["']([^"']*)["']`,
)

// Raw-SQL table extractors mirror internal/patterns/raw_sql_extractor.go so the
// table name surfaced here is consistent with the raw-SQL entity extractor.
// Each capture group 1 is the (leaf) table name; an optional `schema.` prefix
// is tolerated and stripped by the regex's non-capturing `(?:\w+\.)?`.
var (
	rubySQLSelectTableRe = regexp.MustCompile(`(?i)\bSELECT\b[\s\S]*?\bFROM\s+(?:\w+\.)?(\w+)`)
	rubySQLInsertTableRe = regexp.MustCompile(`(?i)\bINSERT\s+(?:IGNORE\s+)?INTO\s+(?:\w+\.)?(\w+)`)
	rubySQLUpdateTableRe = regexp.MustCompile(`(?i)\bUPDATE\s+(?:\w+\.)?(\w+)\s+SET\b`)
	rubySQLDeleteTableRe = regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+(?:\w+\.)?(\w+)`)
)

// rubyFSReadRe matches read-only filesystem primitives.
var rubyFSReadRe = regexp.MustCompile(
	`\bFile\s*\.\s*(?:read|readlines|open\s*\(\s*[^,)]+\s*\)|foreach)\b` +
		`|\bFile\s*\.\s*open\s*\(\s*[^,)]+\s*,\s*['"](?:r|rb|rt)['"]` +
		`|\bIO\s*\.\s*(?:read|readlines|foreach)\b` +
		`|\bDir\s*\.\s*(?:entries|glob|\[|children|each|foreach)\b` +
		`|\bPathname\b[^=]*\.\s*read\b`,
)

// rubyFSWriteRe matches write filesystem primitives.
var rubyFSWriteRe = regexp.MustCompile(
	`\bFile\s*\.\s*(?:write|delete|unlink|rename|chmod|chown|truncate)\b` +
		`|\bFile\s*\.\s*open\s*\(\s*[^,)]+\s*,\s*['"](?:w|wb|wt|a|ab|at|x|xb|r\+|w\+|a\+)['"]` +
		`|\bFileUtils\s*\.\s*(?:cp|cp_r|mv|rm|rm_rf|rm_f|mkdir|mkdir_p|chmod|chown|touch|ln|ln_s)\b` +
		`|\bIO\s*\.\s*write\b`,
)

// rubyProcessRe matches process-spawn primitives. We classify these as
// fs_write under the substrate's "external side-effect" interpretation
// — Process.spawn / system / exec / backticks can mutate the host.
var rubyProcessRe = regexp.MustCompile(
	`\bProcess\s*\.\s*(?:spawn|exec|fork|kill|wait|detach)\b` +
		`|\bKernel\s*\.\s*(?:system|exec|spawn)\b` +
		`|^\s*system\s*\(`,
)

// rubyMutationRe matches `@ivar = ...` instance-variable assignment.
// Excludes `==` by requiring a non-`=` continuation.
var rubyMutationRe = regexp.MustCompile(
	`@[A-Za-z_][\w]*\s*=(?:[^=])`,
)

func sniffEffectsRuby(content string) []EffectMatch {
	if content == "" {
		return nil
	}
	headers := scanRubyFuncHeaders(content)
	var out []EffectMatch
	out = appendRubyMatches(out, content, headers, rubyHTTPRe, EffectHTTPOut, "Net::HTTP/HTTParty/Faraday", 1.0)
	out = appendRubyMatches(out, content, headers, rubyDBReadRe, EffectDBRead, "activerecord.read", 0.85)
	out = appendRubyMatches(out, content, headers, rubyDBWriteRe, EffectDBWrite, "activerecord.write", 0.85)
	out = appendRubyRawDriverSQL(out, content, headers)
	out = appendRubySequelDatasetWrites(out, content, headers)
	out = appendRubyMatches(out, content, headers, rubyFSReadRe, EffectFSRead, "File.read/IO.read", 1.0)
	out = appendRubyMatches(out, content, headers, rubyFSWriteRe, EffectFSWrite, "File.write/FileUtils", 1.0)
	out = appendRubyMatches(out, content, headers, rubyProcessRe, EffectFSWrite, "Process.spawn/system", 0.9)
	out = appendRubyMatches(out, content, headers, rubyMutationRe, EffectMutation, "@ivar=", 0.7)
	return out
}

func scanRubyFuncHeaders(content string) []funcHeader {
	var hs []funcHeader
	for _, m := range rubyFuncHeaderRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		hs = append(hs, funcHeader{Line: lineOfOffset(content, m[0]), Name: content[m[2]:m[3]]})
	}
	return hs
}

func appendRubyMatches(out []EffectMatch, content string, headers []funcHeader, re *regexp.Regexp, eff Effect, sink string, conf float64) []EffectMatch {
	for _, m := range re.FindAllStringIndex(content, -1) {
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		out = append(out, EffectMatch{
			Function:   fn,
			Line:       line,
			Effect:     eff,
			Sink:       sink,
			Confidence: conf,
		})
	}
	return out
}

// appendRubyRawDriverSQL stamps a read/write EffectMatch for each raw-driver
// SQL call (#3949) — pg `conn.exec`/`exec_params`, mysql2 `client.query`,
// sqlite3 `db.execute` (and ActiveRecord `connection.execute`). The SQL string
// literal is parsed to (a) classify the statement as a read (SELECT/WITH) or a
// write (INSERT/UPDATE/DELETE/…) and (b) name the target table in the sink tag,
// e.g. `pg.exec(users)` / `mysql2.query(orders)` / `sqlite3.execute(accounts)`.
//
// Effect classification:
//   - SELECT / WITH …                 → db_read
//   - INSERT / UPDATE / DELETE /
//     REPLACE / MERGE / TRUNCATE      → db_write
//
// Table naming reuses the same shapes as the raw-SQL entity extractor
// (internal/patterns/raw_sql_extractor.go). When the statement is a recognised
// DML verb but no table can be extracted — e.g. a dynamic / interpolated table
// (`"SELECT * FROM #{t}"`) or a non-DML statement — the effect is still stamped
// (the call genuinely touches the DB) but WITHOUT a fabricated table name; the
// sink tag falls back to the bare verb (`pg.exec(?)`). A non-SQL string arg
// (no DML verb at all) is NOT a DB effect and is skipped entirely.
//
// Full confidence: the restricted verb set + explicit SQL DML token make the
// effect unambiguous (unlike the generic ActiveRecord `.where`/`.save`
// heuristics which fire on unknown receivers at 0.85).
func appendRubyRawDriverSQL(out []EffectMatch, content string, headers []funcHeader) []EffectMatch {
	for _, m := range rubyRawDriverCallRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		sql := content[m[2]:m[3]]
		eff, table, ok := classifyRubyRawSQL(sql)
		if !ok {
			continue // non-SQL string arg: not a DB effect.
		}
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		out = append(out, EffectMatch{
			Function:   fn,
			Line:       line,
			Effect:     eff,
			Sink:       rubyRawSQLSink(eff, table),
			Confidence: 1.0,
		})
	}
	return out
}

// classifyRubyRawSQL inspects a raw SQL string body and returns the effect
// (db_read / db_write), the target table (empty when not statically
// recoverable), and ok=false when the body carries no recognised SQL DML verb
// (so the caller can skip non-SQL string args without fabricating an effect).
func classifyRubyRawSQL(sql string) (Effect, string, bool) {
	if m := rubySQLInsertTableRe.FindStringSubmatch(sql); m != nil {
		return EffectDBWrite, m[1], true
	}
	if m := rubySQLUpdateTableRe.FindStringSubmatch(sql); m != nil {
		return EffectDBWrite, m[1], true
	}
	if m := rubySQLDeleteTableRe.FindStringSubmatch(sql); m != nil {
		return EffectDBWrite, m[1], true
	}
	if m := rubySQLSelectTableRe.FindStringSubmatch(sql); m != nil {
		return EffectDBRead, m[1], true
	}
	// Verb present but table not statically recoverable (e.g. interpolated
	// table, bare INSERT without a parseable INTO, or a write keyword without
	// a SET). Classify the effect honestly with no fabricated table.
	upper := strings.ToUpper(sql)
	switch {
	case strings.Contains(upper, "INSERT ") || strings.Contains(upper, "UPDATE ") ||
		strings.Contains(upper, "DELETE ") || strings.Contains(upper, "REPLACE ") ||
		strings.Contains(upper, "MERGE ") || strings.Contains(upper, "TRUNCATE "):
		return EffectDBWrite, "", true
	case strings.HasPrefix(strings.TrimSpace(upper), "SELECT") ||
		strings.HasPrefix(strings.TrimSpace(upper), "WITH"):
		return EffectDBRead, "", true
	}
	return "", "", false
}

// rubyRawSQLSink builds the sink tag for a raw-driver SQL effect, naming the
// table when known and falling back to `(?)` when it is not.
func rubyRawSQLSink(eff Effect, table string) string {
	verb := "rawsql.read"
	if eff == EffectDBWrite {
		verb = "rawsql.write"
	}
	if table == "" {
		return verb + "(?)"
	}
	return verb + "(" + table + ")"
}

// appendRubySequelDatasetWrites stamps a db_write EffectMatch for each Sequel
// dataset write (`DB[:table]…insert/update/delete`), naming the table in the
// sink tag (#3950) — e.g. `sequel.write(users)`. Full confidence: the
// `DB[:table]` dataset literal makes the write unambiguous (unlike the generic
// `.update`/`.save` heuristic which can fire on an unknown receiver).
func appendRubySequelDatasetWrites(out []EffectMatch, content string, headers []funcHeader) []EffectMatch {
	for _, m := range rubySequelDatasetWriteRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		line := lineOfOffset(content, m[0])
		fn := nearestHeader(headers, line)
		table := content[m[2]:m[3]]
		out = append(out, EffectMatch{
			Function:   fn,
			Line:       line,
			Effect:     EffectDBWrite,
			Sink:       "sequel.write(" + table + ")",
			Confidence: 1.0,
		})
	}
	return out
}
