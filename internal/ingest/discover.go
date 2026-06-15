package ingest

import (
	"path/filepath"
	"strings"
)

// excludedDirSegments are path segments whose presence anywhere in a file's
// repo-relative path excludes it from markdown ingestion. These mirror the
// indexer's general vendored/build excludes — documentation living under them
// is third-party or generated and would pollute the graph.
var excludedDirSegments = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".git":         true,
	".grafel":      true,
}

// DiscoverMarkdown filters relPaths (repo-relative slash paths, e.g. the
// indexer's walked file list) down to the markdown files eligible for
// ingestion: a ".md" or ".markdown" extension (case-insensitive) and no
// excluded directory segment. The returned slice is sorted for determinism.
func DiscoverMarkdown(relPaths []string) []string {
	return discover(relPaths, isMarkdownPath)
}

// DiscoverDocs filters relPaths down to ALL ingestible documentation files:
// markdown (*.md, *.markdown) AND PDF (*.pdf), case-insensitive, excluding
// vendored/build directories. This is the entrypoint the indexer uses so PDFs
// flow through the same Document/Section pipeline as markdown; Ingest dispatches
// each path to the right parser by extension.
func DiscoverDocs(relPaths []string) []string {
	return discover(relPaths, isIngestiblePath)
}

func discover(relPaths []string, want func(string) bool) []string {
	var out []string
	for _, p := range relPaths {
		rel := filepath.ToSlash(p)
		if !want(rel) {
			continue
		}
		if hasExcludedSegment(rel) {
			continue
		}
		out = append(out, rel)
	}
	return out
}

func isMarkdownPath(rel string) bool {
	ext := strings.ToLower(filepath.Ext(rel))
	return ext == ".md" || ext == ".markdown"
}

func isPDFPath(rel string) bool {
	return strings.ToLower(filepath.Ext(rel)) == ".pdf"
}

func isIngestiblePath(rel string) bool {
	return isMarkdownPath(rel) || isPDFPath(rel)
}

func hasExcludedSegment(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if excludedDirSegments[seg] {
			return true
		}
	}
	return false
}
