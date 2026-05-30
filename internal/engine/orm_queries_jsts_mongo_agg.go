// MongoDB aggregation-pipeline internal extraction for JS/TS (#3426).
//
// orm_queries_jsts.go and orm_queries_jsts_drivers.go already DETECT the
// `.aggregate([...])` call itself as an aggregate QUERIES edge (mongoose
// `Model.aggregate(...)`, raw driver `db.collection('c').aggregate(...)`),
// but they treat the pipeline array as an opaque argument blob. The pipeline
// is where the actual data-flow lives: a `$lookup` stage is an implicit
// cross-collection JOIN that the migration needs to reason about, and each
// stage is a distinct transformation step worth representing as a node.
//
// This pass is a SIBLING to the existing aggregate detection: it does NOT
// re-emit the aggregate QUERIES edge. Instead it locates the same call sites,
// parses the inline pipeline array literal with a brace/bracket-depth,
// string-aware scanner, and emits:
//
//  1. JOINS_COLLECTION relationship — for every `$lookup` / `$graphLookup`
//     stage, an edge from the aggregating collection/model → the `from`
//     collection. Properties: local_field, foreign_field, as, stage.
//     This is the highest-value output: the implicit application-side join
//     MongoDB has no schema FK for.
//
//  2. SCOPE.DataAccess pipeline-stage entities — one per stage, anchored at
//     the aggregate call site, with subtype = the stage operator
//     ($match, $group, $unwind, $facet, $project, $sort, $limit, $addFields,
//     $lookup, $graphLookup). Stage ORDER is preserved as a stage_index
//     property. Selected stages capture extra structure:
//     - $group:  the `_id` expression + accumulator field names
//     - $facet:  the named sub-pipeline keys
//
// HONEST LIMIT: only INLINE array literals are parsed. A pipeline built
// dynamically (passed as a variable, assembled by `.push()`, or spread from
// another array) is left unresolved — we never fabricate stages we cannot
// see. The receiver resolution is likewise local: `Model.aggregate(...)`
// uses the capitalised model name, `db.collection('c').aggregate(...)` uses
// the collection-string argument; an aggregate on an unrecognised receiver
// shape is skipped.
package engine

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cajasmota/archigraph/internal/types"
)

// mongoAggPatternType tags every entity/edge this pass emits.
const mongoAggPatternType = "mongo_aggregation"

// mongoAggStageEntityKind is the entity Kind used for a single pipeline
// stage. SCOPE.DataAccess models a data-access operation; the stage operator
// lives in Subtype.
var mongoAggStageEntityKind = string(types.EntityKindDataAccess)

// mongoAggJoinEdgeKind is the cross-collection join edge emitted for
// $lookup / $graphLookup.
var mongoAggJoinEdgeKind = string(types.RelationshipKindJoinsCollection)

// mongoAggCallRe locates `.aggregate(` call sites whose first argument opens
// an inline array literal. Two receiver shapes are recognised:
//
//	<Model>.aggregate([ ...        — capitalised model (mongoose / sequelize)
//	.collection('c')...aggregate([ — native driver (collection arg captured
//	                                 separately by mongoAggCollectionArgRe)
//
// The regex only anchors the `.aggregate(` token + optional leading `[`; the
// receiver is recovered by scanning leftward from the match so we can handle
// both `Model.aggregate` and the chained `db.collection('c').aggregate`.
var mongoAggCallRe = regexp.MustCompile(`\.aggregate\s*\(`)

// mongoAggModelRe pulls a capitalised model identifier immediately preceding
// `.aggregate`. Used when the receiver is a mongoose/sequelize Model.
var mongoAggModelRe = regexp.MustCompile(`([A-Z][A-Za-z0-9_$]*)\s*$`)

// mongoAggCollectionArgRe pulls the collection name out of the nearest
// preceding `.collection('c')` on the same chain (native driver).
var mongoAggCollectionArgRe = regexp.MustCompile(
	`\.collection\(\s*['"` + "`" + `]([a-zA-Z_][\w$.]*)['"` + "`" + `]\s*\)`,
)

// scanJSMongoAggregation walks `src`, finds `.aggregate([...])` call sites
// with an inline pipeline array, parses the pipeline, and emits stage
// entities + cross-collection join edges via the supplied appenders.
//
// emitStage(name, kind, subtype, line, props) appends a pipeline-stage
// entity. emitJoin(fromColl, toColl, props) appends a JOINS_COLLECTION edge.
func scanJSMongoAggregation(
	src string,
	funcs []funcSpan,
	path string,
	lang string,
	emitStage func(ent types.EntityRecord),
	emitJoin func(rel types.RelationshipRecord),
) {
	// Gate: only run where a mongo surface is plausible. Reuse the same
	// signals the existing detectors use (mongoose OR native driver), so we
	// don't scan arbitrary `.aggregate(` chains (e.g. RxJS, lodash/fp).
	if !mentionsMongooseSequelize(src) && !mentionsMongoDriver(src) {
		return
	}

	for _, loc := range mongoAggCallRe.FindAllStringIndex(src, -1) {
		openParen := loc[1] - 1 // index of '('
		// Resolve the aggregating collection/model from the receiver.
		coll := mongoAggResolveReceiver(src, loc[0])
		if coll == "" {
			continue
		}
		// The first argument must open an inline array literal `[`.
		arrStart := mongoAggSkipToArray(src, openParen)
		if arrStart < 0 {
			continue // dynamic pipeline (variable / spread) — honest skip.
		}
		stages := mongoAggSplitStages(src, arrStart)
		if len(stages) == 0 {
			continue
		}
		caller := enclosingFuncAt(funcs, loc[0])
		callLine := lineOfOffset(src, loc[0])

		for idx, st := range stages {
			op := mongoAggFirstKey(st)
			if op == "" {
				continue
			}
			props := map[string]string{
				"pattern_type": mongoAggPatternType,
				"collection":   coll,
				"stage_index":  itoa(idx),
				"stage":        op,
			}
			if caller != "" {
				props["caller"] = caller
			}

			switch op {
			case "$lookup":
				lk := mongoAggParseLookup(st)
				if lk.from != "" {
					props["from"] = lk.from
					if lk.localField != "" {
						props["local_field"] = lk.localField
					}
					if lk.foreignField != "" {
						props["foreign_field"] = lk.foreignField
					}
					if lk.as != "" {
						props["as"] = lk.as
					}
					emitJoin(mongoAggJoinEdge(coll, lk, "lookup"))
				}
			case "$graphLookup":
				lk := mongoAggParseLookup(st)
				if lk.from != "" {
					props["from"] = lk.from
					if lk.as != "" {
						props["as"] = lk.as
					}
					emitJoin(mongoAggJoinEdge(coll, lk, "graphLookup"))
				}
			case "$group":
				id, accs := mongoAggParseGroup(st)
				if id != "" {
					props["group_id"] = id
				}
				if accs != "" {
					props["accumulators"] = accs
				}
			case "$facet":
				if keys := mongoAggParseFacetKeys(st); keys != "" {
					props["facets"] = keys
				}
			}

			name := fmt.Sprintf("%s.aggregate#%d %s", coll, idx, op)
			emitStage(types.EntityRecord{
				Name:       name,
				Kind:       mongoAggStageEntityKind,
				Subtype:    op,
				SourceFile: path,
				StartLine:  callLine,
				EndLine:    callLine,
				Language:   lang,
				Properties: props,

				EnrichmentRequired: false,
				EnrichmentStatus:   types.StatusPending,
				QualityScore:       0.8,
			})
		}
	}
}

// mongoAggResolveReceiver recovers the aggregating collection/model name from
// the text immediately preceding the `.aggregate` token at `dotPos`.
//
//	db.collection('orders').aggregate(...)  → "orders" (native driver)
//	Order.aggregate(...)                    → "Order"  (mongoose model)
//
// Native-driver `.collection('c')` wins when present on the same chain (it is
// the authoritative collection name); otherwise we fall back to a capitalised
// model identifier. Returns "" when neither shape is recognised.
func mongoAggResolveReceiver(src string, dotPos int) string {
	// Look back over a bounded window for a `.collection('c')` on the chain.
	winStart := dotPos - 200
	if winStart < 0 {
		winStart = 0
	}
	window := src[winStart:dotPos]
	if cm := mongoAggCollectionArgRe.FindAllStringSubmatch(window, -1); len(cm) > 0 {
		// Nearest (last) collection() on the chain.
		return cm[len(cm)-1][1]
	}
	// Fall back to a capitalised model identifier directly before `.aggregate`.
	// dotPos points at the '.', so scan the identifier ending at dotPos.
	if mm := mongoAggModelRe.FindStringSubmatch(src[winStart:dotPos]); mm != nil {
		return mm[1]
	}
	return ""
}

// mongoAggSkipToArray returns the index of the `[` that opens the pipeline
// array literal, given the `(` of the aggregate call. It skips only
// whitespace between `(` and `[`; if the first non-space token is not `[`
// (e.g. a variable name or spread), the pipeline is dynamic and we return -1.
func mongoAggSkipToArray(src string, openParen int) int {
	i := openParen + 1
	for i < len(src) {
		c := src[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		if c == '[' {
			return i
		}
		return -1
	}
	return -1
}

// mongoAggSplitStages splits the pipeline array literal starting at `arrStart`
// (the `[`) into its top-level stage substrings. It is string-aware (handles
// '/"/` quotes with escapes) and depth-aware (only splits on commas at array
// depth 1 / brace depth 0), so nested objects, nested arrays, and quoted
// commas don't break the split. Trailing commas yield no empty stage.
func mongoAggSplitStages(src string, arrStart int) []string {
	if arrStart >= len(src) || src[arrStart] != '[' {
		return nil
	}
	var stages []string
	depthBracket := 0 // []
	depthBrace := 0   // {}
	depthParen := 0   // ()
	inStr := byte(0)
	segStart := -1

	flush := func(end int) {
		if segStart < 0 {
			return
		}
		seg := strings.TrimSpace(src[segStart:end])
		if seg != "" {
			stages = append(stages, seg)
		}
		segStart = -1
	}

	for i := arrStart; i < len(src); i++ {
		c := src[i]
		if inStr != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == inStr {
				inStr = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			inStr = c
		case '[':
			depthBracket++
			if depthBracket == 1 {
				// opening of the pipeline array itself
				segStart = i + 1
			}
		case ']':
			depthBracket--
			if depthBracket == 0 {
				flush(i)
				return stages
			}
		case '{':
			depthBrace++
		case '}':
			depthBrace--
		case '(':
			depthParen++
		case ')':
			depthParen--
		case ',':
			// Split only at top level of the pipeline array.
			if depthBracket == 1 && depthBrace == 0 && depthParen == 0 {
				flush(i)
				segStart = i + 1
			}
		}
	}
	// Unterminated array — return whatever we resolved.
	return stages
}

// mongoAggFirstKey returns the first object key of a stage substring, e.g.
// "$match" from `{ $match: { ... } }`. It is string- and depth-aware so a key
// inside a nested object is never mistaken for the stage operator. Returns ""
// if no top-level key is found.
func mongoAggFirstKey(stage string) string {
	// Find the opening brace of the stage object.
	i := 0
	for i < len(stage) && stage[i] != '{' {
		i++
	}
	if i >= len(stage) {
		return ""
	}
	i++ // past '{'
	// Skip whitespace.
	for i < len(stage) && (stage[i] == ' ' || stage[i] == '\t' || stage[i] == '\n' || stage[i] == '\r') {
		i++
	}
	if i >= len(stage) {
		return ""
	}
	// Quoted key?
	if stage[i] == '\'' || stage[i] == '"' || stage[i] == '`' {
		q := stage[i]
		i++
		start := i
		for i < len(stage) && stage[i] != q {
			if stage[i] == '\\' {
				i++
			}
			i++
		}
		return stage[start:i]
	}
	// Bare key (identifier, possibly `$`-prefixed).
	start := i
	for i < len(stage) && (isMongoKeyChar(stage[i])) {
		i++
	}
	return stage[start:i]
}

func isMongoKeyChar(c byte) bool {
	return c == '$' || c == '_' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// mongoAggLookup holds the join-relevant fields pulled from a $lookup /
// $graphLookup stage.
type mongoAggLookup struct {
	from         string
	localField   string
	foreignField string
	as           string
}

// mongoAggParseLookup extracts the join fields from a $lookup / $graphLookup
// stage. It handles both the classic `{ from, localField, foreignField, as }`
// form and the `{ from, pipeline: [...], as }` sub-pipeline form (we still
// recover `from` + `as`). String values may be single/double/backtick quoted.
func mongoAggParseLookup(stage string) mongoAggLookup {
	return mongoAggLookup{
		from:         mongoAggStringField(stage, "from"),
		localField:   mongoAggStringField(stage, "localField"),
		foreignField: mongoAggStringField(stage, "foreignField"),
		as:           mongoAggStringField(stage, "as"),
	}
}

// mongoAggStringField pulls the quoted string value of `key` from a stage
// substring: `from: 'orders'` / `"from": "orders"` → "orders". Returns "" if
// the key is absent or its value is not a plain string literal (e.g. an
// expression / variable — which we honestly cannot resolve statically).
func mongoAggStringField(stage, key string) string {
	re := regexp.MustCompile(
		`(?:\b` + regexp.QuoteMeta(key) + `\b|['"` + "`" + `]` + regexp.QuoteMeta(key) +
			`['"` + "`" + `])\s*:\s*['"` + "`" + `]([a-zA-Z_$][\w$.]*)['"` + "`" + `]`,
	)
	m := re.FindStringSubmatch(stage)
	if m == nil {
		return ""
	}
	return m[1]
}

// mongoAggJoinEdge builds the JOINS_COLLECTION relationship from the
// aggregating collection to the looked-up `from` collection.
func mongoAggJoinEdge(fromColl string, lk mongoAggLookup, stageName string) types.RelationshipRecord {
	props := map[string]string{
		"pattern_type": mongoAggPatternType,
		"stage":        stageName,
	}
	if lk.localField != "" {
		props["local_field"] = lk.localField
	}
	if lk.foreignField != "" {
		props["foreign_field"] = lk.foreignField
	}
	if lk.as != "" {
		props["as"] = lk.as
	}
	return types.RelationshipRecord{
		FromID:     fmt.Sprintf("Class:%s", capitalisedSingular(fromColl)),
		ToID:       fmt.Sprintf("Class:%s", capitalisedSingular(lk.from)),
		Kind:       mongoAggJoinEdgeKind,
		Properties: props,
	}
}

// mongoAggParseGroup extracts the `_id` expression text and the accumulator
// field names from a $group stage. The `_id` value can be a string
// (`_id: '$status'`), an object (`_id: { y: '$year' }`), or null; we capture a
// compact text form. Accumulators are the OTHER top-level keys of the group
// object (total, count, …). Returns (idText, "field1,field2,...").
func mongoAggParseGroup(stage string) (idText string, accumulators string) {
	body := mongoAggStageBody(stage, "$group")
	if body == "" {
		return "", ""
	}
	keys := mongoAggTopLevelKeys(body)
	var accs []string
	for _, kv := range keys {
		if kv.key == "_id" {
			idText = strings.TrimSpace(kv.val)
			// Collapse internal whitespace for a compact, stable property.
			idText = mongoAggCollapseWS(idText)
			continue
		}
		accs = append(accs, kv.key)
	}
	return idText, strings.Join(accs, ",")
}

// mongoAggParseFacetKeys returns the comma-joined named sub-pipeline keys of a
// $facet stage (e.g. "byStatus,byMonth").
func mongoAggParseFacetKeys(stage string) string {
	body := mongoAggStageBody(stage, "$facet")
	if body == "" {
		return ""
	}
	keys := mongoAggTopLevelKeys(body)
	var names []string
	for _, kv := range keys {
		names = append(names, kv.key)
	}
	return strings.Join(names, ",")
}

// mongoAggStageBody returns the substring inside the `{...}` value of the
// named stage operator, e.g. for `{ $group: { _id: ..., total: ... } }` and
// op "$group" it returns `_id: ..., total: ...`. String- and depth-aware.
func mongoAggStageBody(stage, op string) string {
	// Locate `op` followed by `:` then `{`.
	idx := strings.Index(stage, op)
	if idx < 0 {
		return ""
	}
	i := idx + len(op)
	for i < len(stage) && stage[i] != ':' {
		i++
	}
	if i >= len(stage) {
		return ""
	}
	i++ // past ':'
	for i < len(stage) && (stage[i] == ' ' || stage[i] == '\t' || stage[i] == '\n' || stage[i] == '\r') {
		i++
	}
	if i >= len(stage) || stage[i] != '{' {
		return ""
	}
	// Balanced-brace, string-aware scan for the matching '}'.
	depth := 0
	inStr := byte(0)
	bodyStart := i + 1
	for ; i < len(stage); i++ {
		c := stage[i]
		if inStr != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == inStr {
				inStr = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			inStr = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return stage[bodyStart:i]
			}
		}
	}
	return ""
}

// mongoAggKV is a top-level key/value pair within an object body.
type mongoAggKV struct {
	key string
	val string
}

// mongoAggTopLevelKeys splits an object body (the text between the outer
// braces) into its top-level key/value pairs. String- and depth-aware so
// nested objects/arrays are kept whole inside the value. Keys may be quoted
// or bare.
func mongoAggTopLevelKeys(body string) []mongoAggKV {
	var out []mongoAggKV
	i := 0
	n := len(body)
	for i < n {
		// Skip leading separators/whitespace.
		for i < n && (body[i] == ',' || body[i] == ' ' || body[i] == '\t' || body[i] == '\n' || body[i] == '\r') {
			i++
		}
		if i >= n {
			break
		}
		// Parse key.
		var key string
		if body[i] == '\'' || body[i] == '"' || body[i] == '`' {
			q := body[i]
			i++
			start := i
			for i < n && body[i] != q {
				if body[i] == '\\' {
					i++
				}
				i++
			}
			key = body[start:i]
			if i < n {
				i++ // past closing quote
			}
		} else {
			start := i
			for i < n && isMongoKeyChar(body[i]) {
				i++
			}
			key = body[start:i]
		}
		// Skip to ':'.
		for i < n && body[i] != ':' {
			i++
		}
		if i >= n {
			break
		}
		i++ // past ':'
		// Capture value up to the top-level comma.
		valStart := i
		depthBrace, depthBracket, depthParen := 0, 0, 0
		inStr := byte(0)
		for i < n {
			c := body[i]
			if inStr != 0 {
				if c == '\\' {
					i += 2
					continue
				}
				if c == inStr {
					inStr = 0
				}
				i++
				continue
			}
			switch c {
			case '\'', '"', '`':
				inStr = c
			case '{':
				depthBrace++
			case '}':
				depthBrace--
			case '[':
				depthBracket++
			case ']':
				depthBracket--
			case '(':
				depthParen++
			case ')':
				depthParen--
			case ',':
				if depthBrace == 0 && depthBracket == 0 && depthParen == 0 {
					goto done
				}
			}
			i++
		}
	done:
		val := strings.TrimSpace(body[valStart:i])
		if key != "" {
			out = append(out, mongoAggKV{key: key, val: val})
		}
		// i is at the comma (or n); loop's leading skip advances past it.
	}
	return out
}

// mongoAggCollapseWS replaces runs of whitespace with a single space so a
// multi-line `_id` object becomes a compact, stable one-line property value.
func mongoAggCollapseWS(s string) string {
	var b strings.Builder
	prevSpace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteByte(c)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}
