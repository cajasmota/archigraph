// Package bicep implements a regex/line-based extractor for Azure Bicep
// infrastructure-as-code files (*.bicep).
//
// No tree-sitter grammar for Bicep is vendored in this repo, so — like the
// COBOL/CDK regex detectors — this extractor parses raw source text. Bicep is
// structurally HCL-like (resource / module / param / var / output declarations
// with symbolic-name cross-references), so the entity + edge model mirrors the
// HCL/Terraform extractor (internal/extractors/hcl):
//
//	resource <symbolicName> 'Microsoft.Storage/storageAccounts@2022-09-01' = {…}
//	    → SCOPE.InfraResource (Kind via bicepResourceKind on the AzureRP type),
//	      named by symbolicName; the deployed name: '…' is folded into Metadata.
//	module  <name> '<path>.bicep' = {…}
//	    → SCOPE.Component/module + IMPORTS edge to the referenced .bicep path.
//	param   <name> <type>            → SCOPE.Schema/param
//	var     <name> = …               → SCOPE.Schema/var
//	output  <name> <type> = …        → SCOPE.Schema/output
//
// Relationships (reusing the HCL DEPENDS_ON edge kind):
//
//	symbolic-name references in a resource/module body
//	    (storageAccount.id, storageAccount.properties.x, foo.outputs.y) → DEPENDS_ON
//	explicit  dependsOn: [a, b]      → DEPENDS_ON to each listed symbolic name
//
// `existing` resources and `[for item in items: {…}]` loop bodies are handled:
// the resource is still emitted (with metadata flags) and references inside a
// loop body are still attributed to the declaring resource.
//
// Edges are emitted as Format-A structural-refs tied to the current file
// (BuildOperationStructuralRef) so the resolver binds them via byLocation to
// the sibling entity in the same file — exactly as the HCL extractor does for
// depends_on.
//
// OTel span: "indexer.extract.bicep" with attributes language, file_line_count,
// entity_count.
//
// Registered under the "bicep" language key via init(); .bicep files are routed
// here by internal/classifier (extension → "bicep").
package bicep

import (
	"bytes"
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("bicep", &Extractor{})
}

// Extractor implements extractor.Extractor for Azure Bicep.
type Extractor struct{}

// Language implements extractor.Extractor.
func (e *Extractor) Language() string { return "bicep" }

const lang = "bicep"

var (
	// resource <symbolicName> 'Microsoft.Storage/storageAccounts@2022-09-01' [existing] = {
	// The trailing portion (existing / = / =[for …]) is matched loosely so the
	// declaration line is recognised regardless of loop / existing modifiers.
	reResource = regexp.MustCompile(`(?m)^\s*resource\s+([A-Za-z_][A-Za-z0-9_]*)\s+'([^']+)'\s*(existing)?\s*=`)
	// module <name> '<path>' = {
	reModule = regexp.MustCompile(`(?m)^\s*module\s+([A-Za-z_][A-Za-z0-9_]*)\s+'([^']+)'\s*=`)
	// param <name> <type> [= default]
	reParam = regexp.MustCompile(`(?m)^\s*param\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z0-9_<>\[\]]+)`)
	// var <name> = …
	reVar = regexp.MustCompile(`(?m)^\s*var\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	// output <name> <type> = …
	reOutput = regexp.MustCompile(`(?m)^\s*output\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z0-9_<>\[\]]+)\s*=`)

	// name: 'literal' — the deployed Azure resource name inside a body.
	reNameAttr = regexp.MustCompile(`(?m)^\s*name:\s*'([^']*)'`)
	// dependsOn: [ … ] — explicit dependency list (may span lines).
	reDependsOn = regexp.MustCompile(`(?s)dependsOn:\s*\[(.*?)\]`)
)

// Extract implements extractor.Extractor. Never panics; returns partial
// results on malformed input.
func (e *Extractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("extractor.bicep")
	_, span := tracer.Start(ctx, "indexer.extract.bicep")
	defer span.End()

	lineCount := 0
	if len(file.Content) > 0 {
		lineCount = bytes.Count(file.Content, []byte{'\n'}) + 1
	}

	if len(file.Content) == 0 {
		span.SetAttributes(
			attribute.String("language", lang),
			attribute.Int("file_line_count", 0),
			attribute.Int("entity_count", 0),
		)
		return nil, nil
	}

	src := string(file.Content)
	path := file.Path

	// Pre-scan: collect the set of symbolic names declared as resources or
	// modules in this file so reference extraction only emits DEPENDS_ON edges
	// to real local declarations (filters out built-in functions, params, etc.).
	symbolic := collectSymbolicNames(src)

	var records []types.EntityRecord

	records = append(records, extractResources(src, path, symbolic)...)
	records = append(records, extractModules(src, path, symbolic)...)
	records = append(records, extractParams(src, path)...)
	records = append(records, extractVars(src, path)...)
	records = append(records, extractOutputs(src, path)...)

	span.SetAttributes(
		attribute.String("language", lang),
		attribute.Int("file_line_count", lineCount),
		attribute.Int("entity_count", len(records)),
	)
	return records, nil
}

// collectSymbolicNames returns the set of resource/module symbolic names
// declared in the file.
func collectSymbolicNames(src string) map[string]bool {
	out := map[string]bool{}
	for _, m := range reResource.FindAllStringSubmatch(src, -1) {
		out[m[1]] = true
	}
	for _, m := range reModule.FindAllStringSubmatch(src, -1) {
		out[m[1]] = true
	}
	return out
}

// extractResources emits a SCOPE.InfraResource per `resource` declaration and
// its DEPENDS_ON edges (symbolic-name references + explicit dependsOn).
func extractResources(src, path string, symbolic map[string]bool) []types.EntityRecord {
	var out []types.EntityRecord
	locs := reResource.FindAllStringSubmatchIndex(src, -1)
	for i, loc := range locs {
		symName := src[loc[2]:loc[3]]
		azureType := src[loc[4]:loc[5]]
		isExisting := loc[6] >= 0 // optional `existing` group matched

		startLine := lineOf(src, loc[0])
		// Body spans from this declaration to the next top-level declaration.
		bodyStart := loc[1]
		bodyEnd := len(src)
		if i+1 < len(locs) {
			bodyEnd = locs[i+1][0]
		}
		// Clamp against the next module declaration too (resources/modules are
		// interleaved); find the earliest following declaration boundary.
		bodyEnd = nextDeclBoundary(src, bodyStart, bodyEnd)
		body := src[bodyStart:bodyEnd]
		endLine := startLine + strings.Count(body, "\n")

		rpType, apiVersion := splitAzureType(azureType)

		category := bicepResourceCoarseScope(rpType)
		meta := map[string]interface{}{
			"subtype":           "resource",
			"iac_tool":          "bicep",
			"symbolic_name":     symName,
			"azure_rp_type":     rpType,
			"resource_category": category,
			// resource_scope kept (== resource_category) for back-compat.
			"resource_scope": category,
		}
		if apiVersion != "" {
			meta["api_version"] = apiVersion
		}
		if isExisting {
			meta["existing"] = "true"
		}
		if nm := reNameAttr.FindStringSubmatch(body); nm != nil {
			meta["deployed_name"] = nm[1]
		}
		if isLoop(body) {
			meta["loop"] = "true"
		}

		rec := types.EntityRecord{
			Name:          symName,
			Kind:          "SCOPE.InfraResource",
			Subtype:       "resource",
			SourceFile:    path,
			StartLine:     startLine,
			EndLine:       endLine,
			Language:      lang,
			QualityScore:  0.9,
			QualifiedName: rpType + "." + symName,
			Metadata:      meta,
		}
		rec.Relationships = dependencyEdges(body, path, symName, symbolic)
		out = append(out, rec)
	}
	return out
}

// extractModules emits a SCOPE.Component/module per `module` declaration with
// an IMPORTS edge to the referenced .bicep path plus DEPENDS_ON edges.
func extractModules(src, path string, symbolic map[string]bool) []types.EntityRecord {
	var out []types.EntityRecord
	locs := reModule.FindAllStringSubmatchIndex(src, -1)
	for i, loc := range locs {
		modName := src[loc[2]:loc[3]]
		modPath := src[loc[4]:loc[5]]

		startLine := lineOf(src, loc[0])
		bodyStart := loc[1]
		bodyEnd := len(src)
		if i+1 < len(locs) {
			bodyEnd = locs[i+1][0]
		}
		bodyEnd = nextDeclBoundary(src, bodyStart, bodyEnd)
		body := src[bodyStart:bodyEnd]
		endLine := startLine + strings.Count(body, "\n")

		meta := map[string]interface{}{
			"subtype":  "module",
			"iac_tool": "bicep",
			"label":    modName,
			"source":   modPath,
		}
		if nm := reNameAttr.FindStringSubmatch(body); nm != nil {
			meta["deployed_name"] = nm[1]
		}
		if isLoop(body) {
			meta["loop"] = "true"
		}

		rec := types.EntityRecord{
			Name:          modName,
			Kind:          "SCOPE.Component",
			Subtype:       "module",
			SourceFile:    path,
			StartLine:     startLine,
			EndLine:       endLine,
			Language:      lang,
			QualityScore:  0.9,
			QualifiedName: modPath,
			Metadata:      meta,
		}

		// IMPORTS edge to the referenced .bicep module path.
		rec.Relationships = append(rec.Relationships, types.RelationshipRecord{
			FromID: path,
			ToID:   "scope:component:file:bicep:" + modPath,
			Kind:   "IMPORTS",
		})
		rec.Relationships = append(rec.Relationships, dependencyEdges(body, path, modName, symbolic)...)
		out = append(out, rec)
	}
	return out
}

// extractParams emits a SCOPE.Schema/param per `param` declaration.
func extractParams(src, path string) []types.EntityRecord {
	var out []types.EntityRecord
	for _, loc := range reParam.FindAllStringSubmatchIndex(src, -1) {
		name := src[loc[2]:loc[3]]
		paramType := src[loc[4]:loc[5]]
		out = append(out, types.EntityRecord{
			Name:         "param." + name,
			Kind:         "SCOPE.Schema",
			Subtype:      "param",
			SourceFile:   path,
			StartLine:    lineOf(src, loc[0]),
			EndLine:      lineOf(src, loc[0]),
			Language:     lang,
			QualityScore: 0.8,
			Metadata: map[string]interface{}{
				"subtype":    "param",
				"iac_tool":   "bicep",
				"label":      name,
				"param_type": paramType,
			},
		})
	}
	return out
}

// extractVars emits a SCOPE.Schema/var per `var` declaration.
func extractVars(src, path string) []types.EntityRecord {
	var out []types.EntityRecord
	for _, loc := range reVar.FindAllStringSubmatchIndex(src, -1) {
		name := src[loc[2]:loc[3]]
		out = append(out, types.EntityRecord{
			Name:         "var." + name,
			Kind:         "SCOPE.Schema",
			Subtype:      "var",
			SourceFile:   path,
			StartLine:    lineOf(src, loc[0]),
			EndLine:      lineOf(src, loc[0]),
			Language:     lang,
			QualityScore: 0.8,
			Metadata: map[string]interface{}{
				"subtype":  "var",
				"iac_tool": "bicep",
				"label":    name,
			},
		})
	}
	return out
}

// extractOutputs emits a SCOPE.Schema/output per `output` declaration.
func extractOutputs(src, path string) []types.EntityRecord {
	var out []types.EntityRecord
	for _, loc := range reOutput.FindAllStringSubmatchIndex(src, -1) {
		name := src[loc[2]:loc[3]]
		outType := src[loc[4]:loc[5]]
		out = append(out, types.EntityRecord{
			Name:         "output." + name,
			Kind:         "SCOPE.Schema",
			Subtype:      "output",
			SourceFile:   path,
			StartLine:    lineOf(src, loc[0]),
			EndLine:      lineOf(src, loc[0]),
			Language:     lang,
			QualityScore: 0.8,
			Metadata: map[string]interface{}{
				"subtype":     "output",
				"iac_tool":    "bicep",
				"label":       name,
				"output_type": outType,
			},
		})
	}
	return out
}

// dependencyEdges returns DEPENDS_ON edges for a resource/module body: one per
// distinct symbolic name referenced (either through a dotted property access
// like `foo.id` / `foo.outputs.x`, or listed in an explicit dependsOn array).
// Self-references are skipped. Only names in `symbolic` (locally-declared
// resources/modules) produce edges.
func dependencyEdges(body, path, self string, symbolic map[string]bool) []types.RelationshipRecord {
	deps := referencedSymbols(body, self, symbolic)
	if len(deps) == 0 {
		return nil
	}
	var rels []types.RelationshipRecord
	for _, dep := range deps {
		rels = append(rels, types.RelationshipRecord{
			FromID: path,
			ToID:   extractor.BuildOperationStructuralRef(lang, path, dep),
			Kind:   "DEPENDS_ON",
		})
	}
	return rels
}

// reSymbolRef matches a symbolic-name property access: `<name>.id`,
// `<name>.properties.x`, `<name>.outputs.y`. The leading boundary excludes
// dotted continuations (so `a.b.c` does not also match `b.c`).
var reSymbolRef = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\.(?:id|name|properties|outputs)\b`)

// referencedSymbols returns the sorted-unique set of locally-declared symbolic
// names referenced in body (excluding self), via property access or dependsOn.
func referencedSymbols(body, self string, symbolic map[string]bool) []string {
	seen := map[string]bool{}
	var order []string
	add := func(name string) {
		if name == self || !symbolic[name] || seen[name] {
			return
		}
		seen[name] = true
		order = append(order, name)
	}

	for _, m := range reSymbolRef.FindAllStringSubmatch(body, -1) {
		// Skip a property access whose token is preceded by a '.' (i.e. it is a
		// nested segment, not a root symbol). FindAllStringSubmatch loses that
		// context, so re-check via index below instead.
		add(m[1])
	}
	// Re-filter nested segments: only keep names that appear as a root token
	// (not immediately preceded by '.').
	order = filterRootTokens(body, order)

	// Explicit dependsOn: [ a, b ] — names are bare symbolic identifiers,
	// optionally with array-index / property suffixes.
	for _, dm := range reDependsOn.FindAllStringSubmatch(body, -1) {
		for _, tok := range splitDependsOn(dm[1]) {
			add(tok)
		}
	}
	return order
}

// filterRootTokens drops names from order that never occur in body as a root
// token (a token not immediately preceded by '.'), guarding against matching
// the tail of a longer dotted path such as parent.child.id.
func filterRootTokens(body string, order []string) []string {
	var out []string
	for _, name := range order {
		if occursAsRoot(body, name) {
			out = append(out, name)
		}
	}
	return out
}

// occursAsRoot reports whether name appears in body as a root identifier: a
// match not immediately preceded by '.' or an identifier character.
func occursAsRoot(body, name string) bool {
	idx := 0
	for {
		j := strings.Index(body[idx:], name)
		if j < 0 {
			return false
		}
		pos := idx + j
		if pos == 0 || !isIdentByte(body[pos-1]) && body[pos-1] != '.' {
			// Ensure the char after is a '.' (property access) to count.
			after := pos + len(name)
			if after < len(body) && body[after] == '.' {
				return true
			}
		}
		idx = pos + len(name)
		if idx >= len(body) {
			return false
		}
	}
}

func isIdentByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// splitDependsOn splits a dependsOn array body into bare symbolic-name tokens,
// stripping array-index / property suffixes (e.g. `vnets[0]` → `vnets`,
// `sa.id` → `sa`).
func splitDependsOn(inner string) []string {
	fields := strings.FieldsFunc(inner, func(r rune) bool {
		return r == ',' || r == '\n' || r == ' ' || r == '\t' || r == '\r'
	})
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		// Strip property/index suffix: keep the leading identifier.
		for i := 0; i < len(f); i++ {
			if !isIdentByte(f[i]) {
				f = f[:i]
				break
			}
		}
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// splitAzureType splits 'Microsoft.Storage/storageAccounts@2022-09-01' into the
// resource-provider type ("Microsoft.Storage/storageAccounts") and api version.
func splitAzureType(full string) (rpType, apiVersion string) {
	if at := strings.IndexByte(full, '@'); at >= 0 {
		return full[:at], full[at+1:]
	}
	return full, ""
}

// isLoop reports whether a body opens with a `[for … in … :` comprehension.
func isLoop(body string) bool {
	trimmed := strings.TrimSpace(body)
	if strings.HasPrefix(trimmed, "[for ") || strings.HasPrefix(trimmed, "[for\t") {
		return true
	}
	return strings.Contains(trimmed, "[for ") && strings.Contains(trimmed, " in ")
}

// nextDeclBoundary returns the byte offset (within [start,max)) of the next
// top-level resource/module/param/var/output/@-decorator declaration after a
// declaration body begins, so a body never bleeds into the following entity.
func nextDeclBoundary(src string, start, max int) int {
	window := src[start:max]
	earliest := len(window)
	for _, re := range []*regexp.Regexp{reResource, reModule, reParam, reVar, reOutput} {
		if loc := re.FindStringIndex(window); loc != nil && loc[0] < earliest {
			earliest = loc[0]
		}
	}
	return start + earliest
}

// lineOf returns the 1-indexed line number of byte offset off in src.
func lineOf(src string, off int) int {
	if off > len(src) {
		off = len(src)
	}
	return strings.Count(src[:off], "\n") + 1
}
