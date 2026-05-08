package mcp

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/cajasmota/archigraph/internal/graph"
)

// BM25 standard parameters.
const (
	bm25K1 = 1.5
	bm25B  = 0.75
)

// Per-source BM25 weighting (multiplicative on the per-document term-frequency).
const (
	weightLabel     = 1.0
	weightFileStem  = 1.5
	weightPathDirs  = 0.8
	weightDocstring = 0.6
)

// docstringLimitChars caps docstring contribution to the first 200 characters.
const docstringLimitChars = 200

// stopWords is a small English stop list; only applied to docstrings (not
// to labels) per the brief.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "of": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "with": true,
	"is": true, "are": true, "be": true, "this": true, "that": true, "it": true,
	"as": true, "by": true, "from": true, "if": true, "but": true,
}

// docTerms holds the bag-of-words for one entity, with multi-source weighting
// already applied to the term frequencies.
type docTerms struct {
	tf     map[string]float64 // term -> weighted frequency
	length float64            // sum of weighted frequencies (acts as |d|)
}

// BM25Index is a per-repo BM25 index over entities, with multi-source weights.
type BM25Index struct {
	docs      []docTerms
	entities  []*graph.Entity
	df        map[string]int
	avgLen    float64
	totalDocs int
}

// BuildBM25 builds a BM25 index for a single graph document.
func BuildBM25(doc *graph.Document) *BM25Index {
	idx := &BM25Index{
		docs:     make([]docTerms, len(doc.Entities)),
		entities: make([]*graph.Entity, len(doc.Entities)),
		df:       make(map[string]int),
	}
	totalLen := 0.0
	for i := range doc.Entities {
		e := &doc.Entities[i]
		idx.entities[i] = e
		d := buildDocTerms(e)
		idx.docs[i] = d
		totalLen += d.length
		seen := make(map[string]bool, len(d.tf))
		for term := range d.tf {
			if seen[term] {
				continue
			}
			seen[term] = true
			idx.df[term]++
		}
	}
	idx.totalDocs = len(idx.entities)
	if idx.totalDocs > 0 {
		idx.avgLen = totalLen / float64(idx.totalDocs)
	}
	return idx
}

// buildDocTerms computes the weighted bag-of-words for a single entity.
func buildDocTerms(e *graph.Entity) docTerms {
	d := docTerms{tf: map[string]float64{}}
	add := func(s string, weight float64, isDocstring bool) {
		for _, t := range tokenize(s) {
			if isDocstring && stopWords[t] {
				continue
			}
			d.tf[t] += weight
			d.length += weight
		}
	}
	// label
	add(e.Name, weightLabel, false)
	// file stem
	if e.SourceFile != "" {
		stem := strings.TrimSuffix(filepath.Base(e.SourceFile), filepath.Ext(e.SourceFile))
		add(stem, weightFileStem, false)
		// last 2 path dirs
		dir := filepath.Dir(e.SourceFile)
		dirs := []string{}
		for i := 0; i < 2 && dir != "." && dir != "/" && dir != ""; i++ {
			dirs = append(dirs, filepath.Base(dir))
			dir = filepath.Dir(dir)
		}
		add(strings.Join(dirs, " "), weightPathDirs, false)
	}
	// docstring (if any)
	if e.Properties != nil {
		if ds, ok := e.Properties["docstring"]; ok && ds != "" {
			if len(ds) > docstringLimitChars {
				ds = ds[:docstringLimitChars]
			}
			add(ds, weightDocstring, true)
		}
	}
	return d
}

// tokenize lowercases, strips diacritics, and splits camelCase + snake_case
// into tokens. Tokens shorter than 2 chars are dropped.
func tokenize(s string) []string {
	if s == "" {
		return nil
	}
	// Strip diacritics by mapping non-ASCII letters down to their nearest base.
	// We use a simple approach: drop combining marks.
	cleaned := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, s)
	// Split camelCase by inserting a separator before each uppercase that
	// follows a lowercase or digit.
	var b strings.Builder
	var prev rune
	for _, r := range cleaned {
		if (unicode.IsUpper(r)) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		prev = r
	}
	expanded := strings.ToLower(b.String())
	out := []string{}
	cur := strings.Builder{}
	flush := func() {
		if cur.Len() >= 2 {
			out = append(out, cur.String())
		}
		cur.Reset()
	}
	for _, r := range expanded {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

// Hit is a scored entity returned from Search.
type Hit struct {
	Entity *graph.Entity
	Score  float64
}

// Search runs a BM25 query and returns a sorted slice of hits, highest first.
// limit caps the result count; pass 0 for unlimited.
func (b *BM25Index) Search(query string, limit int) []Hit {
	if b == nil || b.totalDocs == 0 {
		return nil
	}
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	hits := make([]Hit, 0, b.totalDocs)
	for i, d := range b.docs {
		score := 0.0
		for _, t := range terms {
			tf, ok := d.tf[t]
			if !ok {
				continue
			}
			df := b.df[t]
			if df == 0 {
				continue
			}
			idf := math.Log(1.0 + (float64(b.totalDocs)-float64(df)+0.5)/(float64(df)+0.5))
			lenNorm := 1.0
			if b.avgLen > 0 {
				lenNorm = 1 - bm25B + bm25B*(d.length/b.avgLen)
			}
			score += idf * (tf * (bm25K1 + 1)) / (tf + bm25K1*lenNorm)
		}
		if score > 0 {
			hits = append(hits, Hit{Entity: b.entities[i], Score: score})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}
