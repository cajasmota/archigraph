// types.go — Dart Type System extraction (#4912).
//
// The base dart.go walk recognises only `class` / `abstract class` /
// `mixin` / `extension` declarations. The single highest-value LANGUAGE-CORE
// gap called out in #4912 is the Dart *type system*: all three Dart framework
// records (flutter, graphql-flutter, shelf) mark enum_extraction /
// type_alias_extraction / interface_extraction as `missing`, and dart.go
// actively pushes `enum` into skipKeywords so enums are dropped entirely.
//
// This pass adds the three missing constructs, fixture-proven:
//
//   - enum (plain + Dart 2.17 "enhanced" enums with members/methods) →
//     SCOPE.Enum value-set node via the shared extractor.EnumEntity helper
//     (kind_hint="dart_enum"), so the graph answers "what values can this
//     field take?" for cross-graph enum parity (parity with python/ts/java
//     enum value-sets, #3628/#4420). Enhanced-enum bodies (`enum X { a, b;
//     final int n; ... }`) keep only the constant identifiers before the
//     first `;` as members — method/field noise after the `;` is dropped.
//
//   - typedef → SCOPE.Schema(subtype=type_alias), matching the python
//     (internal/extractors/python/types.go) and rust/go type_alias shape.
//     Both spellings are handled: the modern `typedef Name = <type>;` and
//     the legacy function-type `typedef ReturnT Name(params);`.
//
//   - Dart 3 class modifiers (sealed / base / interface / final / mixin
//     class) → the same SCOPE.Component(subtype=class) the base walk emits,
//     but carrying a `class_modifier` property and a `dart_sealed` /
//     `dart_interface` etc. hint so the resolver / dashboard can distinguish
//     a `sealed class` (exhaustive-switch root) from a plain class. The base
//     classRE does not match these (its leading group only allows an
//     optional `abstract`), so they were invisible.
//
// Regex-based to match dart.go (no tree-sitter Dart grammar is bundled in
// smacker/go-tree-sitter). Runs as a post-pass appended to the base entity
// slice — it never mutates the base class/method records, it only adds the
// previously-dropped type entities. The base extractor's skipKeywords still
// excludes enum/typedef/sealed from the method/call passes, so there is no
// double-emit.
package dart

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

var (
	// `enum Name {` head — plain or enhanced. The optional leading `@anno`
	// / whitespace is tolerated; the body (up to the matching `}`) is parsed
	// for members separately.
	dartEnumRE = regexp.MustCompile(`(?m)^[ \t]*enum\s+(\w+)\s*(?:with\s+[\w,\s<>]+)?(?:implements\s+[\w,\s<>]+)?\{`)

	// Modern `typedef Name = <type>;` (Dart 2.13+). Submatch 1 = name,
	// 2 = the aliased type body (up to the `;`).
	dartTypedefAliasRE = regexp.MustCompile(`(?m)^[ \t]*typedef\s+(\w+)(?:<[^>]*>)?\s*=\s*([^;]+);`)

	// Legacy function-type `typedef ReturnType Name(params);` — no `=`.
	// Submatch 1 = name (the identifier immediately before the `(`).
	dartTypedefFuncRE = regexp.MustCompile(`(?m)^[ \t]*typedef\s+(?:[\w<>\[\]?,\s]+?\s+)?(\w+)\s*\([^;{]*\)\s*;`)

	// Dart 3 class modifiers: `sealed|base|interface|final|mixin` (or pairs
	// like `abstract interface`, `base mixin`) preceding `class Name`.
	// Submatch 1 = modifier run, 2 = class name.
	dartModifiedClassRE = regexp.MustCompile(
		`(?m)^[ \t]*((?:abstract\s+|sealed\s+|base\s+|interface\s+|final\s+|mixin\s+)+)class\s+(\w+)`)
)

// extractDartTypes parses Dart type-system constructs (enum / typedef /
// modified class) the base walk drops, returning the extra entity records.
func extractDartTypes(src, filePath string) []types.EntityRecord {
	var out []types.EntityRecord
	out = append(out, dartEnums(src, filePath)...)
	out = append(out, dartTypedefs(src, filePath)...)
	out = append(out, dartModifiedClasses(src, filePath)...)
	return out
}

// dartEnums emits one SCOPE.Enum value-set per `enum` declaration.
func dartEnums(src, filePath string) []types.EntityRecord {
	var out []types.EntityRecord
	for _, m := range dartEnumRE.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		braceOpen := m[1] - 1
		braceEnd := findBraceEndByte(src, braceOpen)
		startLine := strings.Count(src[:m[0]], "\n") + 1
		endLine := strings.Count(src[:braceEnd], "\n") + 1

		body := ""
		if braceOpen+1 <= braceEnd {
			body = src[braceOpen+1 : braceEnd]
		}
		members := dartEnumMembers(body)
		if e, ok := extractor.EnumEntity(name, "dart", "dart_enum", filePath,
			startLine, endLine, members); ok {
			out = append(out, e)
		}
	}
	return out
}

// dartEnumMembers parses the constant identifiers of an enum body. For an
// enhanced enum the constants come before the first top-level `;`; everything
// after it (fields/methods/const ctor) is dropped.
func dartEnumMembers(body string) []extractor.EnumMember {
	// Enhanced-enum: keep only up to the first `;`.
	if semi := strings.IndexByte(body, ';'); semi >= 0 {
		body = body[:semi]
	}
	var members []extractor.EnumMember
	for _, raw := range strings.Split(body, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		// A member may carry a constructor call: `apple(1)` / `red('#f00')`.
		// Keep the bare identifier as the member name; capture the literal
		// arg as the value when it is a single literal.
		name := tok
		value := ""
		if p := strings.IndexByte(tok, '('); p >= 0 {
			name = strings.TrimSpace(tok[:p])
			arg := strings.TrimSuffix(strings.TrimSpace(tok[p+1:]), ")")
			arg = strings.TrimSpace(strings.TrimSuffix(arg, ")"))
			if arg != "" && !strings.ContainsAny(arg, ",") {
				value = extractor.StripLiteralQuotes(arg)
			}
		}
		// Reject non-identifier leading tokens (annotations, comments).
		if name == "" || !isDartIdent(name) {
			continue
		}
		members = append(members, extractor.EnumMember{Name: name, Value: value})
	}
	return members
}

// dartTypedefs emits SCOPE.Schema(type_alias) per typedef directive.
func dartTypedefs(src, filePath string) []types.EntityRecord {
	var out []types.EntityRecord
	seen := map[string]bool{}
	for _, m := range dartTypedefAliasRE.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		bodyTxt := strings.TrimSpace(src[m[4]:m[5]])
		line := strings.Count(src[:m[0]], "\n") + 1
		out = append(out, dartTypeAlias(name, bodyTxt, filePath, line))
		seen[name] = true
	}
	for _, m := range dartTypedefFuncRE.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		if seen[name] {
			continue
		}
		line := strings.Count(src[:m[0]], "\n") + 1
		out = append(out, dartTypeAlias(name, "", filePath, line))
	}
	return out
}

func dartTypeAlias(name, body, filePath string, line int) types.EntityRecord {
	props := map[string]string{"line": strconv.Itoa(line)}
	if body != "" && len(body) <= 512 {
		props["type_body"] = body
	}
	return types.EntityRecord{
		Name:               name,
		Kind:               "SCOPE.Schema",
		Subtype:            "type_alias",
		Language:           "dart",
		SourceFile:         filePath,
		StartLine:          line,
		EndLine:            line,
		Signature:          "typedef " + name,
		Properties:         props,
		EnrichmentRequired: false,
	}
}

// dartModifiedClasses emits the SCOPE.Component(class) records the base
// classRE drops because of a leading Dart 3 modifier (sealed/base/interface/
// final/mixin class). The base walk already covers `class` / `abstract class`.
func dartModifiedClasses(src, filePath string) []types.EntityRecord {
	var out []types.EntityRecord
	for _, m := range dartModifiedClassRE.FindAllStringSubmatchIndex(src, -1) {
		mods := strings.Fields(strings.TrimSpace(src[m[2]:m[3]]))
		// `abstract class` alone is already handled by the base classRE — skip
		// when the only modifier is abstract (no Dart 3 modifier present).
		if len(mods) == 1 && mods[0] == "abstract" {
			continue
		}
		name := src[m[4]:m[5]]
		startLine := strings.Count(src[:m[0]], "\n") + 1
		// Find the class body brace for end line (best-effort).
		endLine := startLine
		if bracePos := strings.IndexByte(src[m[0]:], '{'); bracePos >= 0 {
			endByte := findBraceEndByte(src, m[0]+bracePos)
			endLine = strings.Count(src[:endByte], "\n") + 1
		}
		modifier := strings.Join(mods, " ")
		props := map[string]string{
			"class_modifier": modifier,
		}
		if hasMod(mods, "sealed") {
			props["dart_sealed"] = "true"
		}
		if hasMod(mods, "interface") {
			props["dart_interface"] = "true"
		}
		out = append(out, types.EntityRecord{
			Name:               name,
			Kind:               "SCOPE.Component",
			Subtype:            "class",
			SourceFile:         filePath,
			Language:           "dart",
			StartLine:          startLine,
			EndLine:            endLine,
			Signature:          modifier + " class " + name,
			EnrichmentRequired: false,
			Properties:         props,
		})
	}
	return out
}

func hasMod(mods []string, want string) bool {
	for _, m := range mods {
		if m == want {
			return true
		}
	}
	return false
}

func isDartIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		ok := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(i > 0 && r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}
	return true
}
