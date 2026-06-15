// crossproduct.go — first-class reachability × line-coverage cross-product
// report (#5063).
//
// This promotes the pure CrossSignal verdict (reachable && line_pct == 0 =>
// candidate ineffective/tautological test) from a single-entity helper into a
// first-class, group-wide REPORT: it classifies every production fn/endpoint
// into a small set of meaningful quadrants combining
//
//   - #5037 static test-reachability (PropTestReachable / PropReachingTests…),
//     stamped at index time by #5061; and
//   - #5036 dynamic LCOV line coverage (PropCoveragePct / PropCoveredLines…),
//     stamped at index time by #5061,
//
// and rolls the quadrants up per module/group with counts. The headline signal
// is the EffReachableNoLines quadrant: an entity that has a static test path
// reaching it (reachable) yet 0% measured line coverage — a candidate
// ineffective / tautological test (ties into the #4893 tautology detector).
//
// Pure: this is a read-only transformation over already-stamped entity
// Properties. It does not run the LCOV or reachability passes, does not mutate
// inputs, does not touch Kinds, and does not call the indexer/daemon. The MCP
// surfacing (grafel_coverage_effectiveness) and dashboard surfacing
// (#5062 / #5067) consume this report.
//
// HONEST degradation: an entity that is reachability-stamped but has NO line
// coverage signal cannot be crossed — it lands in EffReachableNoCoverage (NOT
// EffReachableNoLines, which requires a measured 0%). The roll-up tracks how
// many entities have line coverage at all (CoverageMeasured) so a caller can
// honestly say "line-coverage cross unavailable for this group" rather than
// fabricating a verdict.
package coverage

import (
	"sort"
	"strconv"

	"github.com/cajasmota/grafel/internal/types"
)

// EffectivenessVerdict is the quadrant an entity falls into when static
// reachability is crossed with line coverage. It is a refinement of
// CrossSignalVerdict that splits the "reachable" side by coverage band so the
// report can separate weak coverage from healthy coverage, and separates
// "reachable but no measured line coverage at all" (honest degradation) from
// "reachable but measured 0%" (the ineffective-test signal).
type EffectivenessVerdict string

const (
	// EffReachableNoLines: reachable from a test AND measured line_pct == 0 —
	// the HEADLINE signal. A static test path reaches the entity, yet not one
	// production line executed: a candidate ineffective / tautological test
	// (#4893). Distinct from EffReachableNoCoverage, which has no measurement.
	EffReachableNoLines EffectivenessVerdict = "reachable_no_lines"
	// EffReachableLowCoverage: reachable AND 0 < line_pct < LowCoverageThreshold
	// — weak coverage, the test path barely exercises the entity.
	EffReachableLowCoverage EffectivenessVerdict = "reachable_low_coverage"
	// EffReachableCovered: reachable AND line_pct >= LowCoverageThreshold —
	// healthy: tested and actually run.
	EffReachableCovered EffectivenessVerdict = "reachable_covered"
	// EffReachableNoCoverage: reachable but NO line-coverage measurement exists
	// for the entity (absent from the LCOV report, or the group has no ingested
	// coverage at all). HONEST degradation: we cannot cross, so we do not claim
	// the test is ineffective.
	EffReachableNoCoverage EffectivenessVerdict = "reachable_no_coverage"
	// EffUntested: not reachable from any test — untested surface (#5037
	// orphans). Line coverage, if any, is incidental and not crossed here.
	EffUntested EffectivenessVerdict = "untested"
	// EffNotMeasured: the entity carries no reachability signal at all (e.g.
	// indexed before #5061, or not a reachability-considered production kind).
	// Cannot classify.
	EffNotMeasured EffectivenessVerdict = "not_measured"
)

// LowCoverageThreshold is the line-coverage percentage (exclusive lower bound
// is 0) below which a reachable entity is classed EffReachableLowCoverage
// rather than EffReachableCovered. Chosen as a conservative "barely exercised"
// cutoff; callers wanting a different band can re-derive from the raw pct.
const LowCoverageThreshold = 50.0

// ClassifyEffectiveness reads the reachability + LCOV properties already
// stamped on an entity and returns its cross-product quadrant. Pure over the
// property map; mirrors CrossSignal but with the richer coverage-band split and
// the honest no-measurement quadrant.
//
// It deliberately does NOT depend on entity Kind — callers feed it only
// production-entity properties (the report layer applies the production filter).
func ClassifyEffectiveness(props map[string]string) EffectivenessVerdict {
	reachStr, hasReach := props[PropTestReachable]
	if !hasReach {
		return EffNotMeasured
	}
	reachable, _ := strconv.ParseBool(reachStr)
	if !reachable {
		return EffUntested
	}
	pctStr, hasPct := props[PropCoveragePct]
	if !hasPct {
		return EffReachableNoCoverage
	}
	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		return EffReachableNoCoverage
	}
	switch {
	case pct <= 0:
		return EffReachableNoLines
	case pct < LowCoverageThreshold:
		return EffReachableLowCoverage
	default:
		return EffReachableCovered
	}
}

// EffectivenessRow is one production entity's cross-product classification,
// projected from its stamped properties. The raw signals are retained so a
// caller (MCP tool / dashboard) can render detail without re-reading props.
type EffectivenessRow struct {
	EntityID    string
	Verdict     EffectivenessVerdict
	Reachable   bool
	ReachDepth  int
	ReachCount  int     // distinct reaching tests (uncapped)
	HasCoverage bool    // a line-coverage measurement exists for this entity
	CoveragePct float64 // measured line coverage; valid only when HasCoverage
}

// classifyRow builds an EffectivenessRow from an entity's stamped properties.
func classifyRow(id string, props map[string]string) EffectivenessRow {
	row := EffectivenessRow{EntityID: id, Verdict: ClassifyEffectiveness(props)}
	if v, ok := props[PropTestReachable]; ok {
		row.Reachable, _ = strconv.ParseBool(v)
	}
	if row.Reachable {
		row.ReachDepth, _ = strconv.Atoi(props[PropReachDepth])
		row.ReachCount, _ = strconv.Atoi(props[PropReachingTestCount])
	}
	if v, ok := props[PropCoveragePct]; ok {
		if pct, err := strconv.ParseFloat(v, 64); err == nil {
			row.HasCoverage = true
			row.CoveragePct = pct
		}
	}
	return row
}

// EffectivenessCounts is the per-quadrant count breakdown for a set of
// production entities, plus the derived totals a roll-up needs.
type EffectivenessCounts struct {
	Total                int // production entities considered (reachability-stamped)
	ReachableNoLines     int // the ineffective-test signal
	ReachableLowCoverage int
	ReachableCovered     int
	ReachableNoCoverage  int
	Untested             int
	// CoverageMeasured is the number of entities with ANY line-coverage signal
	// (the cross-product denominator). When 0, the line-coverage cross is
	// unavailable for this bucket — report reachability quadrants only.
	CoverageMeasured int
}

// add folds one row into the counts.
func (c *EffectivenessCounts) add(r EffectivenessRow) {
	c.Total++
	if r.HasCoverage {
		c.CoverageMeasured++
	}
	switch r.Verdict {
	case EffReachableNoLines:
		c.ReachableNoLines++
	case EffReachableLowCoverage:
		c.ReachableLowCoverage++
	case EffReachableCovered:
		c.ReachableCovered++
	case EffReachableNoCoverage:
		c.ReachableNoCoverage++
	case EffUntested:
		c.Untested++
	}
}

// LineCrossAvailable reports whether at least one entity in the bucket has a
// line-coverage measurement to cross with reachability. When false, the caller
// MUST degrade honestly (reachability quadrants only).
func (c EffectivenessCounts) LineCrossAvailable() bool { return c.CoverageMeasured > 0 }

// EffectivenessReport is the first-class cross-product report: per-entity
// classification, the headline ineffective-test list (reachable-but-0%-lines),
// and per-module + group roll-ups.
type EffectivenessReport struct {
	// Rows is the per-entity classification for every reachability-stamped
	// production entity, in input order.
	Rows []EffectivenessRow
	// Ineffective is the subset of Rows in the EffReachableNoLines quadrant —
	// the headline candidate ineffective/tautological tests — sorted by entity
	// ID for determinism.
	Ineffective []EffectivenessRow
	// Group is the group-wide quadrant breakdown.
	Group EffectivenessCounts
	// Modules is the per-module breakdown, keyed by module bucket (Properties
	// "module" when present, else source-file directory).
	Modules map[string]EffectivenessCounts
}

// ComputeEffectivenessReport is the first-class cross-product report builder.
// It crosses #5037 reachability with #5036 line coverage over the already-
// stamped Properties of the entity batch, classifies each production entity
// into a quadrant, collects the ineffective-test list, and rolls up per module
// and group. Pure: inputs are not mutated.
//
// Only reachability-stamped production entities are considered (those carry the
// PropTestReachable property the #5061 pass stamps); other entities — schemas,
// configs, tests, or anything indexed before #5061 — are skipped, so an
// unstamped group yields an empty report (Group.Total == 0), which the caller
// surfaces as "reachability not computed — reindex".
func ComputeEffectivenessReport(entities []types.EntityRecord) EffectivenessReport {
	rep := EffectivenessReport{Modules: map[string]EffectivenessCounts{}}
	for _, e := range entities {
		if e.Properties == nil {
			continue
		}
		if _, ok := e.Properties[PropTestReachable]; !ok {
			continue // not a reachability-considered production entity
		}
		row := classifyRow(entityID(e), e.Properties)
		rep.Rows = append(rep.Rows, row)
		rep.Group.add(row)

		mod := moduleKey(e.SourceFile)
		if m := e.Properties["module"]; m != "" {
			mod = m
		}
		mc := rep.Modules[mod]
		mc.add(row)
		rep.Modules[mod] = mc

		if row.Verdict == EffReachableNoLines {
			rep.Ineffective = append(rep.Ineffective, row)
		}
	}
	sort.Slice(rep.Ineffective, func(i, j int) bool {
		return rep.Ineffective[i].EntityID < rep.Ineffective[j].EntityID
	})
	return rep
}
