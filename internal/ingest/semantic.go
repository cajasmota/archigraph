// Package ingest — Layer 2 (agent-driven semantic) doc ingestion: the EMIT step
// (issue #4308, epic #4294).
//
// Layer 1 (markdown.go / ingest.go) is fully deterministic: it produces
// Document/Section nodes and exact-match MENTIONS edges with NO LLM call. Layer
// 2 is the OPT-IN, agent-driven semantic enrichment on top of those Sections,
// and it preserves the same core invariant: grafel makes ZERO LLM calls.
//
// This file is the EMIT half. For each L1 Section it produces a serialisable
// SemanticBundle carrying the section's prose, grounding context (the code
// entities the section MENTIONS), and an extraction SCHEMA telling an EXTERNAL
// agent what semantic nodes/edges to extract (classify the section as a
// DesignDecision / Rationale / Spec, and extract rationale relationships to the
// code entities it references). grafel EMITS the bundle only; the external
// agent (a Claude Code skill, not this code) runs the LLM and writes back a
// SemanticRunResult, which the APPLY half (semantic_apply.go, #4309) validates
// and merges into the graph.
//
// This mirrors the docgen --llm-mode=emit pattern (internal/docgen/llm_bundle.go:
// LLMPromptBundle / LLMRunResult) and the grafel_enrichments emit→submit
// candidate pattern. No LLM calls, no network, no API keys.
package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/cajasmota/grafel/internal/graph"
	"github.com/cajasmota/grafel/internal/types"
)

// SemanticBundleVersion is the schema version embedded in every bundle and
// echoed back in the result. Bumped on breaking changes. Mirrors
// docgen.LLMBundleVersion.
const SemanticBundleVersion = "1"

// SemanticClass is one of the closed set of section classifications an agent is
// allowed to assign. Keeping this a closed set (rather than free-form) is what
// lets the apply step reject hallucinated classes deterministically.
type SemanticClass string

const (
	// SemanticClassDesignDecision — the section records a design decision (a
	// choice made, with the alternatives or trade-offs).
	SemanticClassDesignDecision SemanticClass = "DesignDecision"
	// SemanticClassRationale — the section explains WHY something is the way it
	// is (justification/reasoning) without necessarily recording a decision.
	SemanticClassRationale SemanticClass = "Rationale"
	// SemanticClassSpec — the section states a specification / requirement /
	// contract that code must satisfy.
	SemanticClassSpec SemanticClass = "Spec"
	// SemanticClassConstraint — the section states a constraint / invariant /
	// limitation.
	SemanticClassConstraint SemanticClass = "Constraint"
	// SemanticClassNone — the section carries no extractable semantic claim
	// (narrative, boilerplate, TOC, etc.). An agent returning None for a section
	// is the honest "nothing here" answer and yields no semantic node.
	SemanticClassNone SemanticClass = "None"
)

// allSemanticClasses is the closed set the apply step validates against.
var allSemanticClasses = map[SemanticClass]bool{
	SemanticClassDesignDecision: true,
	SemanticClassRationale:      true,
	SemanticClassSpec:           true,
	SemanticClassConstraint:     true,
	SemanticClassNone:           true,
}

// IsValidSemanticClass reports whether c is one of the closed-set classes.
func IsValidSemanticClass(c SemanticClass) bool { return allSemanticClasses[c] }

// SemanticBundle is the top-level emit artifact for ONE document's worth of
// sections. It is self-contained: an external agent needs nothing but this JSON
// to run its LLM and produce a SemanticRunResult. Mirrors the envelope shape of
// docgen.LLMPromptBundle.
type SemanticBundle struct {
	// Version is the schema version ("1").
	Version string `json:"version"`
	// RepoTag is the per-repo slug stamped into entity IDs (matches the IDs in
	// SectionID / candidate target IDs).
	RepoTag string `json:"repo_tag"`
	// DocumentID is the SCOPE.MarkdownDocument entity ID these sections belong to.
	DocumentID string `json:"document_id"`
	// DocRelPath is the repo-relative slash path of the source markdown file.
	DocRelPath string `json:"doc_rel_path"`
	// Sections holds one prompt entry per L1 Section in the document.
	Sections []SemanticSectionPrompt `json:"sections"`
	// Schema is the extraction instruction set (closed-set classes + the edge
	// kind + agent-facing guidance). Repeated in the bundle so the artifact is
	// self-describing for an agent that has never seen grafel.
	Schema SemanticExtractionSchema `json:"schema"`
	// PromptHash is the bundle-level cache key (sha256 over per-section hashes).
	// The result must echo it so apply can detect a stale/mismatched result.
	PromptHash string `json:"prompt_hash"`
	// GeneratedAt is the RFC3339 timestamp when the bundle was built.
	GeneratedAt string `json:"generated_at"`
}

// SemanticSectionPrompt is the per-section payload an agent classifies.
type SemanticSectionPrompt struct {
	// SectionID is the SCOPE.Section entity ID. The result references it so the
	// produced DesignDecision node can be anchored back (CONTAINS) to it.
	SectionID string `json:"section_id"`
	// Heading is the section's heading text (or anchor for headingless spans).
	Heading string `json:"heading"`
	// Depth is the ATX heading depth (1..6).
	Depth int `json:"depth"`
	// StartLine / EndLine locate the section in the source file (for the agent's
	// context and for get_source-style quoting).
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
	// Body is the section's OWN direct prose (no nested subsection text), the
	// text the agent classifies and reasons over.
	Body string `json:"body"`
	// MentionTargets are the code entities this section already MENTIONS (from
	// L1 exact-match linking). These are the ONLY entity IDs an agent may cite
	// as rationale targets — grounding the agent to entities the section
	// demonstrably references keeps the apply step's "target must exist + be
	// referenced" validation tight and prevents invented edges.
	MentionTargets []SemanticTarget `json:"mention_targets"`
	// PromptHash is the per-section cache key (sha256). Additive; omitted empty.
	PromptHash string `json:"prompt_hash,omitempty"`
}

// SemanticTarget is one grounded code-entity reference an agent may use as a
// rationale target.
type SemanticTarget struct {
	// ID is the code entity's graph ID (the RATIONALE_FOR edge will point here).
	ID string `json:"id"`
	// Name is the entity's short name (for the agent's readability).
	Name string `json:"name"`
	// Kind is the entity kind (e.g. "function", "class").
	Kind string `json:"kind"`
}

// SemanticExtractionSchema is the agent-facing instruction set embedded in the
// bundle. It is intentionally declarative + closed-set so the apply step can
// validate against the exact same constants.
type SemanticExtractionSchema struct {
	// Classes is the closed set of allowed classifications.
	Classes []SemanticClass `json:"classes"`
	// EdgeKind is the relationship kind a rationale link becomes (RATIONALE_FOR).
	EdgeKind string `json:"edge_kind"`
	// Guidance is human/agent-readable prose telling the agent what to do.
	Guidance string `json:"guidance"`
	// MaxSummaryWords bounds the agent's distilled summary per section.
	MaxSummaryWords int `json:"max_summary_words"`
}

// defaultSemanticSchema is the single, deterministic extraction schema embedded
// in every bundle. Kept here (not config) so emit + apply share one source of
// truth, mirroring docgen.defaultSectionGuidance.
func defaultSemanticSchema() SemanticExtractionSchema {
	return SemanticExtractionSchema{
		Classes: []SemanticClass{
			SemanticClassDesignDecision,
			SemanticClassRationale,
			SemanticClassSpec,
			SemanticClassConstraint,
			SemanticClassNone,
		},
		EdgeKind:        "RATIONALE_FOR",
		MaxSummaryWords: 60,
		Guidance: "For each section, classify its prose as exactly one of `classes`. " +
			"Choose `None` when the section states no design decision, rationale, spec, or constraint " +
			"(e.g. a table of contents, narrative intro, or changelog) — returning None is the correct, " +
			"honest answer and produces no node. For any non-None class, write a `summary` (<= " +
			"max_summary_words words) distilling the decision/rationale/spec/constraint in your own words, " +
			"and list `rationale_target_ids`: the subset of this section's `mention_targets` IDs that the " +
			"claim actually justifies or constrains. You MUST NOT invent target IDs — only IDs present in " +
			"this section's mention_targets are permitted; any other ID will be rejected. Each accepted " +
			"non-None classification becomes a SCOPE.DesignDecision node anchored to the section, with one " +
			"RATIONALE_FOR edge to each cited target. grafel runs NO LLM — you (the calling agent) run " +
			"the model; grafel only validates and applies what you return.",
	}
}

// EmitSemanticBundles runs the L1 markdown pipeline over mdRelPaths and emits
// one SemanticBundle per document (the #4308 emit step). It mirrors Ingest's
// parse → section-ID-stamp → exact-mention-link sequence so the bundles carry
// EXACTLY the section IDs and MENTIONS targets that L1 produced — keeping emit
// and the deterministic graph in lockstep. grafel makes NO LLM call here.
//
// repoRoot/repoTag/codeEntities have the same meaning as in Ingest. Bundles for
// documents with no sections are skipped. Output is deterministic (files sorted,
// sections in source order).
func EmitSemanticBundles(repoRoot, repoTag string, mdRelPaths []string, codeEntities []graph.Entity) []SemanticBundle {
	// Build the same name index Ingest uses for exact-match linking.
	tuples := make([]NameTuple, 0, len(codeEntities))
	idByID := make(map[string]*graph.Entity, len(codeEntities))
	for i := range codeEntities {
		e := &codeEntities[i]
		idByID[e.ID] = e
		tuples = append(tuples, NameTuple{Name: e.Name, QualifiedName: e.QualifiedName, ID: e.ID, Kind: e.Kind})
	}
	nameIdx := IndexNames(tuples)

	paths := append([]string(nil), mdRelPaths...)
	sort.Strings(paths)

	var bundles []SemanticBundle
	for _, rel := range paths {
		rel = filepath.ToSlash(rel)
		abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
		content, err := readBoundedFile(abs, docByteLimit(rel))
		if err != nil {
			continue
		}
		doc, sections, _, perr := parseDoc(rel, content)
		if perr != nil || len(sections) == 0 {
			continue
		}

		// Stamp section IDs identically to Ingest so the bundle references the
		// SAME SCOPE.Section entity IDs present in the graph.
		docID := graph.EntityID(repoTag, string(types.EntityKindMarkdownDocument), rel, rel)
		sectionIDs := make([]string, len(sections))
		for k := range sections {
			s := &sections[k]
			name := fmt.Sprintf("%s#L%d", rel, s.StartLine)
			sectionIDs[k] = graph.EntityID(repoTag, string(types.EntityKindSection), name, rel)
		}

		// L1 mentions → grounded per-section targets.
		mentionsBySection := map[int][]SemanticTarget{}
		for _, m := range LinkMentions(sections, nameIdx) {
			name, kind := m.Token, m.TargetKind
			if e := idByID[m.TargetID]; e != nil {
				name = e.Name
			}
			mentionsBySection[m.SectionIndex] = append(mentionsBySection[m.SectionIndex], SemanticTarget{
				ID: m.TargetID, Name: name, Kind: kind,
			})
		}

		_ = doc // Title not needed in the bundle; ParseDocument shares the parse.
		bundles = append(bundles, EmitDocumentBundle(repoTag, docID, rel, sections, sectionIDs, mentionsBySection))
	}
	return bundles
}

// EmitDocumentBundle builds a SemanticBundle for one document's sections.
//
// docID is the SCOPE.MarkdownDocument entity ID; docRelPath its repo-relative
// slash path; sectionIDs[k] is the SCOPE.Section entity ID for sections[k] (as
// stamped by Ingest). mentionsBySection maps a section index to the code
// entities that section MENTIONS (from L1 linking) — these become the bundle's
// grounded targets. repoTag is stamped into the artifact for traceability.
//
// Fully deterministic given identical inputs: the same graph state always
// yields the same PromptHash. No LLM call, no network.
func EmitDocumentBundle(repoTag, docID, docRelPath string, sections []Section, sectionIDs []string, mentionsBySection map[int][]SemanticTarget) SemanticBundle {
	b := SemanticBundle{
		Version:     SemanticBundleVersion,
		RepoTag:     repoTag,
		DocumentID:  docID,
		DocRelPath:  docRelPath,
		Schema:      defaultSemanticSchema(),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	for k := range sections {
		s := &sections[k]
		secID := ""
		if k < len(sectionIDs) {
			secID = sectionIDs[k]
		}
		targets := append([]SemanticTarget(nil), mentionsBySection[k]...)
		// Deterministic target ordering.
		sort.SliceStable(targets, func(a, c int) bool { return targets[a].ID < targets[c].ID })

		sp := SemanticSectionPrompt{
			SectionID:      secID,
			Heading:        headingOrAnchor(s.HeadingText, s.StartLine),
			Depth:          s.Depth,
			StartLine:      s.StartLine,
			EndLine:        s.EndLine,
			Body:           s.Body,
			MentionTargets: targets,
		}
		sp.PromptHash = sectionPromptHash(repoTag, secID, sp.Body, targets)
		b.Sections = append(b.Sections, sp)
	}

	b.PromptHash = bundlePromptHash(&b)
	return b
}

// sectionPromptHash is the per-section cache key: a stable sha256 over the
// identity + content that determines the agent's task. Mirrors docgen's
// per-section prompt_hash design.
func sectionPromptHash(repoTag, sectionID, body string, targets []SemanticTarget) string {
	h := sha256.New()
	h.Write([]byte(SemanticBundleVersion))
	h.Write([]byte{0})
	h.Write([]byte(repoTag))
	h.Write([]byte{0})
	h.Write([]byte(sectionID))
	h.Write([]byte{0})
	h.Write([]byte(body))
	for _, t := range targets {
		h.Write([]byte{0})
		h.Write([]byte(t.ID))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// bundlePromptHash is the bundle-level cache key: sha256 over the per-section
// hashes in order. Mirrors docgen's bundle-hash design.
func bundlePromptHash(b *SemanticBundle) string {
	h := sha256.New()
	for i := range b.Sections {
		h.Write([]byte(b.Sections[i].PromptHash))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// WriteBundle serialises a SemanticBundle as indented JSON to
// <runDir>/<documentID>.bundle.json, creating runDir if needed. Mirrors the
// docgen run-dir artifact convention (one bundle file per unit of work).
// Returns the written path.
func WriteBundle(runDir string, b SemanticBundle) (string, error) {
	if runDir == "" {
		return "", fmt.Errorf("ingest: WriteBundle: empty run dir")
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("ingest: create run dir %q: %w", runDir, err)
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return "", fmt.Errorf("ingest: marshal bundle: %w", err)
	}
	name := b.DocumentID
	if name == "" {
		name = "document"
	}
	path := filepath.Join(runDir, name+".bundle.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("ingest: write bundle %q: %w", path, err)
	}
	return path, nil
}

// ReadBundle reads and unmarshals a SemanticBundle from disk.
func ReadBundle(path string) (SemanticBundle, error) {
	var b SemanticBundle
	data, err := os.ReadFile(path) //nolint:gosec // path from a run-dir listing
	if err != nil {
		return b, fmt.Errorf("ingest: read bundle %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &b); err != nil {
		return b, fmt.Errorf("ingest: unmarshal bundle %q: %w", path, err)
	}
	return b, nil
}
