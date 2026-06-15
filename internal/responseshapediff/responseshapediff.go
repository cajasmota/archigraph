// Package responseshapediff is the cross-graph, branch-aware RESPONSE-contract
// differ behind the grafel_response_shape_diff MCP tool (#4424, epic #4419
// capability E — the LAST of the four parity diff tools).
//
// The rewrite-parity question it answers: for a joined oracle↔v3 endpoint pair,
// does the v3 rewrite reproduce the oracle's response contract PER BRANCH? A
// rewrite that collapses the oracle's 200(existing)/201(new) split into a single
// 201, or that drops a 409 conflict branch, or that returns a minimal body where
// the oracle returns a full serializer, passes the rewrite's own typecheck +
// unit + contract gate — because those tests assert the shape the author wrote,
// not equivalence to the oracle. The only place that comparison can live is the
// co-resident oracle + v3 graphs.
//
// This package is the DIFF CORE only. It knows nothing about MCP, graph
// entities, frameworks or serializers: it takes two already-composed
// per-endpoint response contracts (a status→field-set map per side, derived by
// the MCP layer from effective_contract's per-branch response shapes) and emits
// a branch-aware diff. Keeping the core framework-agnostic is the epic #4419
// ALL-FRAMEWORK mandate: the MCP layer plugs in DRF / NestJS / Spring / FastAPI
// / Express response composers; the core just diffs the resulting field sets.
//
// Two drift axes are reported, mirroring the issue:
//
//   - status_set_drift — a STATUS branch present on one side and absent on the
//     other (oracle has a 409 v3 lacks; v3 added a 422 the oracle never returns).
//     This is the per-branch status drift the issue calls out.
//   - per-status field_drift — for a status present on BOTH sides, the response
//     FIELD SET diff: fields only-in-oracle / only-in-v3 / type-mismatched /
//     optionality-mismatched. Casing differences (snake vs camel) are folded via
//     the canonical-key alignment so they never false-positive.
//
// Verdict is conservative and honest (mirrors literal_parity #4665 and the
// effective_contract honest-partial discipline):
//
//	equivalent  — every status aligns and every shared status' field set matches.
//	drift       — at least one status_set_drift or field_drift.
//	unresolved  — a side's response shape could not be resolved AT ALL (no
//	              branches with a field set). We refuse to call a pair equivalent
//	              when we simply couldn't see one side — a false "full object"
//	              (#756 F1) is worse than an honest unresolved.
package responseshapediff

import (
	"sort"
	"strings"

	"github.com/cajasmota/grafel/internal/literalparity"
)

// Field is one response field on one side, as the MCP layer resolved it from a
// branch's response shape / DTO field set. Name is the AS-WRITTEN field name
// (preserved for display); Type is the declared type when known ("" = unknown,
// never type-mismatched against another unknown); Optional marks a nullable /
// `?` field.
type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

// Branch is one response branch on one side: the HTTP status and its resolved
// field set. Resolved is false when the branch was detected (a status fired) but
// its payload shape could not be turned into a field set — so the field diff for
// that status must be honest-partial rather than reporting every oracle field as
// "missing in v3".
type Branch struct {
	Status   int     `json:"status"`
	Fields   []Field `json:"fields,omitempty"`
	Resolved bool    `json:"resolved"`
}

// Contract is one endpoint's full response contract on one side: its branches
// keyed by status. ResolvedAny is false when NO branch carried a usable shape —
// the whole side is unresolved (→ verdict unresolved).
type Contract struct {
	Branches    []Branch `json:"branches,omitempty"`
	ResolvedAny bool     `json:"resolved_any"`
}

// Verdict is the per-endpoint response-parity outcome.
type Verdict string

const (
	VerdictEquivalent Verdict = "equivalent"
	VerdictDrift      Verdict = "drift"
	VerdictUnresolved Verdict = "unresolved"
)

// StatusDrift is one status branch present on exactly one side.
type StatusDrift struct {
	Status int    `json:"status"`
	Side   string `json:"side"` // "oracle" (only oracle has it) | "v3" (only v3)
}

// FieldDrift is the field-set diff for one status present on BOTH sides.
type FieldDrift struct {
	Status              int            `json:"status"`
	OnlyInOracle        []string       `json:"only_in_oracle,omitempty"`
	OnlyInV3            []string       `json:"only_in_v3,omitempty"`
	TypeMismatches      []TypeMismatch `json:"type_mismatches,omitempty"`
	OptionalityMismatch []OptMismatch  `json:"optionality_mismatches,omitempty"`
	// Unresolved is true when one or both sides' branch shape for this status
	// could not be turned into a field set — the field diff is then suppressed
	// (honest-partial) and this flag explains why no fields are listed.
	Unresolved bool `json:"unresolved,omitempty"`
}

// TypeMismatch is a field present on both sides at a status whose declared type
// differs (only reported when BOTH sides declare a non-empty type).
type TypeMismatch struct {
	Field  string `json:"field"`
	Oracle string `json:"oracle"`
	V3     string `json:"v3"`
}

// OptMismatch is a field present on both sides at a status whose optionality
// (nullable / `?`) differs.
type OptMismatch struct {
	Field          string `json:"field"`
	OracleOptional bool   `json:"oracle_optional"`
	V3Optional     bool   `json:"v3_optional"`
}

// Result is the full per-endpoint response-shape diff.
type Result struct {
	Verdict        Verdict       `json:"verdict"`
	StatusSetDrift []StatusDrift `json:"status_set_drift,omitempty"`
	FieldDrift     []FieldDrift  `json:"field_drift,omitempty"`
	// Note carries the honest-partial / unresolved explanation when relevant.
	Note string `json:"note,omitempty"`
}

// Diff computes the branch-aware response-shape diff between an oracle contract
// and a v3 contract.
//
// Resolution / honesty rules:
//   - If EITHER side resolved no branch at all (ResolvedAny == false), the
//     verdict is unresolved — we never fabricate an equivalent or a full-drift
//     from a blind side.
//   - status_set_drift is computed over the RESOLVED branch statuses only: a
//     status only one side ever emits is a real branch drift (the #4424 409-drop
//     case). A status present on both gets a field diff.
//   - field_drift for a shared status is suppressed (Unresolved:true) when one
//     side's branch for that status carried no field set — reporting every field
//     as missing would be a false drift.
func Diff(oracle, v3 Contract) Result {
	res := Result{}

	oByStatus := indexBranches(oracle)
	vByStatus := indexBranches(v3)

	// Honest unresolved: a side we couldn't see AT ALL.
	if !oracle.ResolvedAny || !v3.ResolvedAny {
		res.Verdict = VerdictUnresolved
		switch {
		case !oracle.ResolvedAny && !v3.ResolvedAny:
			res.Note = "neither the oracle nor the v3 response shape could be resolved " +
				"to a field set on any branch — refusing to call this equivalent"
		case !oracle.ResolvedAny:
			res.Note = "the oracle response shape could not be resolved to a field set " +
				"on any branch — refusing to call this equivalent (a false full-object is " +
				"worse than an honest unresolved)"
		default:
			res.Note = "the v3 response shape could not be resolved to a field set on any " +
				"branch — refusing to call this equivalent"
		}
		return res
	}

	// Union of statuses across both sides, deterministic order.
	statusSet := map[int]bool{}
	for s := range oByStatus {
		statusSet[s] = true
	}
	for s := range vByStatus {
		statusSet[s] = true
	}
	statuses := make([]int, 0, len(statusSet))
	for s := range statusSet {
		statuses = append(statuses, s)
	}
	sort.Ints(statuses)

	for _, s := range statuses {
		ob, oHas := oByStatus[s]
		vb, vHas := vByStatus[s]
		switch {
		case oHas && !vHas:
			res.StatusSetDrift = append(res.StatusSetDrift, StatusDrift{Status: s, Side: "oracle"})
		case vHas && !oHas:
			res.StatusSetDrift = append(res.StatusSetDrift, StatusDrift{Status: s, Side: "v3"})
		default:
			if fd, drift := diffStatus(s, ob, vb); drift {
				res.FieldDrift = append(res.FieldDrift, fd)
			}
		}
	}

	if len(res.StatusSetDrift) == 0 && len(res.FieldDrift) == 0 {
		res.Verdict = VerdictEquivalent
	} else {
		res.Verdict = VerdictDrift
	}
	return res
}

// indexBranches builds a status→Branch index over a side's RESOLVED branches.
// Unresolved branches (a status fired but no field set) are still indexed so a
// shared status with one unresolved side is reported honest-partial rather than
// as a phantom status_set_drift.
func indexBranches(c Contract) map[int]Branch {
	out := map[int]Branch{}
	for _, b := range c.Branches {
		if b.Status == 0 {
			continue
		}
		// Richest branch wins on a status collision (more fields = more resolved).
		if existing, ok := out[b.Status]; ok {
			if len(existing.Fields) >= len(b.Fields) {
				continue
			}
		}
		out[b.Status] = b
	}
	return out
}

// diffStatus diffs the field sets of a status present on both sides. Returns
// (drift record, true) when there is any field drift; (zero, false) when the
// field sets match. Field alignment is by CANONICAL key (snake_case ↔ camelCase
// fold) so casing differences never false-positive; the AS-WRITTEN oracle name
// is reported for display.
func diffStatus(status int, ob, vb Branch) (FieldDrift, bool) {
	fd := FieldDrift{Status: status}

	// Honest-partial: a side that fired this status but resolved no field set —
	// don't report every field as missing.
	if !ob.Resolved || !vb.Resolved {
		fd.Unresolved = true
		return fd, true
	}

	oByKey := indexFields(ob.Fields)
	vByKey := indexFields(vb.Fields)

	// only_in_oracle / type / optionality mismatch (oracle-driven walk).
	oKeys := sortedKeys(oByKey)
	for _, k := range oKeys {
		of := oByKey[k]
		vf, ok := vByKey[k]
		if !ok {
			fd.OnlyInOracle = append(fd.OnlyInOracle, of.Name)
			continue
		}
		if of.Type != "" && vf.Type != "" && !typesEqual(of.Type, vf.Type) {
			fd.TypeMismatches = append(fd.TypeMismatches, TypeMismatch{
				Field: of.Name, Oracle: of.Type, V3: vf.Type,
			})
		}
		if of.Optional != vf.Optional {
			fd.OptionalityMismatch = append(fd.OptionalityMismatch, OptMismatch{
				Field: of.Name, OracleOptional: of.Optional, V3Optional: vf.Optional,
			})
		}
	}
	// only_in_v3 (v3-driven walk for keys absent in oracle).
	for _, k := range sortedKeys(vByKey) {
		if _, ok := oByKey[k]; !ok {
			fd.OnlyInV3 = append(fd.OnlyInV3, vByKey[k].Name)
		}
	}

	if len(fd.OnlyInOracle) == 0 && len(fd.OnlyInV3) == 0 &&
		len(fd.TypeMismatches) == 0 && len(fd.OptionalityMismatch) == 0 {
		return FieldDrift{}, false
	}
	return fd, true
}

// indexFields buckets a field slice by canonical key (last writer wins is fine —
// duplicate canonical keys within one branch are degenerate).
func indexFields(fs []Field) map[string]Field {
	out := make(map[string]Field, len(fs))
	for _, f := range fs {
		k := literalparity.CanonicalKey(f.Name)
		if k == "" {
			continue
		}
		out[k] = f
	}
	return out
}

func sortedKeys(m map[string]Field) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// typesEqual compares two declared types tolerantly: trimmed, lower-cased, and
// with a few cross-stack scalar synonyms folded (string↔str, int↔integer↔number,
// bool↔boolean). It is deliberately conservative — when in doubt it treats types
// as equal so a stylistic difference is not reported as a mismatch.
func typesEqual(a, b string) bool {
	na := normalizeType(a)
	nb := normalizeType(b)
	return na == nb
}

var typeSynonyms = map[string]string{
	"str":     "string",
	"integer": "int",
	"number":  "int",
	"long":    "int",
	"float":   "float",
	"double":  "float",
	"boolean": "bool",
}

func normalizeType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	t = strings.TrimSuffix(t, "?") // nullable marker handled via Optional
	if syn, ok := typeSynonyms[t]; ok {
		return syn
	}
	return t
}
