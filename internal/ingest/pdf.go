package ingest

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
)

// pdf.go adds PDF text-extraction to the deterministic doc-ingestion pipeline
// (the PDF axis of epic #4294). It is the structural sibling of markdown.go:
// ParsePDF reads a *.pdf byte buffer and produces the SAME Document/[]Section
// shape ParseDocument produces for markdown, so the existing MENTIONS linker
// (#4307) and the Layer-2 emit/apply path (#4308/#4309) operate on PDFs with
// NO change.
//
// It is FULLY DETERMINISTIC and OFFLINE: the only dependency is the pure-Go,
// zero-transitive-dep github.com/ledongthuc/pdf text extractor (no cgo, no
// network, no API keys). PDFs have no markdown-style heading syntax, so
// sections are derived by font-size / all-caps / numbered-heading heuristics
// over the extracted text-span geometry, with an honest per-page-section
// fallback when no headings are recoverable.
//
// Honest limits (documented, never a crash):
//   - Scanned / image-only PDFs (no text layer): ParsePDF returns a Document
//     with a note property and ZERO sections (there is nothing to extract).
//   - Encrypted PDFs: ParsePDF returns an error so the caller can skip + log a
//     warning (we never prompt for or guess a password).
//   - Malformed PDFs that make the underlying extractor panic are recovered
//     into an ordinary error rather than crashing the indexer.

// pdfNoteScanned is stamped on the Document when a PDF parsed but yielded no
// extractable text (almost always a scanned / image-only PDF with no text
// layer). Callers may surface it; the doc node still exists with zero sections.
const pdfNoteScanned = "no text layer (scanned/image-only PDF); zero sections extracted"

// pdfLine is one logical line of extracted text on a page: the spans that share
// (approximately) a baseline, left-to-right, with the line's dominant font size.
type pdfLine struct {
	text     string
	fontSize float64
	page     int
	// lineNo is the 1-indexed sequential logical line number across the whole
	// document (assigned in reading order). It plays the role markdown line
	// numbers play: Section.StartLine/EndLine spans and stable section IDs.
	lineNo int
}

// ParsePDF parses one PDF file's bytes into a Document plus a flat,
// reading-order slice of Section nodes whose ParentIndex fields encode the
// derived heading hierarchy, matching ParseDocument's contract.
//
// relPath is stored verbatim on the returned Document. An error is returned
// ONLY for inputs that cannot be parsed at all (encrypted, corrupt, not a PDF);
// a valid PDF with no text layer returns a Document (with pdfNoteScanned) and
// no sections, and nil error.
//
// Determinism: extraction order is page-ascending, then top-to-bottom,
// then left-to-right; identical bytes always yield identical lines, sections
// and IDs.
func ParsePDF(relPath string, content []byte) (doc Document, sections []Section, err error) {
	// The underlying extractor can panic on malformed object streams. Recover
	// into an error so a single bad PDF never takes down the index.
	defer func() {
		if r := recover(); r != nil {
			doc = Document{}
			sections = nil
			err = fmt.Errorf("parse pdf %q: %v", relPath, r)
		}
	}()

	r, err := pdflib.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		// Encrypted PDFs surface here ("encrypted PDF" / missing password), as
		// do non-PDF / corrupt inputs. The caller skips + logs.
		return Document{}, nil, fmt.Errorf("parse pdf %q: %w", relPath, err)
	}

	lines := extractLines(r)

	doc = Document{RelPath: relPath, LineCount: len(lines)}
	if len(lines) == 0 {
		// Valid PDF, but no recoverable text layer (scanned/image-only). Honest
		// empty result with a note; NOT an error and NOT a crash.
		doc.Note = pdfNoteScanned
		return doc, nil, nil
	}

	bodySize := dominantFontSize(lines)
	sections = sectionize(lines, bodySize)

	// Title: the heading text of the first top-level section, mirroring how the
	// markdown parser uses the first H1. Purely informational.
	for i := range sections {
		if sections[i].Depth == 1 {
			doc.Title = sections[i].HeadingText
			break
		}
	}
	return doc, sections, nil
}

// extractLines pulls every text span from every page in reading order and
// groups spans into logical lines. Pages are processed in ascending order;
// within a page, spans are grouped into lines by Y proximity (PDF Y increases
// upward, so larger Y is higher on the page) and ordered top-to-bottom, then
// left-to-right within a line. Each line records its page number, dominant
// (max) font size, and a document-global 1-indexed line number.
func extractLines(r *pdflib.Reader) []pdfLine {
	var out []pdfLine
	lineNo := 0
	n := r.NumPage()
	for pageNum := 1; pageNum <= n; pageNum++ {
		p := r.Page(pageNum)
		if p.V.IsNull() {
			continue
		}
		spans := p.Content().Text
		if len(spans) == 0 {
			continue
		}
		// Bucket spans into lines by baseline Y. A small tolerance absorbs
		// sub-pixel baseline jitter within a visual line.
		type bucket struct {
			y     float64
			spans []pdflib.Text
		}
		var buckets []*bucket
		const yTol = 3.0
		for _, t := range spans {
			// Keep whitespace-only spans: many PDFs (especially those without an
			// embedded font Widths array, where W is 0) encode inter-word spaces
			// as their OWN space spans. Dropping them here would glue adjacent
			// words ("OrderServiceDesign") and break whole-token mention linking.
			// Empty (zero-length) spans carry nothing and are skipped.
			if t.S == "" {
				continue
			}
			placed := false
			for _, b := range buckets {
				if math.Abs(b.y-t.Y) <= yTol {
					b.spans = append(b.spans, t)
					placed = true
					break
				}
			}
			if !placed {
				buckets = append(buckets, &bucket{y: t.Y, spans: []pdflib.Text{t}})
			}
		}
		// Top-to-bottom: larger Y first.
		sort.SliceStable(buckets, func(a, b int) bool { return buckets[a].y > buckets[b].y })
		for _, b := range buckets {
			// Left-to-right within the line.
			sort.SliceStable(b.spans, func(i, j int) bool { return b.spans[i].X < b.spans[j].X })
			var sb strings.Builder
			var maxSize float64
			prevEnd := math.Inf(-1)
			for _, t := range b.spans {
				// Insert a space when a visible horizontal gap separates spans
				// that the extractor emitted without one.
				if sb.Len() > 0 && t.X-prevEnd > 1.0 && !strings.HasSuffix(sb.String(), " ") {
					sb.WriteByte(' ')
				}
				sb.WriteString(t.S)
				prevEnd = t.X + t.W
				if t.FontSize > maxSize {
					maxSize = t.FontSize
				}
			}
			text := strings.TrimSpace(collapseSpaces(sb.String()))
			if text == "" {
				continue
			}
			lineNo++
			out = append(out, pdfLine{text: text, fontSize: maxSize, page: pageNum, lineNo: lineNo})
		}
	}
	return out
}

// dominantFontSize returns the most common (rounded) font size across lines —
// the body-text size against which heading sizes are compared. Returns 0 when
// no sizes are available (font info absent); callers then rely on the
// non-size heuristics (all-caps / numbered) and the per-page fallback.
func dominantFontSize(lines []pdfLine) float64 {
	counts := map[float64]int{}
	for _, l := range lines {
		if l.fontSize <= 0 {
			continue
		}
		counts[math.Round(l.fontSize*2)/2]++ // round to nearest 0.5pt
	}
	var best float64
	var bestN int
	// Deterministic: iterate sorted keys.
	keys := make([]float64, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Float64s(keys)
	for _, k := range keys {
		if counts[k] > bestN {
			bestN, best = counts[k], k
		}
	}
	return best
}

// sectionize converts the ordered lines into a Section tree. A line is treated
// as a heading when isHeading reports so; its derived depth (1 = largest
// heading, deeper for smaller headings) builds the CONTAINS hierarchy exactly
// like markdown ATX depth. Non-heading lines accumulate into the current
// section's Body.
//
// Fallback: if NO headings are recovered anywhere, sections are emitted
// per-page (page N → one Section titled "Page N") — an honest, deterministic
// structure when the PDF carries no recoverable heading signal.
func sectionize(lines []pdfLine, bodySize float64) []Section {
	// First pass: classify and collect heading sizes so depth can be assigned
	// by size rank (largest heading size = depth 1).
	var headingSizes []float64
	isHead := make([]bool, len(lines))
	for i, l := range lines {
		if isHeading(l, bodySize) {
			isHead[i] = true
			headingSizes = append(headingSizes, roundHalf(l.fontSize))
		}
	}

	anyHeading := false
	for _, h := range isHead {
		if h {
			anyHeading = true
			break
		}
	}
	if !anyHeading {
		return perPageSections(lines)
	}

	// Map distinct heading sizes (descending) to depths 1,2,3,... Larger font =
	// shallower depth. Sizes that tie share a depth.
	uniq := dedupDescending(headingSizes)
	depthOf := map[float64]int{}
	for i, s := range uniq {
		depthOf[s] = i + 1
	}
	var sections []Section
	var stack []int // indices of open ancestor sections (increasing depth)
	var curBody strings.Builder

	flushBody := func() {
		if len(sections) == 0 {
			return
		}
		last := &sections[len(sections)-1]
		last.Body = strings.TrimRight(curBody.String(), "\n")
		curBody.Reset()
	}

	closeTo := func(depth, endLine int) {
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			if sections[top].Depth < depth {
				break
			}
			sections[top].EndLine = endLine
			stack = stack[:len(stack)-1]
		}
	}

	// Leading lines before the first heading (e.g. a cover line) form a synthetic
	// depth-1 "preamble" section so their text is still linkable, mirroring how
	// markdown keeps pre-heading text addressable.
	for i, l := range lines {
		if isHead[i] {
			depth := depthOf[roundHalf(l.fontSize)]
			if depth == 0 {
				depth = 1
			}
			flushBody()
			closeTo(depth, l.lineNo-1)
			parent := -1
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			sections = append(sections, Section{
				HeadingText: l.text,
				Depth:       depth,
				StartLine:   l.lineNo,
				EndLine:     lastLineNo(lines),
				ParentIndex: parent,
				Page:        l.page,
			})
			stack = append(stack, len(sections)-1)
			continue
		}
		// Body line.
		if len(sections) == 0 {
			// Pre-heading preamble: open an implicit depth-1 section once.
			sections = append(sections, Section{
				HeadingText: "",
				Depth:       1,
				StartLine:   l.lineNo,
				EndLine:     lastLineNo(lines),
				ParentIndex: -1,
				Page:        l.page,
			})
			stack = append(stack, 0)
		}
		curBody.WriteString(l.text)
		curBody.WriteByte('\n')
	}
	flushBody()
	for len(stack) > 0 {
		top := stack[len(stack)-1]
		sections[top].EndLine = lastLineNo(lines)
		stack = stack[:len(stack)-1]
	}
	return sections
}

// perPageSections is the honest fallback: one depth-1 Section per page, titled
// "Page N", whose body is that page's full text. Used when no heading signal is
// recoverable.
func perPageSections(lines []pdfLine) []Section {
	var sections []Section
	curPage := -1
	var body strings.Builder
	flush := func() {
		if len(sections) == 0 {
			return
		}
		sections[len(sections)-1].Body = strings.TrimRight(body.String(), "\n")
		body.Reset()
	}
	for _, l := range lines {
		if l.page != curPage {
			flush()
			curPage = l.page
			sections = append(sections, Section{
				HeadingText: fmt.Sprintf("Page %d", l.page),
				Depth:       1,
				StartLine:   l.lineNo,
				EndLine:     lastLineNo(lines),
				ParentIndex: -1,
				Page:        l.page,
			})
		}
		// Close the previous section's span at the line before this page starts.
		if len(sections) >= 2 && sections[len(sections)-1].StartLine == l.lineNo {
			sections[len(sections)-2].EndLine = l.lineNo - 1
		}
		body.WriteString(l.text)
		body.WriteByte('\n')
	}
	flush()
	return sections
}

// isHeading reports whether a line should start a new section. The heuristics
// are precision-leaning (a false heading just over-splits; it never invents
// text), and any one of them suffices:
//   - Font size clearly larger than body text (>= 1.15x), with a real body size.
//   - A short, all-caps line (title-case section banners; <= 8 words).
//   - A numbered-heading prefix ("1.", "1.2", "1.2.3", "Chapter 4", "Section 2").
func isHeading(l pdfLine, bodySize float64) bool {
	t := strings.TrimSpace(l.text)
	if t == "" {
		return false
	}
	words := strings.Fields(t)
	// Long paragraphs are never headings regardless of size.
	if len(words) > 14 {
		return false
	}
	if bodySize > 0 && l.fontSize >= bodySize*1.15 {
		return true
	}
	if numberedHeading(t) {
		return true
	}
	if len(words) <= 8 && isAllCaps(t) {
		return true
	}
	return false
}

// numberedHeading matches common numbered/section heading prefixes.
func numberedHeading(t string) bool {
	// "1.", "1.2", "12.3.4", optionally followed by heading text.
	i := 0
	sawDigit := false
	for i < len(t) && (t[i] >= '0' && t[i] <= '9' || t[i] == '.') {
		if t[i] >= '0' && t[i] <= '9' {
			sawDigit = true
		}
		i++
	}
	if sawDigit && i > 0 {
		// Require a separator (space/end) so "1.5x" prose isn't a heading.
		if i == len(t) {
			return strings.Contains(t[:i], ".")
		}
		if t[i] == ' ' {
			return true
		}
	}
	low := strings.ToLower(t)
	for _, p := range []string{"chapter ", "section ", "appendix ", "part "} {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}

// isAllCaps reports whether t has letters and every cased letter is uppercase.
func isAllCaps(t string) bool {
	hasUpper := false
	for _, r := range t {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			hasUpper = true
		}
	}
	return hasUpper
}

// collapseSpaces collapses runs of whitespace to a single space.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func roundHalf(f float64) float64 { return math.Round(f*2) / 2 }

// dedupDescending returns the distinct values of sizes sorted largest-first.
func dedupDescending(sizes []float64) []float64 {
	seen := map[float64]bool{}
	var out []float64
	for _, s := range sizes {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(out)))
	return out
}

func lastLineNo(lines []pdfLine) int {
	if len(lines) == 0 {
		return 1
	}
	return lines[len(lines)-1].lineNo
}
