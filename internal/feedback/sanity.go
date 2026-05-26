package feedback

import (
	"fmt"
	"math"
)

// SanityResult is the outcome of a single sanity check.
type SanityResult struct {
	Name   string
	Passed bool
	Note   string
}

// runSanityChecks evaluates the loaded metrics against the defined sanity checks
// and returns a slice of results plus a confidence score (passed/total as 0–100).
func runSanityChecks(r *Report) ([]SanityResult, int) {
	var results []SanityResult

	// 1. Entity count > 0 for each indexed language.
	for lang, count := range r.EntitiesByLanguage {
		passed := count > 0
		note := ""
		if !passed {
			note = fmt.Sprintf("language %q has 0 entities", lang)
		}
		results = append(results, SanityResult{
			Name:   fmt.Sprintf("entity-count-nonzero[%s]", lang),
			Passed: passed,
			Note:   note,
		})
	}

	// 2. Orphan rate < 100% for all kinds with N >= 10.
	for kind, ks := range r.OrphanByKind {
		if ks.Total < 10 {
			continue
		}
		passed := ks.OrphanPct < 100.0
		note := ""
		if !passed {
			note = fmt.Sprintf("kind %q has 100%% orphan rate (%d/%d)", kind, ks.OrphanCount, ks.Total)
		}
		results = append(results, SanityResult{
			Name:   fmt.Sprintf("orphan-rate-not-100pct[%s]", kind),
			Passed: passed,
			Note:   note,
		})
	}

	// 3. Resolution vector sums to 100% ± 0.1%.
	if r.ResolutionTotal > 0 {
		sum := r.Resolution.ResolvedPct +
			r.Resolution.ExternalKnownPct +
			r.Resolution.ExternalUnknownPct +
			r.Resolution.BugExtractorPct +
			r.Resolution.BugResolverPct +
			r.Resolution.DynamicPct
		passed := math.Abs(sum-100.0) <= 0.1
		note := ""
		if !passed {
			note = fmt.Sprintf("resolution vector sums to %.3f%% (expected 100.0%% ± 0.1%%)", sum)
		}
		results = append(results, SanityResult{
			Name:   "resolution-vector-sums-to-100pct",
			Passed: passed,
			Note:   note,
		})
	}

	// 4. Framework hits >= 1 if known-framework files were detected.
	if r.FrameworkFilesDetected > 0 {
		total := 0
		for _, count := range r.FrameworkHits {
			total += count
		}
		passed := total >= 1
		note := ""
		if !passed {
			note = fmt.Sprintf("%d known-framework files detected but framework_detector_hits = 0", r.FrameworkFilesDetected)
		}
		results = append(results, SanityResult{
			Name:   "framework-hits-if-detected",
			Passed: passed,
			Note:   note,
		})
	}

	// 5. Total entities >= 50.
	passed := r.TotalEntities >= minEntitiesForReport
	note := ""
	if !passed {
		note = fmt.Sprintf("total entities = %d (minimum %d required for reliable report)", r.TotalEntities, minEntitiesForReport)
	}
	results = append(results, SanityResult{
		Name:   "minimum-entity-count",
		Passed: passed,
		Note:   note,
	})

	// Compute confidence.
	passing := 0
	for _, res := range results {
		if res.Passed {
			passing++
		}
	}
	confidence := 0
	if len(results) > 0 {
		confidence = int(math.Round(100.0 * float64(passing) / float64(len(results))))
	}
	return results, confidence
}
