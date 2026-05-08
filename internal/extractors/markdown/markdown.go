// Package markdown implements a clean-room markdown extractor.
//
// It scans .md / .mdx / .markdown files line by line and emits SCOPE entities:
//
//   - SCOPE.Document   — one per file
//   - SCOPE.Heading    — one per ATX heading (#, ##, ..., ######)
//   - SCOPE.CodeBlock  — one per fenced code block (``` or ~~~)
//
// And the following relationships:
//
//   - Document --CONTAINS--> Heading        (every heading in the file)
//   - Heading  --CONTAINS--> Heading        (parent heading wraps deeper-level child)
//   - Heading  --CONTAINS--> CodeBlock      (most recent heading at fence-open)
//   - Heading  --REFERENCES--> <code-slug>  (one per backtick literal in heading text;
//     ToID is a bare slug — a later cross-file
//     pass can resolve it to a real entity ID)
//
// v1 deliberately ignores: setext headings (=== / ---), inline code, links,
// lists, tables, images, blockquotes, HTML blocks. Only ATX headings and
// fenced code blocks are recognised.
//
// REFERENCES resolution: the extractor emits the bare slug as ToID. A
// cross-file post-process step can walk every entity, build a slug→ID map,
// and rewrite REFERENCES.ToID values that match a known slug. If the slug is
// unresolved it stays as the bare slug — agents reading graph.json can still
// interpret the relationship.
//
// Heading slug algorithm (deterministic):
//  1. take the heading text WITHOUT backtick characters (text_stripped)
//  2. lowercase
//  3. replace runs of non-alphanumeric characters (Unicode-aware) with "_"
//  4. trim leading/trailing "_"
//  5. fallback to "heading_<line>" if step 4 leaves an empty string
//
// The extractor registers itself via init() and is wired into the dispatch
// registry by registry_gen.go.
package markdown

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"
	"unicode"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

const langName = "markdown"

func init() {
	extractor.Register(langName, &Extractor{})
}

// Extractor implements extractor.Extractor for markdown.
type Extractor struct{}

// Language returns the canonical language key.
func (e *Extractor) Language() string { return langName }

// Extract scans the file and produces SCOPE entities + relationships.
//
// Pure stdlib — no tree-sitter, no third-party markdown library.
func (e *Extractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("extractor.markdown")
	_, span := tracer.Start(ctx, "extractor.markdown")
	defer span.End()

	span.SetAttributes(attribute.String("file", file.Path))

	if len(file.Content) == 0 {
		span.SetAttributes(
			attribute.Int("entity_count", 0),
			attribute.Int("heading_count", 0),
			attribute.Int("codeblock_count", 0),
		)
		return nil, nil
	}

	lines, err := splitLines(file.Content)
	if err != nil {
		return nil, fmt.Errorf("markdown extractor: %w", err)
	}

	headings, codeBlocks := scan(lines)

	totalLines := len(lines)
	docName := basename(file.Path)
	docQName := file.Path

	// Build SCOPE.Document entity. Its relationships are added below
	// (CONTAINS each heading).
	doc := types.EntityRecord{
		Name:          docName,
		QualifiedName: docQName,
		Kind:          "SCOPE.Document",
		Subtype:       langName,
		Language:      langName,
		SourceFile:    file.Path,
		StartLine:     1,
		EndLine:       totalLines,
	}

	// Compute heading end_lines: end at the line *before* the next sibling-or-higher
	// heading, or EOF.
	for i := range headings {
		end := totalLines
		for j := i + 1; j < len(headings); j++ {
			if headings[j].level <= headings[i].level {
				end = headings[j].line - 1
				break
			}
		}
		if end < headings[i].line {
			end = headings[i].line
		}
		headings[i].endLine = end
	}

	// Build heading entities + Document→Heading CONTAINS edges.
	headingEntities := make([]types.EntityRecord, 0, len(headings))
	for _, h := range headings {
		slug := slugify(h.textStripped, h.line)
		qname := docQName + "::" + slug

		// REFERENCES edges: one per backtick literal in the heading text.
		var rels []types.RelationshipRecord
		for _, lit := range h.backtickLiterals {
			refSlug := slugify(lit, h.line)
			if refSlug == "" {
				continue
			}
			rels = append(rels, types.RelationshipRecord{
				ToID: refSlug,
				Kind: "REFERENCES",
				Properties: map[string]string{
					"reference_text": lit,
					"resolution":     "bare_slug",
				},
			})
		}

		ent := types.EntityRecord{
			Name:          h.textRaw,
			QualifiedName: qname,
			Kind:          "SCOPE.Heading",
			Subtype:       fmt.Sprintf("h%d", h.level),
			Language:      langName,
			SourceFile:    file.Path,
			StartLine:     h.line,
			EndLine:       h.endLine,
			Metadata: map[string]interface{}{
				"level":         h.level,
				"text_raw":      h.textRaw,
				"text_stripped": h.textStripped,
				"slug":          slug,
			},
			Relationships: rels,
		}
		headingEntities = append(headingEntities, ent)

		doc.Relationships = append(doc.Relationships, types.RelationshipRecord{
			ToID: qname,
			Kind: "CONTAINS",
		})
	}

	// Heading hierarchy: each heading CONTAINS subsequent headings of strictly
	// greater level until a sibling-or-higher level is encountered.
	for i, h := range headings {
		parentQName := headingEntities[i].QualifiedName
		for j := i + 1; j < len(headings); j++ {
			if headings[j].level <= h.level {
				break
			}
			// Only emit the edge for *direct* children — i.e. no intervening
			// heading exists at a level strictly between h.level and headings[j].level.
			direct := true
			for k := i + 1; k < j; k++ {
				if headings[k].level > h.level && headings[k].level < headings[j].level {
					direct = false
					break
				}
			}
			if !direct {
				continue
			}
			headingEntities[i].Relationships = append(headingEntities[i].Relationships, types.RelationshipRecord{
				FromID: parentQName,
				ToID:   headingEntities[j].QualifiedName,
				Kind:   "CONTAINS",
			})
		}
	}

	// Build code block entities + attach each to the most recent heading.
	codeEntities := make([]types.EntityRecord, 0, len(codeBlocks))
	for _, cb := range codeBlocks {
		lang := cb.lang
		if lang == "" {
			lang = "unspecified"
		}
		name := fmt.Sprintf("code-block-line-%d-%s", cb.start, lang)
		qname := fmt.Sprintf("%s::block::L%d", docQName, cb.start)

		ent := types.EntityRecord{
			Name:          name,
			QualifiedName: qname,
			Kind:          "SCOPE.CodeBlock",
			Subtype:       lang,
			Language:      langName,
			SourceFile:    file.Path,
			StartLine:     cb.start,
			EndLine:       cb.end,
			Metadata: map[string]interface{}{
				"language":   cb.lang,
				"byte_count": cb.byteCount,
			},
		}
		codeEntities = append(codeEntities, ent)

		// Attach to most recent heading whose start_line < cb.start.
		parentIdx := -1
		for i := range headings {
			if headings[i].line < cb.start {
				parentIdx = i
			} else {
				break
			}
		}
		if parentIdx >= 0 {
			headingEntities[parentIdx].Relationships = append(headingEntities[parentIdx].Relationships, types.RelationshipRecord{
				FromID: headingEntities[parentIdx].QualifiedName,
				ToID:   qname,
				Kind:   "CONTAINS",
			})
		}
	}

	out := make([]types.EntityRecord, 0, 1+len(headingEntities)+len(codeEntities))
	out = append(out, doc)
	out = append(out, headingEntities...)
	out = append(out, codeEntities...)

	span.SetAttributes(
		attribute.Int("entity_count", len(out)),
		attribute.Int("heading_count", len(headingEntities)),
		attribute.Int("codeblock_count", len(codeEntities)),
	)
	return out, nil
}

// heading is an internal record collected during scan.
type heading struct {
	line             int
	endLine          int
	level            int
	textRaw          string // heading text without leading "#" markers, with backticks preserved
	textStripped     string // textRaw with "`" removed
	backtickLiterals []string
}

// codeBlock is an internal record for a fenced code block.
type codeBlock struct {
	start     int
	end       int
	lang      string
	byteCount int
}

// scan walks lines once and emits headings + code blocks.
//
// While inside a fenced block, ATX heading lines are NOT recognised.
// A code-fence opener determines the closing fence token (``` or ~~~) and
// the minimum length (>=3, must be matched by a closer of equal-or-greater length
// using the same character).
func scan(lines []string) ([]heading, []codeBlock) {
	var headings []heading
	var codeBlocks []codeBlock

	inFence := false
	var fenceChar byte
	var fenceLen int
	var fenceStart int
	var fenceLang string
	var fenceBytes int

	for i, line := range lines {
		lineNo := i + 1

		if inFence {
			if isFenceClose(line, fenceChar, fenceLen) {
				codeBlocks = append(codeBlocks, codeBlock{
					start:     fenceStart,
					end:       lineNo,
					lang:      fenceLang,
					byteCount: fenceBytes,
				})
				inFence = false
				fenceLang = ""
				fenceBytes = 0
				continue
			}
			fenceBytes += len(line) + 1 // +1 for newline
			continue
		}

		// Outside a fence — check for new fence open.
		if ch, n, lang, ok := parseFenceOpen(line); ok {
			inFence = true
			fenceChar = ch
			fenceLen = n
			fenceStart = lineNo
			fenceLang = lang
			fenceBytes = 0
			continue
		}

		// Check for ATX heading.
		if h, ok := parseATXHeading(line, lineNo); ok {
			headings = append(headings, h)
		}
	}

	// Unclosed fence: terminate at EOF.
	if inFence {
		codeBlocks = append(codeBlocks, codeBlock{
			start:     fenceStart,
			end:       len(lines),
			lang:      fenceLang,
			byteCount: fenceBytes,
		})
	}

	return headings, codeBlocks
}

// parseATXHeading recognises lines like "# text", "## text", up to "######".
// CommonMark requires:
//   - up to 3 leading spaces
//   - 1-6 "#" characters
//   - at least one space (or end of line) after the "#"s
//   - optional trailing run of "#" with whitespace before is stripped
//
// Returns (heading, true) on match.
func parseATXHeading(line string, lineNo int) (heading, bool) {
	// Strip up to 3 leading spaces (no tabs allowed for ATX per CommonMark).
	s := line
	indent := 0
	for indent < 3 && indent < len(s) && s[indent] == ' ' {
		indent++
	}
	s = s[indent:]

	// Count leading '#' (1-6).
	level := 0
	for level < 7 && level < len(s) && s[level] == '#' {
		level++
	}
	if level < 1 || level > 6 {
		return heading{}, false
	}
	rest := s[level:]
	// After the # run, require either end-of-line or a space/tab.
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return heading{}, false
	}

	// Trim leading whitespace from rest.
	text := strings.TrimLeft(rest, " \t")

	// Strip optional trailing closing-sequence: a run of '#' preceded by whitespace
	// (or at start), followed only by whitespace.
	text = stripTrailingHashes(text)

	stripped := strings.ReplaceAll(text, "`", "")
	literals := extractBacktickLiterals(text)

	return heading{
		line:             lineNo,
		level:            level,
		textRaw:          text,
		textStripped:     stripped,
		backtickLiterals: literals,
	}, true
}

// stripTrailingHashes implements the CommonMark optional trailing "#" closing
// sequence: a run of "#" preceded by whitespace and followed only by whitespace
// is removed (CommonMark §4.2).
func stripTrailingHashes(s string) string {
	t := strings.TrimRight(s, " \t")
	if t == "" {
		return t
	}
	// Find a trailing "#"-run.
	end := len(t)
	hashes := 0
	for hashes < end && t[end-1-hashes] == '#' {
		hashes++
	}
	if hashes == 0 {
		return t
	}
	cutoff := end - hashes
	if cutoff == 0 {
		// The whole line is "#"s — that's the entire text; drop it (matches CommonMark).
		return ""
	}
	if t[cutoff-1] != ' ' && t[cutoff-1] != '\t' {
		return t // hashes not preceded by whitespace; keep as-is
	}
	return strings.TrimRight(t[:cutoff], " \t")
}

// parseFenceOpen recognises a fenced-code-block opening line.
// Returns (fenceChar, fenceLen, infoString, true) on match.
//
// CommonMark §4.5: a code fence is a sequence of at least three consecutive
// backticks (`) or tildes (~). Up to 3 leading spaces are allowed. The info
// string is the text after the fence (used as the language tag here — first
// whitespace-separated token).
//
// We also forbid backtick-fence info strings from containing additional
// backticks (per CommonMark) — such lines are treated as plain text.
func parseFenceOpen(line string) (byte, int, string, bool) {
	s := line
	indent := 0
	for indent < 3 && indent < len(s) && s[indent] == ' ' {
		indent++
	}
	s = s[indent:]
	if s == "" {
		return 0, 0, "", false
	}
	ch := s[0]
	if ch != '`' && ch != '~' {
		return 0, 0, "", false
	}
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	if n < 3 {
		return 0, 0, "", false
	}
	info := strings.TrimSpace(s[n:])
	if ch == '`' && strings.ContainsRune(info, '`') {
		return 0, 0, "", false
	}
	// First whitespace-separated token is the language.
	lang := info
	if idx := strings.IndexAny(lang, " \t"); idx >= 0 {
		lang = lang[:idx]
	}
	return ch, n, lang, true
}

// isFenceClose reports whether a line closes a fence opened with `ch` of
// length `openLen`. Closer must be the same char, length >= openLen, and may
// only be followed by whitespace.
func isFenceClose(line string, ch byte, openLen int) bool {
	s := line
	indent := 0
	for indent < 3 && indent < len(s) && s[indent] == ' ' {
		indent++
	}
	s = s[indent:]
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	if n < openLen {
		return false
	}
	rest := s[n:]
	for i := 0; i < len(rest); i++ {
		if rest[i] != ' ' && rest[i] != '\t' {
			return false
		}
	}
	return true
}

// extractBacktickLiterals returns each `...` literal in heading text (no
// nesting, single-backtick form only — sufficient for v1).
func extractBacktickLiterals(s string) []string {
	var out []string
	for {
		i := strings.IndexByte(s, '`')
		if i < 0 {
			return out
		}
		j := strings.IndexByte(s[i+1:], '`')
		if j < 0 {
			return out
		}
		lit := s[i+1 : i+1+j]
		if lit != "" {
			out = append(out, lit)
		}
		s = s[i+2+j:]
	}
}

// slugify implements the heading slug algorithm documented in the package
// comment. Deterministic: same input → same output.
func slugify(text string, lineNo int) string {
	lower := strings.ToLower(text)
	var b strings.Builder
	b.Grow(len(lower))
	prevUnderscore := false
	for _, r := range lower {
		if isAlphaNum(r) {
			b.WriteRune(r)
			prevUnderscore = false
		} else {
			if !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return fmt.Sprintf("heading_%d", lineNo)
	}
	return out
}

func isAlphaNum(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// splitLines splits content into lines, accepting LF, CRLF, and CR endings.
// The trailing line (no newline) is included if non-empty.
func splitLines(content []byte) ([]string, error) {
	// Normalise: scanner with default split treats LF/CRLF correctly when
	// we strip the trailing CR ourselves.
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		// scanner already drops the trailing \n; strip a trailing \r if present
		// (CRLF input).
		line = strings.TrimRight(line, "\r")
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// basename returns the final path component of p (forward-slash separated).
func basename(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}
