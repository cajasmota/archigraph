// Package docgen — Tier 4 cross-repo contract checks (issue #1760).
//
// Three contracts are enforced at the group level:
//
//  1. checkCrossRepoCoverage  — every cross-repo link target resolves to a page
//     in both the source and target repo's generated doc set.
//  2. checkGroupIndex         — the group index.md exists and links to every
//     repo's index.md.
//  3. checkCrossRepoFlowDedup — the same flow (mermaid block body) does not
//     appear in pages of two different repos (indicates copy-paste or missing
//     shared-component page).
package docgen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CrossRepoViolation is a single group-level contract failure.
type CrossRepoViolation struct {
	// Kind is one of: "cross-repo-coverage", "group-index", "cross-repo-flow-dedup".
	Kind string
	// Message is a human-readable description.
	Message string
	// RepoA and RepoB identify the repos involved (when applicable).
	RepoA string
	RepoB string
	// EntityID is the target entity involved (when applicable).
	EntityID string
}

// ---------------------------------------------------------------------------
// 1. Cross-repo coverage
// ---------------------------------------------------------------------------

// checkCrossRepoCoverage verifies that every cross-repo link target has a page
// in its target repo's generated doc set. It also checks that source entities
// have pages in the source repo. Both sides of each link must be documented.
//
// It returns (violations, totalLinkCount, unresolvedCount).
func checkCrossRepoCoverage(
	_ string,
	links []groupCrossRepoLink,
	repoPageMap map[string][]PageOutput,
) (violations []CrossRepoViolation, totalLinks int, unresolvedCount int) {
	// Build per-repo covered-entity sets.
	repoEntities := make(map[string]map[string]bool) // slug → set of entity IDs
	for slug, pages := range repoPageMap {
		ids := make(map[string]bool, len(pages))
		for _, p := range pages {
			ids[p.EntityID] = true
		}
		repoEntities[slug] = ids
	}

	totalLinks = len(links)

	for _, link := range links {
		// Cross-repo link format: "<repo>::<localId>" or plain "<localId>".
		// Extract repo slug and local ID for source and target.
		srcRepo, srcID := splitCrossRepoRef(link.Source)
		tgtRepo, tgtID := splitCrossRepoRef(link.Target)

		unresolved := false

		// Check target side: target entity must have a page in tgtRepo.
		if tgtRepo != "" && tgtID != "" {
			if tgtEntities, ok := repoEntities[tgtRepo]; ok {
				if !tgtEntities[tgtID] {
					violations = append(violations, CrossRepoViolation{
						Kind:     "cross-repo-coverage",
						Message:  fmt.Sprintf("cross-repo link target %q not found as a page in repo %q", link.Target, tgtRepo),
						RepoB:    tgtRepo,
						EntityID: tgtID,
					})
					unresolved = true
				}
			}
			// If tgtRepo is not in repoPageMap, the repo was not successfully
			// documented — already covered by tier3-error violation.
		}

		// Check source side: source entity should have a page in srcRepo.
		if srcRepo != "" && srcID != "" {
			if srcEntities, ok := repoEntities[srcRepo]; ok {
				if !srcEntities[srcID] {
					violations = append(violations, CrossRepoViolation{
						Kind:     "cross-repo-coverage",
						Message:  fmt.Sprintf("cross-repo link source %q not found as a page in repo %q", link.Source, srcRepo),
						RepoA:    srcRepo,
						EntityID: srcID,
					})
					unresolved = true
				}
			}
		}

		if unresolved {
			unresolvedCount++
		}
	}

	// Sort for determinism.
	sortCrossRepoViolations(violations)
	return
}

// splitCrossRepoRef splits a "<repo>::<localId>" ref into (repo, localId).
// If no "::" separator is present, returns ("", ref) — caller treats as
// unscoped and skips repo-specific checks.
func splitCrossRepoRef(ref string) (repo, localID string) {
	parts := strings.SplitN(ref, "::", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ref
}

// ---------------------------------------------------------------------------
// 2. Group index
// ---------------------------------------------------------------------------

// checkGroupIndex verifies that the group index.md exists and contains a link
// to every repo's index.md.
//
// It increments *unresolvedCount for each missing link and returns violations.
func checkGroupIndex(
	group string,
	repoSlugs []string,
	rootDir string,
	unresolvedCount *int,
) []CrossRepoViolation {
	indexPath := filepath.Join(rootDir, "index.md")
	var violations []CrossRepoViolation

	// Group index must exist.
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		violations = append(violations, CrossRepoViolation{
			Kind:    "group-index",
			Message: fmt.Sprintf("group index.md not found at %q (group: %q): %v", indexPath, group, err),
		})
		if unresolvedCount != nil {
			*unresolvedCount += len(repoSlugs)
		}
		return violations
	}
	indexContent := string(indexData)

	// Every repo must have a link to its index.md in the group index.
	for _, slug := range repoSlugs {
		// Expected link target: "<slug>/index.md" (filepath.Join uses OS separator).
		expectedLink := slug + "/index.md"
		if !strings.Contains(indexContent, expectedLink) {
			violations = append(violations, CrossRepoViolation{
				Kind:    "group-index",
				Message: fmt.Sprintf("group index.md is missing a link to repo %q (expected %q)", slug, expectedLink),
				RepoA:   slug,
			})
			if unresolvedCount != nil {
				*unresolvedCount++
			}
		}
	}

	// Group index must link to score.json.
	if !strings.Contains(indexContent, "score.json") {
		violations = append(violations, CrossRepoViolation{
			Kind:    "group-index",
			Message: "group index.md does not link to score.json",
		})
	}

	return violations
}

// ---------------------------------------------------------------------------
// 3. Cross-repo flow deduplication
// ---------------------------------------------------------------------------

// crossRepoFlowBodyRE reuses the tier2 mermaid block extractor pattern.
var crossRepoFlowBodyRE = regexp.MustCompile("(?s)```mermaid\n(.*?)```")

// checkCrossRepoFlowDedup detects identical mermaid flow blocks appearing in
// pages from two or more different repos. Cross-repo flow duplication indicates
// that a shared architectural component or integration pattern is being described
// redundantly rather than in a dedicated shared-component page.
//
// Returns (violations, dedupViolationCount).
func checkCrossRepoFlowDedup(repoPageMap map[string][]PageOutput) ([]CrossRepoViolation, int) {
	// flowOccurrence records (repoSlug, entityID) for each mermaid block body.
	type flowOccurrence struct {
		repoSlug string
		entityID string
	}

	// Map from trimmed mermaid body → list of (repo, entity) that contain it.
	seen := make(map[string][]flowOccurrence)

	for slug, pages := range repoPageMap {
		for _, p := range pages {
			for _, m := range crossRepoFlowBodyRE.FindAllStringSubmatch(p.MD, -1) {
				body := strings.TrimSpace(m[1])
				if body == "" {
					continue
				}
				seen[body] = append(seen[body], flowOccurrence{
					repoSlug: slug,
					entityID: p.EntityID,
				})
			}
		}
	}

	var violations []CrossRepoViolation
	dedupCount := 0

	for body, occurrences := range seen {
		// Collect unique repo slugs for this flow body.
		repoSet := make(map[string]bool)
		for _, o := range occurrences {
			repoSet[o.repoSlug] = true
		}
		if len(repoSet) < 2 {
			// Flow is only in one repo — intra-repo dedup is Tier 2's job.
			continue
		}

		// Build the set of unique repos (sorted for determinism).
		uniqueRepos := sortedKeys(repoSet)

		snippet := body
		if len(snippet) > 60 {
			snippet = snippet[:60] + "…"
		}

		// Emit one violation per cross-repo pair.
		for i := 0; i < len(uniqueRepos)-1; i++ {
			for j := i + 1; j < len(uniqueRepos); j++ {
				violations = append(violations, CrossRepoViolation{
					Kind: "cross-repo-flow-dedup",
					Message: fmt.Sprintf(
						"identical flow block appears in repos %q and %q: %q",
						uniqueRepos[i], uniqueRepos[j], snippet,
					),
					RepoA: uniqueRepos[i],
					RepoB: uniqueRepos[j],
				})
				dedupCount++
			}
		}
	}

	return violations, dedupCount
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

// sortCrossRepoViolations sorts violations deterministically by Kind+Message.
func sortCrossRepoViolations(vs []CrossRepoViolation) {
	// Insertion sort — violation slices are typically small.
	for i := 1; i < len(vs); i++ {
		for j := i; j > 0; j-- {
			ai := vs[j-1].Kind + vs[j-1].Message
			bi := vs[j].Kind + vs[j].Message
			if ai <= bi {
				break
			}
			vs[j], vs[j-1] = vs[j-1], vs[j]
		}
	}
}

// sortedKeys returns the keys of a string-bool map in sorted order.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Insertion sort — map is small (number of repos in group).
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// ---------------------------------------------------------------------------
// Test-exported wrappers (ForTest suffix — dead-stripped in production builds)
// ---------------------------------------------------------------------------

// GroupCrossRepoLinkForTest is the exported alias of groupCrossRepoLink for
// use by external test packages. Using ForTest suffix follows the docgen
// convention established in tier2 and tier3 test-exported wrappers.
type GroupCrossRepoLinkForTest = groupCrossRepoLink

// CheckCrossRepoCoverageForTest exposes checkCrossRepoCoverage for unit tests.
func CheckCrossRepoCoverageForTest(
	links []groupCrossRepoLink,
	repoPageMap map[string][]PageOutput,
) (violations []CrossRepoViolation, linkCount int, unresolvedCount int) {
	return checkCrossRepoCoverage("", links, repoPageMap)
}

// CheckGroupIndexForTest exposes checkGroupIndex for unit tests.
func CheckGroupIndexForTest(
	group string,
	repoSlugs []string,
	rootDir string,
	unresolvedCount *int,
) []CrossRepoViolation {
	return checkGroupIndex(group, repoSlugs, rootDir, unresolvedCount)
}

// CheckCrossRepoFlowDedupForTest exposes checkCrossRepoFlowDedup for unit tests.
func CheckCrossRepoFlowDedupForTest(
	repoPageMap map[string][]PageOutput,
) (violations []CrossRepoViolation, dedupCount int) {
	return checkCrossRepoFlowDedup(repoPageMap)
}

// CheckCrossRepoContracts runs all three cross-repo contract checks.
// Exported so tests can call it directly.
func CheckCrossRepoContracts(
	group string,
	links []groupCrossRepoLink,
	repoPageMap map[string][]PageOutput,
	repoSlugs []string,
	rootDir string,
) (violations []CrossRepoViolation, linkCount int, linkUnresolved int, flowDedupCount int, groupIndexUnresolved int) {
	crViolations, lc, lu := checkCrossRepoCoverage(group, links, repoPageMap)
	violations = append(violations, crViolations...)
	linkCount = lc
	linkUnresolved = lu

	giViolations := checkGroupIndex(group, repoSlugs, rootDir, &groupIndexUnresolved)
	violations = append(violations, giViolations...)

	fdViolations, fdc := checkCrossRepoFlowDedup(repoPageMap)
	violations = append(violations, fdViolations...)
	flowDedupCount = fdc

	return
}
