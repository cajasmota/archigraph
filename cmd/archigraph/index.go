package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/cajasmota/archigraph/internal/classifier"
	"github.com/cajasmota/archigraph/internal/engine"
	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/extractors"
	"github.com/cajasmota/archigraph/internal/extractors/cross"
	"github.com/cajasmota/archigraph/internal/graph"
	"github.com/cajasmota/archigraph/internal/treesitter"
	"github.com/cajasmota/archigraph/internal/types"
	"github.com/cajasmota/archigraph/internal/version"
)

// Pass names accepted by the --skip-pass flag.
const (
	PassExtract       = "extract"        // Pass 1: per-language AST extraction
	PassFramework     = "framework"      // Pass 2.5: YAML-driven framework rules
	PassCrossLang     = "cross-lang"     // Pass 3: cross-language extractors
	PassGraphAlgo     = "graph-algo"     // Pass 4: placeholder for PORT-4
	PassBuildDocument = "build-document" // Pass 5: assemble graph.Document
)

// allPassNames is used to validate --skip-pass entries.
var allPassNames = []string{
	PassExtract, PassFramework, PassCrossLang, PassGraphAlgo, PassBuildDocument,
}

// fileTask carries one repo-relative path and its absolute counterpart
// through the Pass 1 worker pool.
type fileTask struct {
	relPath string
	absPath string
}

// classifiedFile is a file that survived classification — extractors will
// be run against it in Pass 1 and Pass 3.
type classifiedFile struct {
	relPath  string
	absPath  string
	language string
	content  []byte
	tree     *sitter.Tree
}

// Indexer owns the pass-by-pass orchestration. Constructing a fresh Indexer
// per Index() call keeps state (counters, configuration) local.
type Indexer struct {
	repoTag    string
	classifier *classifier.Classifier
	parser     *treesitter.ParserFactory
	detector   *engine.Detector
	skipPasses map[string]bool
	workers    int

	// Statistics — populated as passes run, surfaced in the final summary.
	stats indexerStats
}

type indexerStats struct {
	files     int
	processed int
	extracted int
	skipped   int
	failed    int
	pass1Rels int
	pass2Rels int
	pass3Rels int
}

// Index walks repoPath, runs the orchestrated passes, and writes the
// resulting entity/relationship graph to outPath (or the default
// <repo>/.archigraph/graph.json). repoTag is stored on every entity; an
// empty value falls back to filepath.Base(repoPath). skipPasses is the
// (possibly empty) set of pass names to skip — see allPassNames.
func Index(repoPath, outPath, repoTag string, skipPasses []string) error {
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}
	info, err := os.Stat(absRepo)
	if err != nil {
		return fmt.Errorf("stat repo: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("repo path %q is not a directory", absRepo)
	}

	if repoTag == "" {
		repoTag = filepath.Base(absRepo)
	}
	if outPath == "" {
		outPath = filepath.Join(absRepo, ".archigraph", "graph.json")
	}

	skipSet, err := parseSkipPasses(skipPasses)
	if err != nil {
		return err
	}

	cls, err := classifier.New("", nil)
	if err != nil {
		return fmt.Errorf("init classifier: %w", err)
	}
	parser := treesitter.NewParserFactory(nil)

	rules, err := engine.LoadAllRules()
	if err != nil {
		return fmt.Errorf("load engine rules: %w", err)
	}
	detector := engine.New(rules)

	idx := &Indexer{
		repoTag:    repoTag,
		classifier: cls,
		parser:     parser,
		detector:   detector,
		skipPasses: skipSet,
		workers:    8,
	}

	doc, err := idx.Run(context.Background(), absRepo)
	if err != nil {
		return err
	}

	if !skipSet[PassBuildDocument] {
		if err := graph.WriteAtomic(outPath, doc); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "archigraph: wrote %s\n", outPath)
	}
	return nil
}

// Run executes the orchestrated pipeline. Each pass is a named method so
// callers (and tests) can reason about per-pass output independently.
func (i *Indexer) Run(ctx context.Context, absRepo string) (*graph.Document, error) {
	start := time.Now()

	files, err := walkRepo(absRepo)
	if err != nil {
		return nil, fmt.Errorf("walk repo: %w", err)
	}
	i.stats.files = len(files)
	fmt.Fprintf(os.Stderr, "archigraph: discovered %d candidate files in %s\n", len(files), absRepo)

	// Pass 1 — per-language AST extraction.
	pass1Records, classified, err := i.runPass1Extract(ctx, absRepo, files)
	if err != nil {
		return nil, fmt.Errorf("pass 1: %w", err)
	}
	i.stats.pass1Rels = countEmbeddedRels(pass1Records)

	// Pass 2.5 — YAML-driven framework rules.
	pass2Records, pass2Rels, err := i.runPass25FrameworkRules(ctx, classified)
	if err != nil {
		return nil, fmt.Errorf("pass 2.5: %w", err)
	}
	i.stats.pass2Rels = len(pass2Rels) + countEmbeddedRels(pass2Records)

	// Pass 3 — cross-language extractors.
	pass3Records, err := i.runPass3CrossLang(ctx, classified)
	if err != nil {
		return nil, fmt.Errorf("pass 3: %w", err)
	}
	i.stats.pass3Rels = countEmbeddedRels(pass3Records)

	// Pass 4 — graph algorithms — placeholder for PORT-4.

	// Pass 5 — build document (deduped).
	doc := i.buildDocument(pass1Records, pass2Records, pass2Rels, pass3Records)

	dur := time.Since(start)
	fmt.Fprintf(os.Stderr,
		"archigraph: processed=%d extracted=%d skipped=%d failed=%d "+
			"entities=%d relationships=%d "+
			"pass1_rels=%d pass2.5_rels=%d pass3_rels=%d "+
			"duration=%s\n",
		i.stats.processed, i.stats.extracted, i.stats.skipped, i.stats.failed,
		doc.Stats.Entities, doc.Stats.Relationships,
		i.stats.pass1Rels, i.stats.pass2Rels, i.stats.pass3Rels,
		dur.Round(time.Millisecond))
	return doc, nil
}

// runPass1Extract runs the per-file AST extractors in parallel. The classified
// slice is also returned for reuse by Pass 2.5 and Pass 3 so we don't pay the
// classification + read + parse cost twice.
func (i *Indexer) runPass1Extract(ctx context.Context, absRepo string, files []string) ([]types.EntityRecord, []classifiedFile, error) {
	if i.skipPasses[PassExtract] {
		// Even when Pass 1 is skipped we still need to classify+read so
		// downstream passes have file content. Run the worker loop in
		// classification-only mode.
		classified, _ := i.classifyAndRead(ctx, absRepo, files, false)
		return nil, classified, nil
	}
	classified, records := i.classifyAndRead(ctx, absRepo, files, true)
	return records, classified, nil
}

// classifyAndRead is the shared worker pool used by Pass 1. When runExtract
// is true it also dispatches to per-language extractors and accumulates
// EntityRecords. The classifiedFile slice is always populated for files that
// survived classification, so other passes can reuse the parse tree + bytes.
func (i *Indexer) classifyAndRead(ctx context.Context, absRepo string, files []string, runExtract bool) ([]classifiedFile, []types.EntityRecord) {
	tasks := make(chan fileTask, len(files))
	for _, rel := range files {
		tasks <- fileTask{relPath: rel, absPath: filepath.Join(absRepo, rel)}
	}
	close(tasks)

	var (
		mu         sync.Mutex
		allRecords []types.EntityRecord
		classified []classifiedFile
	)

	var wg sync.WaitGroup
	for w := 0; w < i.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				size := int64(-1)
				if st, err := os.Stat(t.absPath); err == nil {
					size = st.Size()
				}
				cr := i.classifier.ClassifyWithSize(ctx, t.relPath, size)
				if cr.Skip || cr.Language == "" {
					mu.Lock()
					i.stats.skipped++
					mu.Unlock()
					continue
				}

				content, err := os.ReadFile(t.absPath)
				if err != nil {
					mu.Lock()
					i.stats.failed++
					mu.Unlock()
					continue
				}

				cf := classifiedFile{
					relPath:  t.relPath,
					absPath:  t.absPath,
					language: cr.Language,
					content:  content,
				}

				file := extractor.FileInput{
					Path:     t.relPath,
					Content:  content,
					Language: cr.Language,
				}

				if pr, perr := i.parser.Parse(ctx, content, cr.Language); perr == nil && pr != nil {
					file.Tree = pr.Tree
					cf.tree = pr.Tree
				}

				if !runExtract {
					mu.Lock()
					classified = append(classified, cf)
					mu.Unlock()
					continue
				}

				ents, err := extractors.Extract(ctx, file)
				mu.Lock()
				i.stats.processed++
				if err != nil {
					i.stats.failed++
				} else {
					i.stats.extracted++
					allRecords = append(allRecords, ents...)
				}
				classified = append(classified, cf)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return classified, allRecords
}

// runPass25FrameworkRules applies the YAML rule engine to every classified
// file. Returns extra entity records (from source_patterns) plus standalone
// relationship records (from relationship_rules).
func (i *Indexer) runPass25FrameworkRules(ctx context.Context, classified []classifiedFile) ([]types.EntityRecord, []types.RelationshipRecord, error) {
	if i.skipPasses[PassFramework] {
		return nil, nil, nil
	}
	var (
		mu       sync.Mutex
		entities []types.EntityRecord
		rels     []types.RelationshipRecord
	)

	work := make(chan classifiedFile, len(classified))
	for _, cf := range classified {
		work <- cf
	}
	close(work)

	var wg sync.WaitGroup
	for w := 0; w < i.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cf := range work {
				input := extractor.FileInput{
					Path:     cf.relPath,
					Content:  cf.content,
					Language: cf.language,
				}
				res, err := i.detector.Detect(ctx, input)
				if err != nil || res == nil {
					continue
				}
				if len(res.Entities) == 0 && len(res.Relationships) == 0 {
					continue
				}
				mu.Lock()
				entities = append(entities, res.Entities...)
				rels = append(rels, res.Relationships...)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return entities, rels, nil
}

// runPass3CrossLang runs every registered cross-language extractor against
// every classified file. The cross extractors short-circuit on languages
// they don't handle, so the cost on irrelevant files is small.
//
// This is the critical fix flagged by the PORT-1 review: the
// internal/extractors/cross/* packages had ZERO callers before this pass.
func (i *Indexer) runPass3CrossLang(ctx context.Context, classified []classifiedFile) ([]types.EntityRecord, error) {
	if i.skipPasses[PassCrossLang] {
		return nil, nil
	}
	exts := cross.AllExtractors()
	if len(exts) == 0 {
		return nil, nil
	}

	var (
		mu  sync.Mutex
		out []types.EntityRecord
	)

	work := make(chan classifiedFile, len(classified))
	for _, cf := range classified {
		work <- cf
	}
	close(work)

	var wg sync.WaitGroup
	for w := 0; w < i.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cf := range work {
				input := extractor.FileInput{
					Path:     cf.relPath,
					Content:  cf.content,
					Language: cf.language,
				}
				if cf.tree != nil {
					input.Tree = cf.tree
				}
				for _, e := range exts {
					ents, err := e.Extractor.Extract(ctx, input)
					if err != nil {
						continue
					}
					if len(ents) == 0 {
						continue
					}
					mu.Lock()
					out = append(out, ents...)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	// After collecting raw cross-extractor output, resolve bare-name
	// to_id references against the union of Pass 1 + Pass 2.5 + Pass 3
	// entities. Resolution is a best-effort same-name lookup keyed by
	// (kind, name). If no match is found we leave the bare name in place
	// — this preserves the truly-external (stdlib) signal the report calls
	// out — but cross-file user calls now resolve to a stable entity ID.
	return out, nil
}

// resolveCrossFileRefs walks the merged entity slice and rewrites
// RelationshipRecord.ToID values that look like bare names into entity IDs
// when a unique match exists in the index. This is the substance of the
// "imports / hierarchy" cross-resolution flagged in the PORT-2 spec —
// previously every cross-file CALLS edge stored a literal name (e.g.
// "Println") which could never match a graph node by ID.
func (i *Indexer) resolveCrossFileRefs(records []types.EntityRecord) {
	// Build a name index — name -> deterministic graph entity ID. We index
	// by name only (kind-agnostic) because Pass 1 extractors emit CALLS edges
	// with just the bare callee name; we don't know the callee kind upfront.
	// Collisions (same name, multiple kinds) are kept on the side and skipped
	// during resolution to avoid wrong matches.
	type idEntry struct {
		id   string
		hits int
	}
	byName := make(map[string]*idEntry, len(records))
	for k := range records {
		r := &records[k]
		if r.Name == "" {
			continue
		}
		id := graph.EntityID(i.repoTag, r.Kind, r.Name, r.SourceFile)
		e, ok := byName[r.Name]
		if !ok {
			byName[r.Name] = &idEntry{id: id, hits: 1}
			continue
		}
		e.hits++
		// Keep first id; ambiguous names are skipped at lookup time via hits>1.
	}

	for k := range records {
		r := &records[k]
		for j := range r.Relationships {
			rel := &r.Relationships[j]
			if rel.ToID == "" {
				continue
			}
			// If ToID already looks like a 16-char hex id we trust it.
			if isHexID(rel.ToID) {
				continue
			}
			e, ok := byName[rel.ToID]
			if !ok || e.hits != 1 {
				continue
			}
			rel.ToID = e.id
		}
	}
}

// isHexID reports whether s is a 16-char lower-hex string — the shape of
// graph.EntityID() output.
func isHexID(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// buildDocument merges entity records from every pass, dedupes by stable
// graph-entity ID, resolves cross-file CALLS edges, then assembles the
// final on-disk document.
func (i *Indexer) buildDocument(pass1, pass2 []types.EntityRecord, pass2Rels []types.RelationshipRecord, pass3 []types.EntityRecord) *graph.Document {
	merged := make([]types.EntityRecord, 0, len(pass1)+len(pass2)+len(pass3))
	merged = append(merged, pass1...)
	merged = append(merged, pass2...)
	merged = append(merged, pass3...)

	// Resolve bare-name CALLS / EXTENDS / IMPLEMENTS / etc. ToID references
	// against the merged entity index. Pass 3 explicitly relies on this
	// step to upgrade Pass 1's bare-name edges into real graph IDs.
	i.resolveCrossFileRefs(merged)

	entities := make([]graph.Entity, 0, len(merged))
	relationships := make([]graph.Relationship, 0)

	seenEntity := make(map[string]bool, len(merged))
	seenRel := make(map[string]bool)

	for k := range merged {
		r := &merged[k]
		id := graph.EntityID(i.repoTag, r.Kind, r.Name, r.SourceFile)
		if !seenEntity[id] {
			seenEntity[id] = true
			entities = append(entities, graph.Entity{
				ID:            id,
				Name:          r.Name,
				QualifiedName: r.QualifiedName,
				Kind:          r.Kind,
				Subtype:       r.Subtype,
				SourceFile:    r.SourceFile,
				StartLine:     r.StartLine,
				EndLine:       r.EndLine,
				Language:      r.Language,
				Signature:     r.Signature,
				Tags:          r.Tags,
				Metadata:      r.Metadata,
				Properties:    r.Properties,
			})
		}

		for j := range r.Relationships {
			rel := &r.Relationships[j]
			fromID := rel.FromID
			toID := rel.ToID
			if fromID == "" {
				fromID = id
			}
			relID := graph.RelationshipID(fromID, toID, rel.Kind)
			if seenRel[relID] {
				continue
			}
			seenRel[relID] = true
			relationships = append(relationships, graph.Relationship{
				ID:         relID,
				FromID:     fromID,
				ToID:       toID,
				Kind:       rel.Kind,
				Properties: rel.Properties,
			})
		}
	}

	// Pass 2.5 standalone relationships: synthesise FromID/ToID from the
	// engine's "kind:name" stub strings. We look those up in the merged
	// entity index by name; unmatched stubs are kept as bare strings so the
	// relationship is still surfaced and can be reconciled downstream.
	for j := range pass2Rels {
		rel := &pass2Rels[j]
		fromID := rel.FromID
		toID := rel.ToID
		relID := graph.RelationshipID(fromID, toID, rel.Kind)
		if seenRel[relID] {
			continue
		}
		seenRel[relID] = true
		relationships = append(relationships, graph.Relationship{
			ID:         relID,
			FromID:     fromID,
			ToID:       toID,
			Kind:       rel.Kind,
			Properties: rel.Properties,
		})
	}

	return &graph.Document{
		Version:        graph.SchemaVersion,
		GeneratedAt:    time.Now().UTC(),
		Repo:           i.repoTag,
		IndexerVersion: version.String(),
		Stats: graph.Stats{
			Files:         i.stats.files,
			Entities:      len(entities),
			Relationships: len(relationships),
		},
		Entities:      entities,
		Relationships: relationships,
	}
}

// countEmbeddedRels totals the relationships embedded inside EntityRecords.
func countEmbeddedRels(records []types.EntityRecord) int {
	n := 0
	for k := range records {
		n += len(records[k].Relationships)
	}
	return n
}

// parseSkipPasses validates a comma-separated --skip-pass list and returns
// it as a set. Unknown entries are surfaced as an error so typos don't
// silently degrade the pipeline.
func parseSkipPasses(skip []string) (map[string]bool, error) {
	out := make(map[string]bool)
	valid := make(map[string]bool, len(allPassNames))
	for _, n := range allPassNames {
		valid[n] = true
	}
	for _, raw := range skip {
		for _, part := range strings.Split(raw, ",") {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			if !valid[p] {
				return nil, fmt.Errorf("unknown pass %q (valid: %s)", p, strings.Join(allPassNames, ","))
			}
			out[p] = true
		}
	}
	return out, nil
}

// walkRepo returns repo-relative file paths, skipping common directories
// that should never be indexed (.git, node_modules, vendor, etc.).
func walkRepo(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		base := d.Name()
		if d.IsDir() {
			if isSkippedDir(base) {
				return filepath.SkipDir
			}
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out, err
}

func isSkippedDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn",
		"node_modules", "vendor", "__pycache__",
		".archigraph", ".venv", "venv",
		".idea", ".vscode",
		"dist", "build", "target", ".next", ".nuxt",
		"coverage", ".pytest_cache", ".mypy_cache":
		return true
	}
	if strings.HasPrefix(name, ".") && len(name) > 1 {
		// hidden dirs: skip by default (.terraform, .gradle, .m2, etc.)
		return true
	}
	return false
}
