package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
)

// Coverage parity probe (#3876).
//
// The per-language audits hand-spotted a recurring meta-pattern: a
// framework-agnostic capability is credited (full/partial) on one
// framework in a language but left missing on its same-language siblings
// — a likely BUILT-UNCREDITED gap (the extractor already handles it; the
// sibling record just was never re-verified). This probe finds that
// asymmetry continuously so it no longer has to be eyeballed.
//
// Grouping key: (language, category, subcategory). Records in the same
// group are genuine siblings — the same kind of thing (e.g. JVM HTTP
// backends) in the same language. A capability is compared only against
// peers in its own group.
//
// Scaffold suppression (the stale-audit trap): the http_framework+orm
// matrix seeds EVERY framework with the same all-`missing` lane cells as
// a scaffold default. Those uniform-missing cells are NOT gaps — nobody
// has it, so there is no "flagship" to trail. The probe only emits a
// finding when there is REAL ASYMMETRY within a group: at least one
// credited framework AND at least one missing framework for the same
// capability. A group where every framework is missing (uniform
// scaffold) yields zero findings, as does a group where every framework
// is credited.

// parityCredited reports whether a status counts as "the capability is
// present" for parity purposes. partial is included by default because
// the by-hand audits treat a partial flagship as enough to expose a
// trailing missing sibling; --no-partial narrows it to full-only.
func parityCredited(status string, includePartial bool) bool {
	if status == StatusFull {
		return true
	}
	return includePartial && status == StatusPartial
}

// parityFrameworkStatus records one framework's status for a capability
// within a (language, category, subcategory) group.
type parityFrameworkStatus struct {
	RecordID string `json:"record_id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
}

// ParityFinding is a single flagship→sibling asymmetry: within one
// (language, category, subcategory) group, Capability is credited on the
// FullIn/PartialIn frameworks but missing on the TrailingIn frameworks.
// Such a finding is a candidate BUILT-UNCREDITED gap to re-verify.
type ParityFinding struct {
	Language    string                  `json:"language"`
	Category    string                  `json:"category"`
	Subcategory string                  `json:"subcategory,omitempty"`
	Capability  string                  `json:"capability"`
	GroupSize   int                     `json:"group_size"`
	FullIn      []parityFrameworkStatus `json:"full_in"`
	PartialIn   []parityFrameworkStatus `json:"partial_in,omitempty"`
	TrailingIn  []parityFrameworkStatus `json:"trailing_in"`
}

// ParityReport is the full deterministic output of the probe.
type ParityReport struct {
	// MinGroup is the minimum number of sibling frameworks (records) a
	// group must contain before its capabilities are eligible for a
	// finding. With min-group 1 a singleton group can never be asymmetric
	// (it has no peer), so the effective floor is 2; the field is surfaced
	// for transparency and CI configurability.
	MinGroup        int             `json:"min_group"`
	IncludePartial  bool            `json:"include_partial"`
	GroupsScanned   int             `json:"groups_scanned"`
	CellsCompared   int             `json:"cells_compared"`
	Findings        []ParityFinding `json:"findings"`
	SuppressedScaff int             `json:"suppressed_uniform_scaffold"`
}

// parityGroupKey identifies a sibling cohort.
type parityGroupKey struct {
	language    string
	category    string
	subcategory string
}

// computeParity scans the registry and returns the asymmetry report.
//
// language, when non-empty, restricts the scan to that language slug.
// minGroup is the minimum sibling-count threshold (see ParityReport).
// includePartial controls whether a partial cell counts as credited.
func computeParity(reg *Registry, language string, minGroup int, includePartial bool) ParityReport {
	if minGroup < 1 {
		minGroup = 1
	}
	// group → capability → frameworks (each framework's status for that cap)
	groups := map[parityGroupKey]map[string][]parityFrameworkStatus{}
	// distinct record IDs per group, to size the sibling cohort honestly
	// even when frameworks declare disjoint capability sets.
	groupRecords := map[parityGroupKey]map[string]struct{}{}

	for _, rec := range reg.Records {
		if language != "" && rec.Language != language {
			continue
		}
		key := parityGroupKey{rec.Language, rec.Category, rec.Subcategory}
		if groups[key] == nil {
			groups[key] = map[string][]parityFrameworkStatus{}
			groupRecords[key] = map[string]struct{}{}
		}
		groupRecords[key][rec.ID] = struct{}{}
		for cap, cell := range rec.AllCapabilitiesIncludingFrameworkSpecific() {
			groups[key][cap] = append(groups[key][cap], parityFrameworkStatus{
				RecordID: rec.ID,
				Label:    rec.Label,
				Status:   cell.Status,
			})
		}
	}

	report := ParityReport{
		MinGroup:       minGroup,
		IncludePartial: includePartial,
	}

	for key, caps := range groups {
		groupSize := len(groupRecords[key])
		report.GroupsScanned++
		if groupSize < minGroup || groupSize < 2 {
			// A singleton (or sub-threshold) cohort has no peer to trail,
			// so no capability in it can be an asymmetry. Skip whole group.
			continue
		}
		for cap, fws := range caps {
			if len(fws) < 2 {
				continue
			}
			report.CellsCompared++
			var full, partial, trailing []parityFrameworkStatus
			for _, fw := range fws {
				switch {
				case fw.Status == StatusFull:
					full = append(full, fw)
				case includePartial && fw.Status == StatusPartial:
					partial = append(partial, fw)
				case fw.Status == StatusMissing:
					trailing = append(trailing, fw)
				}
				// not_applicable (and partial when !includePartial) are
				// neither credited nor a gap — they drop out of both lists.
			}
			credited := len(full) + len(partial)
			if credited == 0 || len(trailing) == 0 {
				// Uniform scaffold (all missing) or all-credited or a mix
				// with no real trailing sibling: not an asymmetry.
				if credited == 0 && len(trailing) >= 2 {
					// Every comparable peer is missing → the uniform
					// scaffold default. Count it so the report can prove
					// it suppressed the matrix rather than missing it.
					report.SuppressedScaff++
				}
				continue
			}
			sortFrameworks(full)
			sortFrameworks(partial)
			sortFrameworks(trailing)
			report.Findings = append(report.Findings, ParityFinding{
				Language:    key.language,
				Category:    key.category,
				Subcategory: key.subcategory,
				Capability:  cap,
				GroupSize:   groupSize,
				FullIn:      full,
				PartialIn:   partial,
				TrailingIn:  trailing,
			})
		}
	}

	sortFindings(report.Findings)
	return report
}

// sortFrameworks orders a framework slice by record ID for deterministic
// output.
func sortFrameworks(fs []parityFrameworkStatus) {
	sort.Slice(fs, func(i, j int) bool { return fs[i].RecordID < fs[j].RecordID })
}

// sortFindings orders findings deterministically: language, category,
// subcategory, then capability.
func sortFindings(fs []ParityFinding) {
	sort.Slice(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Language != b.Language {
			return a.Language < b.Language
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if a.Subcategory != b.Subcategory {
			return a.Subcategory < b.Subcategory
		}
		return a.Capability < b.Capability
	})
}

// cmdParity implements `coverage parity`: a READ-ONLY probe. It never
// mutates the registry. Exit is non-zero only when --strict is set and
// at least one finding exists (CI gate); otherwise findings are purely
// informational.
func cmdParity(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("parity", flag.ContinueOnError)
	path := registryFlag(fs)
	lang := fs.String("language", "", "restrict to one language slug")
	minGroup := fs.Int("min-group", 2, "minimum sibling frameworks in a (language,category,subcategory) group before its capabilities are compared")
	includePartial := fs.Bool("include-partial", true, "count a partial cell as credited (a partial flagship still exposes a missing sibling)")
	asJSON := fs.Bool("json", false, "JSON output")
	strict := fs.Bool("strict", false, "exit non-zero when any parity gap is found (CI gate)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	reg, err := loadRegistry(*path)
	if err != nil {
		return err
	}
	report := computeParity(reg, *lang, *minGroup, *includePartial)
	if *asJSON {
		if err := printJSON(out, report); err != nil {
			return err
		}
	} else {
		printParityText(out, report)
	}
	if *strict && len(report.Findings) > 0 {
		return fmt.Errorf("parity: %d flagship→sibling gap(s) found", len(report.Findings))
	}
	return nil
}

// printParityText writes a human-readable, diffable parity report.
func printParityText(w io.Writer, r ParityReport) {
	fmt.Fprintf(w, "coverage parity probe — %d finding(s)\n", len(r.Findings))
	fmt.Fprintf(w, "  groups scanned: %d   cells compared: %d   uniform-scaffold suppressed: %d\n",
		r.GroupsScanned, r.CellsCompared, r.SuppressedScaff)
	fmt.Fprintf(w, "  min-group: %d   include-partial: %v\n\n", r.MinGroup, r.IncludePartial)
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "no flagship→sibling asymmetry detected.")
		return
	}
	for _, f := range r.Findings {
		lane := f.Category
		if f.Subcategory != "" {
			lane = f.Category + "/" + f.Subcategory
		}
		fmt.Fprintf(w, "%-10s %-28s %s  (group of %d)\n", f.Language, lane, f.Capability, f.GroupSize)
		for _, fw := range f.FullIn {
			fmt.Fprintf(w, "    full     %s\n", fw.RecordID)
		}
		for _, fw := range f.PartialIn {
			fmt.Fprintf(w, "    partial  %s\n", fw.RecordID)
		}
		for _, fw := range f.TrailingIn {
			fmt.Fprintf(w, "    MISSING  %s  <- trailing sibling\n", fw.RecordID)
		}
		fmt.Fprintln(w)
	}
}
