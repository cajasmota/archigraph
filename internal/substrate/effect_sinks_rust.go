// Rust effect-sink sniffer (#2765 Phase 1A T2).
//
// Recognises Rust sink primitives:
//
//   - http_out  : reqwest::Client / reqwest::get / .get|post|put|patch|
//     delete().send(), hyper Client::request, surf::<verb>,
//     ureq::get/post, isahc::<verb>
//   - db_read   : sqlx `query!` / `query_as!` / `query / query_as`
//     with SELECT-shaped SQL or fetch_*, Diesel `.load /
//     .get_result / .get_results`, sea-orm relation loaders
//     (`.find_by_id / .find_also_related / .stream / .paginate`);
//     the AMBIGUOUS terminals (`.first / .find / .filter /
//     .select / .all / .one`) that collide with `Iterator`
//     combinators are credited ONLY on a Diesel-query / sea-orm
//     `Select`-typed receiver (#4737, so `vec.iter().filter(...)`
//     stays pure) — see rustQueryReadMatches
//   - db_write  : sqlx `execute / fetch_one` on INSERT/UPDATE/DELETE,
//     Diesel `.insert_into / .update / .delete / .execute`,
//     sea-orm `.insert / .update / .delete / .save`
//   - fs_read   : std::fs::read / read_to_string / read_dir / File::open,
//     tokio::fs equivalents, std::path::Path::exists
//   - fs_write  : std::fs::write / create_dir(_all) / remove_file /
//     remove_dir(_all) / rename / set_permissions /
//     File::create, tokio::fs::write, std::process::Command
//   - mutation  : `self.field = ...` assignment inside a method
//
// Function attribution uses the nearest preceding `fn name(` header. The
// Rust grammar's strict `fn` keyword + visibility makes this very precise.
package substrate

import "regexp"

func init() { RegisterEffectSniffer("rust", sniffEffectsRust) }

// rustFuncHeaderRe matches `fn name(` (with optional pub / pub(...) /
// async / const / unsafe / extern qualifiers). Capture group 1 is the
// function name.
var rustFuncHeaderRe = regexp.MustCompile(
	`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+|const\s+|unsafe\s+|extern\s+(?:"[^"]*"\s+)?)*` +
		`fn\s+([A-Za-z_][\w]*)\s*[<(]`,
)

// rustHTTPRe matches outbound HTTP primitives.
var rustHTTPRe = regexp.MustCompile(
	`\breqwest\s*::\s*(?:get|Client\s*::\s*new|Client\s*::\s*builder|blocking\s*::\s*(?:get|Client))\b` +
		`|\bhyper\s*::\s*Client\b` +
		`|\bClient\s*::\s*new\s*\(\s*\)\s*\.\s*(?:get|post|put|patch|delete|head|request)\s*\(` +
		`|\b(?:surf|ureq|isahc)\s*::\s*(?:get|post|put|patch|delete|head|connect)\s*\(` +
		`|\.\s*(?:get|post|put|patch|delete|head)\s*\(\s*"https?://` +
		`|\.\s*send\s*\(\s*\)\s*\.\s*await\b`,
)

// rustDBReadRe matches the DISTINCTIVE sqlx / Diesel / sea-orm read primitives
// — names that do NOT collide with the Rust `Iterator` combinators on a plain
// Vec/slice/iterator, so they are safe to bare-match on ANY receiver (#4737):
//   - sqlx `query!`/`query_as!`/`query_scalar!` macros and the `.fetch_*`
//     terminals (`fetch_all`/`fetch_one`/`fetch_optional`/`fetch`),
//   - Diesel `diesel::select`/`diesel::sql_query` and the loader terminals
//     `.load`/`.get_result`/`.get_results` plus relation helpers,
//   - sea-orm relation loaders `.find_by_id`/`.find_also_related`/
//     `.find_with_related`/`.stream`/`.paginate`.
//
// The AMBIGUOUS terminals (`.first`/`.find`/`.filter`/`.select`/`.all`/`.one`,
// the query shapers `.order`/`.group_by`/`.limit`/`.offset`/`.inner_join`/
// `.left_join`/`.on`) are NOT here — they collide with `Iterator`
// (`vec.iter().filter(...).find(...)`, `slice.first()`, `it.map(...).all(...)`),
// which would be a false db_read. They are credited ONLY on a Diesel-query /
// sea-orm `Select`/`Entity`-typed receiver by rustQueryReadMatches (#4737
// receiver-typed read credit, mirroring the Python #4691 / C# #4692 model).
var rustDBReadRe = regexp.MustCompile(
	`\bsqlx\s*::\s*query(?:_as|_scalar)?(?:_unchecked)?\s*[!\(]` +
		`|\.\s*(?:fetch_all|fetch_one|fetch_optional|fetch)\s*\(` +
		`|\bdiesel\s*::\s*(?:select|sql_query)\b` +
		`|\.\s*(?:load|get_result|get_results)\s*\(` +
		`|\.\s*(?:find_by_id|find_also_related|find_with_related|stream|paginate)\s*\(`,
)

// --- #4737 Diesel / sea-orm receiver-typed read credit (ambiguous terminals) ---
//
// The terminals below collide with the Rust `Iterator` combinators on a plain
// Vec/slice/iterator (`vec.iter().filter(...).find(...)`, `slice.first()`,
// `it.map(...).all(...)`), so they are credited db_read ONLY when the receiver
// is known to hold a Diesel query (`users::table`, `.into_boxed()`, a
// `QueryDsl` chain) or a sea-orm `Select`/`Entity` (`User::find()`). On any
// other receiver they stay pure, preserving the false-positive guard.
const rustAmbiguousQueryVerbs = `first|find|filter|select|all|one|order|order_by|group_by|limit|offset|inner_join|left_join|on`

// rustQueryTypedRe seeds the set of Diesel-query / sea-orm-typed receiver names
// from the recurring query-root forms. Group 1 captures the typed local name:
//   - `let q = users::table;`                       (Diesel schema table root)
//   - `let q = users::table.filter(...)...;`        (Diesel QueryDsl chain root)
//   - `let q = posts::table.into_boxed();`          (Diesel boxed query)
//   - `let q = User::find();` / `let q = User::find().filter(...);`  (sea-orm)
//   - `let q: Select<User> = ...;` / `let q: BoxedQuery<...> = ...;` (annotation)
var rustQueryTypedRe = regexp.MustCompile(
	`(?m)\blet\s+(?:mut\s+)?([A-Za-z_]\w*)\s*` +
		`(?::\s*(?:Select|SelectTwo|BoxedQuery|BoxedSelectStatement|SelectStatement|FindStatement)\b[^=]*)?` +
		`=\s*(?:` +
		`[A-Za-z_]\w*\s*::\s*table\b` + // Diesel: schema::table[. … ]
		`|[A-Za-z_]\w*\s*::\s*find\s*\(` + // sea-orm: Entity::find(
		`|[A-Za-z_]\w*\s*::\s*find_by_id\s*\(` + // sea-orm: Entity::find_by_id(
		`)`,
)

// rustQueryRootInlineRe credits an ambiguous terminal invoked DIRECTLY off a
// query root inline — `users::table.filter(...)`, `User::find().all(&db)`,
// `posts::table.order(id).first(conn)`. The root must be a Diesel `schema::table`
// or a sea-orm `Entity::find(...)`; the terminal must be the ambiguous set
// (distinctive terminals are already covered bare by rustDBReadRe). A leading
// `\b` keeps `something.users::table` (impossible) and plain `vec.filter` out.
var rustQueryRootInlineRe = regexp.MustCompile(
	`\b[A-Za-z_]\w*\s*::\s*(?:table\b|find(?:_by_id)?\s*\([^;]*?\))` +
		`(?:\s*\.\s*[A-Za-z_]\w*\s*\([^;]*?\))*?` +
		`\s*\.\s*(?:` + rustAmbiguousQueryVerbs + `)\s*\(`,
)

// rustSqlxSelectRe matches sqlx `query!("SELECT ...")` literals.
var rustSqlxSelectRe = regexp.MustCompile(
	`\bsqlx\s*::\s*query(?:_as|_scalar)?(?:_unchecked)?\s*!?\s*\(\s*r?"(?i:\s*(?:SELECT|WITH)\b)`,
)

// rustDBWriteRe matches sqlx / Diesel / sea-orm write primitives.
var rustDBWriteRe = regexp.MustCompile(
	`\.\s*(?:execute|execute_many)\s*\(` +
		`|\bdiesel\s*::\s*(?:insert_into|update|delete|insert_or_ignore_into|replace_into)\b` +
		`|\.\s*(?:insert|update|delete|save|save_active|insert_many|update_many|exec|exec_with_returning)\s*\(`,
)

// rustSqlxWriteRe matches sqlx `query!("INSERT/UPDATE/DELETE ...")`.
var rustSqlxWriteRe = regexp.MustCompile(
	`\bsqlx\s*::\s*query(?:_as|_scalar)?(?:_unchecked)?\s*!?\s*\(\s*r?"(?i:\s*(?:INSERT|UPDATE|DELETE|REPLACE|MERGE|TRUNCATE)\b)`,
)

// rustFSReadRe matches read-only filesystem primitives.
var rustFSReadRe = regexp.MustCompile(
	`\b(?:std|tokio)\s*::\s*fs\s*::\s*(?:read|read_to_string|read_dir|read_link|metadata|symlink_metadata|canonicalize)\s*\(` +
		`|\bFile\s*::\s*open\s*\(` +
		`|\bOpenOptions\s*::\s*new\s*\(\s*\)\s*\.\s*read\s*\(\s*true\b`,
)

// rustFSWriteRe matches write filesystem primitives.
var rustFSWriteRe = regexp.MustCompile(
	`\b(?:std|tokio)\s*::\s*fs\s*::\s*(?:write|create_dir|create_dir_all|remove_file|remove_dir|remove_dir_all|rename|set_permissions|copy|hard_link|symlink|symlink_dir|symlink_file)\s*\(` +
		`|\bFile\s*::\s*create\s*\(` +
		`|\bOpenOptions\s*::\s*new\s*\(\s*\)\s*\.\s*(?:write|append|create)\s*\(\s*true\b`,
)

// rustProcessRe matches process-spawn primitives (modelled as fs_write).
var rustProcessRe = regexp.MustCompile(
	`\bstd\s*::\s*process\s*::\s*Command\s*::\s*new\b` +
		`|\btokio\s*::\s*process\s*::\s*Command\s*::\s*new\b` +
		`|\bCommand\s*::\s*new\s*\([^)]+\)\s*\.\s*(?:arg|args|spawn|output|status)\b`,
)

// rustMutationRe matches `self.field = ...` assignment.
var rustMutationRe = regexp.MustCompile(
	`\bself\s*\.\s*[A-Za-z_][\w]*\s*=(?:[^=])`,
)

func sniffEffectsRust(content string) []EffectMatch {
	if content == "" {
		return nil
	}
	headers := scanRustFuncHeaders(content)
	var out []EffectMatch
	out = appendRustMatches(out, content, headers, rustHTTPRe, EffectHTTPOut, "reqwest/hyper/surf", 1.0)
	out = appendRustMatches(out, content, headers, rustDBReadRe, EffectDBRead, "sqlx/diesel/sea-orm.read", 0.85)
	out = append(out, rustQueryReadMatches(content, headers)...)
	out = appendRustMatches(out, content, headers, rustSqlxSelectRe, EffectDBRead, "sqlx::query!(SELECT)", 1.0)
	out = appendRustMatches(out, content, headers, rustDBWriteRe, EffectDBWrite, "sqlx/diesel/sea-orm.write", 0.85)
	out = appendRustMatches(out, content, headers, rustSqlxWriteRe, EffectDBWrite, "sqlx::query!(WRITE)", 1.0)
	out = appendRustMatches(out, content, headers, rustFSReadRe, EffectFSRead, "std::fs::read/File::open", 1.0)
	out = appendRustMatches(out, content, headers, rustFSWriteRe, EffectFSWrite, "std::fs::write/File::create", 1.0)
	out = appendRustMatches(out, content, headers, rustProcessRe, EffectFSWrite, "process::Command", 0.9)
	out = appendRustMatches(out, content, headers, rustMutationRe, EffectMutation, "self.field=", 0.7)
	return out
}

// rustQueryReadMatches implements the #4737 receiver-typed read credit for
// Diesel / sea-orm. It (1) collects the set of Diesel-query / sea-orm-typed
// receiver names (locals assigned from a `schema::table` root, an `Entity::find`
// call, or a typed annotation, propagated across `let q2 = q.filter(...)` chains
// to a fixpoint), then (2) emits db_read for each ambiguous terminal invoked on
// one of those typed names, AND for ambiguous terminals chained directly off a
// query root inline (`users::table.filter(...).first(conn)`). An ambiguous
// terminal on an UNTYPED receiver (a plain Vec/slice/iterator) earns no credit —
// the Iterator-combinator false-positive guard is preserved.
func rustQueryReadMatches(content string, headers []funcHeader) []EffectMatch {
	typed := collectRustQueryTypedNames(content)
	var out []EffectMatch
	emit := func(off int) {
		line := lineOfOffset(content, off)
		out = append(out, EffectMatch{
			Function:   nearestHeader(headers, line),
			Line:       line,
			Effect:     EffectDBRead,
			Sink:       "diesel/sea-orm.read.query",
			Confidence: 0.85,
		})
	}
	for name := range typed {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\.\s*(?:` + rustAmbiguousQueryVerbs + `)\s*\(`)
		for _, m := range re.FindAllStringIndex(content, -1) {
			emit(m[0])
		}
	}
	for _, m := range rustQueryRootInlineRe.FindAllStringIndex(content, -1) {
		emit(m[0])
	}
	return out
}

// collectRustQueryTypedNames returns the set of names known to hold a Diesel
// query / sea-orm Select. Seeds from rustQueryTypedRe (schema::table root,
// Entity::find call, typed annotation), then iterates the
// `let <dst> = <src>.<query-op>(...)` form to a fixpoint so a query bound from an
// already-typed receiver (`let q2 = q.filter(...)`) is itself typed.
func collectRustQueryTypedNames(content string) map[string]bool {
	typed := map[string]bool{}
	for _, m := range rustQueryTypedRe.FindAllStringSubmatch(content, -1) {
		if len(m) >= 2 && m[1] != "" {
			typed[m[1]] = true
		}
	}
	// Fixpoint: `let <dst> = <src>.<query-op>(...)` where <src> is typed.
	chainRe := regexp.MustCompile(
		`(?m)\blet\s+(?:mut\s+)?([A-Za-z_]\w*)\s*=\s*([A-Za-z_]\w*)\s*\.\s*` +
			`(?:filter|select|order|order_by|group_by|limit|offset|inner_join|left_join|into_boxed|distinct|for_update)\b`)
	chains := chainRe.FindAllStringSubmatch(content, -1)
	for {
		changed := false
		for _, m := range chains {
			if len(m) < 3 {
				continue
			}
			if typed[m[2]] && !typed[m[1]] {
				typed[m[1]] = true
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return typed
}

func scanRustFuncHeaders(content string) []funcHeader {
	var hs []funcHeader
	for _, m := range rustFuncHeaderRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		hs = append(hs, funcHeader{Line: lineOfOffset(content, m[0]), Name: content[m[2]:m[3]]})
	}
	return hs
}

func appendRustMatches(out []EffectMatch, content string, headers []funcHeader, re *regexp.Regexp, eff Effect, sink string, conf float64) []EffectMatch {
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
