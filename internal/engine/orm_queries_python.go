// Python ORM matchers for applyORMQueries (#723).
//
// Covers:
//   - Django ORM    : `Model.objects.<verb>(...)`
//     `<qs>.filter(...).filter(...)` chains are attributed
//     to the originating Model when statically visible
//     (only the first link is required — subsequent links
//     are inferred to share the same target).
//   - SQLAlchemy    : `session.query(Model)` (Classic API)
//     `select(Model).where(...)` (2.0 / async style)
//     `session.execute(select(Model)...)` — handled by
//     the same select(Model) matcher
//   - Tortoise ORM  : `Model.filter(...)` / `Model.get(...)` / `Model.all()`
//     (recognised by capitalised target + Tortoise-style verbs)
//   - Peewee        : `Model.select()` / `Model.get(...)` (overlaps with
//     Tortoise — both share the capitalised-target shape)
//
// Out of phase 1:
//   - SQLAlchemy ORM relationships (`User.addresses` traversal -> is_join)
//   - Tortoise prefetch_related / select_related → is_join
//   - Peewee join() chains → is_join
package engine

import (
	"regexp"
	"strings"
)

// Django: `<Model>.objects.<verb>(...)`. The model name is anchored to a
// capitalised identifier to avoid matching variable receivers like
// `self.objects.get`.
var pyDjangoOrmRe = regexp.MustCompile(
	`\b([A-Z][A-Za-z0-9_]*)\.objects\.(get|filter|all|create|update|delete|exclude|annotate|aggregate|count|first|last|exists|get_or_create|update_or_create|bulk_create|bulk_update|values|values_list|order_by|prefetch_related|select_related|none|raw|in_bulk)\s*\(`,
)

// SQLAlchemy classic: `session.query(<Model>)` — accepts any leading
// receiver identifier (`session`, `self.session`, `db.session`, etc.).
var pySAQueryRe = regexp.MustCompile(
	`\b(?:[A-Za-z_]\w*\.)*query\s*\(\s*([A-Z][A-Za-z0-9_]*)\s*[,)]`,
)

// SQLAlchemy 2.0: `select(<Model>)` followed by optional `.where(...)`
// chain. Used in both sync and async forms (`await session.execute(
// select(User).where(...))`).
var pySASelectRe = regexp.MustCompile(
	`\bselect\s*\(\s*([A-Z][A-Za-z0-9_]*)\s*\)`,
)

// Tortoise / Peewee shared shape: `<Model>.<verb>(`. We restrict the verb
// to ORM-flavoured names so the matcher doesn't fire on every static
// method call. Verbs are the union of Tortoise and Peewee surface APIs;
// canonicalOp() flattens them to find/create/update/delete/aggregate.
var pyTortoisePeeweeRe = regexp.MustCompile(
	`\b([A-Z][A-Za-z0-9_]*)\.(select|get|filter|all|create|update|delete|insert|save|count|exists|first|annotate|prefetch_related|where|join)\s*\(`,
)

// Beanie (async MongoDB ODM): document-class queries `<Model>.<verb>(...)`.
// Beanie's surface is method calls on the Document subclass itself —
// `User.find(User.age > 18)`, `User.find_one(...)`, `User.get(id)`,
// `User.find_all()`, `User.aggregate([...])`, `User.insert_many([...])`,
// `User.count()`. The verb list is Beanie-specific (find_all / find_one /
// insert_many are not Tortoise/Peewee verbs) so canonicalOp() flattens it
// to find/create/aggregate/etc.
var pyBeanieRe = regexp.MustCompile(
	`\b([A-Z][A-Za-z0-9_]*)\.(find_one|find_all|find_many|find|get|aggregate|insert_many|insert|count|delete_all|delete)\s*\(`,
)

// MongoEngine (sync MongoDB ODM): the `objects` QuerySet manager is invoked
// either DIRECTLY as a call — `User.objects(name="x")` — or chained with a
// verb — `User.objects.filter(...)` / `.get(...)` / `.first()` / `.count()`
// / `.delete()` / `.update(...)`. The direct-call form is the idiom the
// Django matcher (which requires `.objects.<verb>`) does not cover, so this
// matcher anchors on `<Model>.objects` and inspects what follows.
var pyMongoEngineRe = regexp.MustCompile(
	`\b([A-Z][A-Za-z0-9_]*)\.objects\b`,
)

// pyMongoEngineVerbRe recovers the chained verb after `<Model>.objects.`
// when present (anchored at the char immediately after `.objects`).
var pyMongoEngineVerbRe = regexp.MustCompile(
	`^\.(get|filter|all|first|count|exists|delete|update|order_by|only|exclude|aggregate|create|insert|save)\b`,
)

// scanPythonORM walks `src` and emits QUERIES edges for every detected
// ORM call site.
func scanPythonORM(src string, funcs []funcSpan, emit emitORMQueryFn) {
	// MongoEngine reuses Django's `<Model>.objects.<verb>` chain shape but is
	// a MongoDB ODM, not Django. When a file imports mongoengine and NOT
	// django, route `<Model>.objects.*` exclusively through the MongoEngine
	// block below so the QUERIES edge is credited orm=mongoengine instead of
	// a fabricated orm=django (and never double-emitted).
	mongoEngineOnly := (strings.Contains(src, "from mongoengine") || strings.Contains(src, "import mongoengine")) &&
		!(strings.Contains(src, "from django") || strings.Contains(src, "import django"))

	// Django ORM
	if !mongoEngineOnly {
		for _, m := range pyDjangoOrmRe.FindAllStringSubmatchIndex(src, -1) {
			if len(m) < 6 {
				continue
			}
			model := src[m[2]:m[3]]
			verb := src[m[4]:m[5]]
			caller := enclosingFuncAt(funcs, m[0])
			argsBlob := extractCallArgs(src, m[5])
			filterKeys := parseFilterKeys(argsBlob)
			isJoin := pythonIsJoinDjango(verb, argsBlob)
			// Promote terminal chain verbs to the emitted operation. Django
			// idioms like `User.objects.filter(id=1).delete()` express the
			// intent on the trailing call, not the queryset builder. We scan
			// the tail of the statement for a recognised terminal verb and
			// override the canonical op when one is present.
			tail := lookAheadChain(src, m[5], 256)
			op := canonicalOp(verb)
			if t := promoteTerminalDjangoOp(tail); t != "" {
				op = t
			}
			emit(caller, model, op, filterKeys, "django", isJoin)
		}
	}

	// SQLAlchemy classic
	for _, m := range pySAQueryRe.FindAllStringSubmatchIndex(src, -1) {
		if len(m) < 4 {
			continue
		}
		model := src[m[2]:m[3]]
		caller := enclosingFuncAt(funcs, m[0])
		// Look ahead for a chained `.filter(...)` or `.where(...)` to
		// extract filter keys + join detection. Bounded by the nearest
		// newline-followed-by-non-whitespace to avoid spanning unrelated
		// statements.
		tail := lookAheadChain(src, m[1], 512)
		filterKeys := parseFilterKeys(tail)
		isJoin := strings.Contains(tail, ".join(") ||
			strings.Contains(tail, ".outerjoin(") ||
			strings.Contains(tail, "joinedload(")
		emit(caller, model, "find", filterKeys, "sqlalchemy", isJoin)
	}

	// SQLAlchemy 2.0 select()
	for _, m := range pySASelectRe.FindAllStringSubmatchIndex(src, -1) {
		if len(m) < 4 {
			continue
		}
		model := src[m[2]:m[3]]
		caller := enclosingFuncAt(funcs, m[0])
		tail := lookAheadChain(src, m[1], 512)
		filterKeys := parseFilterKeys(tail)
		isJoin := strings.Contains(tail, ".join(") ||
			strings.Contains(tail, ".outerjoin(") ||
			strings.Contains(tail, "joinedload(")
		op := "find"
		if strings.Contains(tail, "func.count") || strings.Contains(tail, "func.sum") {
			op = "aggregate"
		}
		emit(caller, model, op, filterKeys, "sqlalchemy", isJoin)
	}

	// Beanie (async MongoDB ODM). Gate on import-presence — the matcher is
	// broad (`<Capitalised>.<verb>(`) and would otherwise over-fire on
	// ordinary static-method calls. Beanie document queries target the
	// Document subclass directly, so the model name is the regex capture.
	if strings.Contains(src, "from beanie") || strings.Contains(src, "import beanie") {
		for _, m := range pyBeanieRe.FindAllStringSubmatchIndex(src, -1) {
			if len(m) < 6 {
				continue
			}
			model := src[m[2]:m[3]]
			verb := src[m[4]:m[5]]
			caller := enclosingFuncAt(funcs, m[0])
			argsBlob := extractCallArgs(src, m[5])
			filterKeys := parseFilterKeys(argsBlob)
			emit(caller, model, canonicalOp(verb), filterKeys, "beanie", false)
		}
	}

	// MongoEngine (sync MongoDB ODM). Gate on import-presence. The Django
	// matcher already covers `<Model>.objects.<verb>(...)` for a fixed verb
	// list but credits it as orm=django; MongoEngine needs its OWN provider
	// label plus the direct-call form `<Model>.objects(...)` that Django's
	// `.objects.<verb>` shape never matches. We anchor on `<Model>.objects`
	// and classify what follows.
	if strings.Contains(src, "from mongoengine") || strings.Contains(src, "import mongoengine") {
		for _, m := range pyMongoEngineRe.FindAllStringSubmatchIndex(src, -1) {
			if len(m) < 4 {
				continue
			}
			model := src[m[2]:m[3]]
			caller := enclosingFuncAt(funcs, m[0])
			rest := src[m[1]:] // text immediately after `<Model>.objects`
			switch {
			case strings.HasPrefix(rest, "("):
				// Direct manager call: `User.objects(name="x")` — a find with
				// the kwargs as filter keys.
				argsBlob := extractCallArgs(src, m[1])
				filterKeys := parseFilterKeys(argsBlob)
				emit(caller, model, "find", filterKeys, "mongoengine", false)
			default:
				// Chained verb: `User.objects.filter(...)` etc.
				vm := pyMongoEngineVerbRe.FindStringSubmatchIndex(rest)
				if vm == nil {
					continue
				}
				verb := rest[vm[2]:vm[3]]
				// Locate the verb's call args: the verb sits at rest[vm[2]:vm[3]]
				// which maps to src offset m[1]+vm[3]; args start there.
				argsBlob := extractCallArgs(src, m[1]+vm[3])
				filterKeys := parseFilterKeys(argsBlob)
				op := canonicalOp(verb)
				// Promote a terminal CRUD verb on the chain, mirroring Django:
				// `Article.objects.filter(...).delete()` expresses delete intent
				// on the trailing call, not the queryset builder.
				tail := lookAheadChain(src, m[1]+vm[3], 256)
				if t := promoteTerminalDjangoOp(tail); t != "" {
					op = t
				}
				emit(caller, model, op, filterKeys, "mongoengine", false)
			}
		}
	}

	// Tortoise / Peewee. Gate on import-presence: the matcher is broad
	// (`<Capitalised>.<verb>(`) and would over-fire on Django stdlib code
	// that uses methods like `AppConfig.create(...)`. Restrict it to files
	// that visibly import Tortoise or Peewee — that's enough to recover
	// the dominant cases while avoiding cross-language false positives.
	if !(strings.Contains(src, "from tortoise") || strings.Contains(src, "import tortoise") ||
		strings.Contains(src, "from peewee") || strings.Contains(src, "import peewee")) {
		return
	}
	for _, m := range pyTortoisePeeweeRe.FindAllStringSubmatchIndex(src, -1) {
		if len(m) < 6 {
			continue
		}
		// Skip overlap with the Django matcher above: `<Model>.objects.X(`
		// would also match if `objects` were a verb, but our verb list
		// doesn't include `objects`, so no de-dup needed here. We DO need
		// to skip cases where the leading dot is preceded by `.objects`
		// (chained QuerySet methods on a Django queryset).
		start := m[0]
		if start >= 8 && src[start-8:start] == ".objects" {
			continue
		}
		model := src[m[2]:m[3]]
		verb := src[m[4]:m[5]]
		caller := enclosingFuncAt(funcs, m[0])
		argsBlob := extractCallArgs(src, m[5])
		filterKeys := parseFilterKeys(argsBlob)
		emit(caller, model, canonicalOp(verb), filterKeys, "tortoise_peewee", false)
	}
}

// promoteTerminalDjangoOp scans a chain tail like
// `.filter(...).update(name="x")` and returns the canonical op of the
// LAST chain link when it's a CRUD verb. Returns "" when the chain has
// no recognised terminal CRUD call.
func promoteTerminalDjangoOp(tail string) string {
	out := ""
	for _, kw := range []struct{ name, op string }{
		{".delete(", "delete"},
		{".update(", "update"},
		{".create(", "create"},
		{".bulk_create(", "create"},
		{".bulk_update(", "update"},
		{".save(", "update"},
	} {
		if idx := strings.LastIndex(tail, kw.name); idx >= 0 {
			// Only promote when the terminal call sits AFTER any
			// intermediate ones (longest-suffix wins). Track the rightmost
			// match.
			if out == "" || idx > strings.LastIndex(tail, "."+out+"(") {
				out = kw.op
			}
		}
	}
	return out
}

// pythonIsJoinDjango reports whether a Django ORM verb + args blob looks
// like a relation-traversing query. Heuristic: any double-underscore key
// in the kwargs (Django's relation traversal notation), or use of
// select_related/prefetch_related.
func pythonIsJoinDjango(verb, args string) bool {
	if verb == "select_related" || verb == "prefetch_related" {
		return true
	}
	return strings.Contains(args, "__")
}

// lookAheadChain returns up to `limit` chars of text starting at `pos`,
// stopping at the first newline that is followed by a non-whitespace
// character (a coarse statement-boundary heuristic).
func lookAheadChain(src string, pos, limit int) string {
	end := pos + limit
	if end > len(src) {
		end = len(src)
	}
	tail := src[pos:end]
	// Scan for newline+non-whitespace.
	for i := 0; i < len(tail)-1; i++ {
		if tail[i] == '\n' {
			j := i + 1
			for j < len(tail) && (tail[j] == ' ' || tail[j] == '\t') {
				j++
			}
			if j < len(tail) && tail[j] != '.' && tail[j] != ')' {
				return tail[:i]
			}
		}
	}
	return tail
}

// extractCallArgs locates the next `(` at or after `start` and returns
// the balanced argument substring (without the surrounding parens), or
// "" when no paren is found within a short scan window. This is the
// helper used by every per-language ORM scanner to recover the call
// args for filter-key parsing.
func extractCallArgs(src string, start int) string {
	if start < 0 || start >= len(src) {
		return ""
	}
	// Search at most 16 bytes ahead for the opening paren — the matcher
	// regexes always anchor `\s*\(` immediately after the verb capture,
	// so a larger window risks accidentally matching the next statement.
	end := start + 16
	if end > len(src) {
		end = len(src)
	}
	idx := strings.IndexByte(src[start:end], '(')
	if idx < 0 {
		return ""
	}
	return matchCall(src, start+idx, 4096)
}
