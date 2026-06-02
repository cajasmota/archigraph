package links

// confidence.go centralises the per-pass confidence scoring functions.
//
// The three passes emit cross-repo link candidates with a `confidence`
// float in [0, 1]. The bands per the spec are:
//
//   P1 (import_pass)  — structural, highest signal       → 0.9 .. 1.0
//   P2 (label_pass)   — TF-IDF + kind compatibility      → 0.6 .. 0.8
//   P3 (string_pass)  — string-pattern catalog match     → 0.3 .. 0.6
//
// The functions below are the single source of truth for these scores.
// They are intentionally pure (no I/O, no clock) so unit tests can pin
// every category to an expected range.
//
// Heuristics:
//   - P1 has no degrees of freedom: the indexer already produced a
//     resolved cross-repo edge, so confidence is 1.0.
//   - P2 starts from `idf * kindCompat` (computed by the pass) and
//     squashes the result into the medium band [0.6, 0.8]. Below the
//     pass's link-threshold the raw value flows straight through (it
//     becomes a candidate, not a link).
//   - P3 maps each pattern category to a base score reflecting how
//     specific that category is. ARNs and SQS URLs encode an account
//     id and region and so have the highest base; bare Redis keys are
//     the most ambiguous and sit at the bottom of the band.
//
// Keeping the maths here (rather than scattered across the pass files)
// makes it easy to tune the bands later without disturbing the pair
// iteration logic.

const (
	// ImportConfidence is the fixed score for every P1 link.
	ImportConfidence = 1.0

	// labelBandLow / labelBandHigh bracket the P2 medium band.
	labelBandLow  = 0.6
	labelBandHigh = 0.8

	// stringBandLow / stringBandHigh bracket the P3 low band.
	stringBandLow  = 0.3
	stringBandHigh = 0.6

	// sameAsBandLow / sameAsBandHigh bracket the P8 cross-language
	// SAME_AS band. The pass is heavily gated (shared-lib location +
	// canonical name + structural field overlap), so even the floor
	// represents a high-signal identity match. The confidence scales
	// with the Jaccard field overlap between the two models.
	sameAsBandLow  = 0.7
	sameAsBandHigh = 0.98
)

// ScoreSameAs maps a field-overlap ratio in [0, 1] (Jaccard over
// normalized field-name sets) to the P8 SAME_AS band. A perfect field
// match lands near the top; the configured minimum overlap maps to the
// floor of the band.
func ScoreSameAs(overlap, minOverlap float64) float64 {
	if overlap >= 1.0 {
		return sameAsBandHigh
	}
	if minOverlap >= 1.0 {
		minOverlap = 0.0
	}
	span := 1.0 - minOverlap
	if span <= 0 {
		return sameAsBandLow
	}
	scaled := sameAsBandLow + (overlap-minOverlap)/span*(sameAsBandHigh-sameAsBandLow)
	if scaled < sameAsBandLow {
		scaled = sameAsBandLow
	}
	if scaled > sameAsBandHigh {
		scaled = sameAsBandHigh
	}
	return scaled
}

// ScoreImport returns the confidence for a P1 (import/calls) link.
func ScoreImport() float64 { return ImportConfidence }

// ScoreLabel maps a raw `idf * kindCompat` product to the P2 band.
//
// The raw value is in [0, 1]. We only compress values that are above
// the candidate threshold; values below pass through unchanged so the
// caller can decide whether to emit a candidate row.
//
// For raw ≥ candidateThreshold we map linearly into [low, high]:
//
//	scaled = low + (raw - candidateThreshold) /
//	              (1 - candidateThreshold) * (high - low)
//
// This guarantees every emitted P2 *link* sits inside [0.6, 0.8].
func ScoreLabel(raw float64) float64 {
	if raw <= 0 {
		return 0
	}
	if raw < labelCandidateThreshold {
		// Pass through; caller will drop it as a non-emission.
		return raw
	}
	span := 1.0 - labelCandidateThreshold
	if span <= 0 {
		return labelBandHigh
	}
	scaled := labelBandLow + (raw-labelCandidateThreshold)/span*(labelBandHigh-labelBandLow)
	if scaled < labelBandLow {
		scaled = labelBandLow
	}
	if scaled > labelBandHigh {
		scaled = labelBandHigh
	}
	return scaled
}

// stringCategoryBase is the base confidence for each P3 pattern
// category. Higher values mean the category is intrinsically less
// likely to collide across unrelated repos.
//
// The numeric ordering reflects the spec's "more specific = more
// confident" rule. All values fit inside the P3 band [0.3, 0.6].
var stringCategoryBase = map[extractionCategory]float64{
	// AWS resources: account id + region make collisions extremely
	// unlikely. Top of the P3 band.
	catSQSARN:         0.60,
	catSQSURL:         0.60,
	catSNSARN:         0.60,
	catLambdaARN:      0.60,
	catEventbridgeARN: 0.60,
	// S3 URIs include a globally-unique bucket name.
	catS3URI: 0.55,
	// HTTP / webhook paths: shared API contracts, fairly specific.
	catWebhookPath: 0.50,
	catHTTPPath:    0.45,
	// Pub/sub channel names: medium specificity.
	catKafkaTopic:  0.40,
	catNATSSubject: 0.40,
	// Feature flags: short tokens, more collisions possible.
	catFeatureFlag: 0.40,
	// Redis keys: broadest pattern, lowest confidence.
	catRedisKey: 0.35,
}

// ScoreString returns the P3 confidence for a given pattern category.
// Unknown categories fall back to the bottom of the band.
func ScoreString(cat extractionCategory) float64 {
	if v, ok := stringCategoryBase[cat]; ok {
		return v
	}
	return stringBandLow
}

// ---------------------------------------------------------------------------
// Extraction-confidence honesty marker (#3628 roadmap area #23)
// ---------------------------------------------------------------------------
//
// The numeric `confidence` float above is a per-pass *quality score* used for
// threshold gating and shortest-path weighting. It is orthogonal to the
// question a graph consumer most often asks: "is this edge grounded in a real
// tree-sitter-AST match on BOTH sides, or was it synthesised heuristically?"
//
// EdgeConfidenceKey carries that categorical honesty marker as a Link property
// so a consumer can distinguish a fully-matched cross-repo edge from a
// fuzzy/string/runtime-derived one WITHOUT having to reverse-engineer the
// numeric band. It is stored under a property (not a top-level field) so it
// rides through the existing Link.Properties → MCP surface plumbing and never
// collides with the float `confidence` JSON field.
//
// Value vocabulary (exactly three values):
//
//   - "resolved"  — both the client/producer and server/consumer endpoints
//     matched on their canonical id (path-normalised HTTP pairs,
//     topic pub↔sub on the same channel, gRPC method↔stub,
//     OpenAPI operationId). Structurally grounded on both sides.
//   - "heuristic" — only one side is AST-grounded and the match is fuzzy:
//     TF-IDF label match, string-literal catalog match,
//     cross-language SAME_AS field-overlap. The edge is a
//     best-effort guess, not a proven contract.
//   - "inferred"  — derived from a runtime-dynamic / interpolated value: a
//     `${apiUrl}`-prefixed call whose static suffix uniquely
//     matched a backend (dynamic_suffix_match), or a concrete
//     caller segment filling a producer param slot
//     (literal_param_fill). One side is real, the other is
//     reconstructed from a runtime expression.
//
// ABSENCE ⇒ resolved: AST-grounded structural edges (CALLS / IMPORTS /
// DEFINES emitted directly by tree-sitter, and the import_pass) deliberately
// do NOT stamp this property — stamping every edge would bloat the graph.
// A consumer MUST treat a missing marker as "resolved".
const EdgeConfidenceKey = "confidence"

const (
	// ConfidenceResolved marks an edge whose two endpoints both matched on a
	// canonical id (the highest-honesty cross-repo class).
	ConfidenceResolved = "resolved"
	// ConfidenceHeuristic marks a fuzzy / single-side-grounded synthesised edge.
	ConfidenceHeuristic = "heuristic"
	// ConfidenceInferred marks an edge derived from a runtime-dynamic /
	// interpolated value.
	ConfidenceInferred = "inferred"
)

// WithEdgeConfidence stamps the honesty marker on a Link, allocating the
// Properties map on demand. It is the single mutation point so the property
// name stays consistent across every pass.
func (l *Link) WithEdgeConfidence(level string) {
	if l.Properties == nil {
		l.Properties = map[string]string{}
	}
	l.Properties[EdgeConfidenceKey] = level
}
