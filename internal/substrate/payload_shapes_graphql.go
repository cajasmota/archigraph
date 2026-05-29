// GraphQL SDL payload-shape sniffer (#3076 B-part).
//
// Sniffs GraphQL Schema Definition Language (SDL) files (.graphql / .gql) for
// resolver argument shapes (input types and inline args) and return-type shapes
// (object types referenced as Query/Mutation return types).
//
// Shape model:
//
//   - `input CreateUserInput { name: String!, age: Int }` → every field in the
//     input type becomes a PayloadField on a ProducerRequest shape attributed to
//     the input type name. The GraphQL resolver that accepts this input receives
//     it as its `args` parameter — semantically equivalent to an HTTP request
//     body read.
//
//   - `type User { id: ID!, name: String!, email: String }` → every field in
//     an object type becomes a PayloadField on a ProducerResponse shape
//     attributed to the type name. Query/Mutation operations that return `User`
//     (or `[User]` / `User!`) emit this shape as the resolver's response contract.
//
//   - `type Query { createUser(input: CreateUserInput!): User }` → an inline
//     argument list on an operation definition is sniffed to extract argument
//     names directly (not recursively resolved through the input type — single
//     level only, Phase 3 dependency). The return type provides the response
//     shape attribution.
//
// Direction / side assignment:
//
//   - input type fields  → ProducerRequest  (resolver reads them off args)
//   - object type fields → ProducerResponse (resolver writes them as the return)
//   - inline query args  → ProducerRequest  (attributed to the operation name)
//
// Confidence: SDL definitions are static types, so confidence is always 1.0.
// Cross-file type resolution is out of scope for this pass (Phase 4 work).
package substrate

import (
	"regexp"
	"strings"
)

func init() { RegisterPayloadShapeSniffer("graphql", sniffPayloadShapesGraphQL) }

// sdlInputBlockRe matches `input TypeName {` capturing the type name.
var sdlInputBlockRe = regexp.MustCompile(`(?m)^input\s+(\w+)\s*\{`)

// sdlTypeBlockRe matches `type TypeName {` or `type TypeName implements Foo {`,
// capturing the type name.  Excludes the built-in scalars (String, Int, etc.)
// which are single-token and never declared as block types in user SDL.
var sdlTypeBlockRe = regexp.MustCompile(`(?m)^type\s+(\w+)(?:\s+implements\s+\w+(?:\s*&\s*\w+)*)?\s*\{`)

// sdlFieldLineRe matches a single field declaration inside a block:
//
//	fieldName(args…): ReturnType
//	fieldName: TypeName
//
// Capture group 1 = field name.
var sdlFieldLineRe = regexp.MustCompile(`^\s{1,4}(\w+)\s*(?:\([^)]*\))?\s*:`)

// sdlOperationArgRe extracts individual argument names from an inline
// argument list `(argName: Type, ...)`. Capture group 1 = arg name.
var sdlOperationArgRe = regexp.MustCompile(`\b(\w+)\s*:\s*\[?\w+`)

// sniffPayloadShapesGraphQL parses a GraphQL SDL file and emits PayloadShape
// records for input types (request shapes) and object types (response shapes).
func sniffPayloadShapesGraphQL(content string) []PayloadShape {
	if content == "" {
		return nil
	}
	var out []PayloadShape

	// Collect named block ranges so we can extract field lines per block.
	blocks := collectSDLBlocks(content)

	for _, b := range blocks {
		// Slice the block body (between '{' and matching '}').
		openBrace := strings.Index(content[b.start:], "{")
		if openBrace < 0 {
			continue
		}
		abs := b.start + openBrace + 1
		body, _ := sdlBlockBody(content, abs)
		if body == "" {
			continue
		}

		fields := sdlExtractFields(body, b.kind == "type" && isOperationRoot(b.name))

		if len(fields) == 0 {
			continue
		}

		var dir PayloadDirection
		var side PayloadSide
		switch b.kind {
		case "input":
			dir = PayloadDirectionRequest
			side = PayloadSideProducer
		default: // "type" — object type used as return type
			dir = PayloadDirectionResponse
			side = PayloadSideProducer
		}

		// Skip Query / Mutation / Subscription root type — their children are
		// operations, not plain fields.  Those are sniffed separately as inline
		// arg shapes below.
		if isOperationRoot(b.name) {
			out = append(out, sdlOperationArgShapes(b.name, body)...)
			continue
		}

		out = append(out, PayloadShape{
			Function:   b.name,
			Direction:  dir,
			Side:       side,
			Fields:     DedupFields(fields),
			Confidence: 1.0,
		})
	}

	return out
}

// isOperationRoot reports whether name is one of the three GraphQL root
// operation types.
func isOperationRoot(name string) bool {
	switch name {
	case "Query", "Mutation", "Subscription":
		return true
	}
	return false
}

// sdlOperationArgShapes emits one ProducerRequest shape per operation
// declared inside a Query/Mutation/Subscription root block. Each operation
// contributes the inline argument names as request fields (these are the
// variables the resolver receives in its `args` parameter).
//
// Lines have the shape: `  opName(argName: ArgType, ...): ReturnType`
// We locate the paren pair first, then extract the operation name from
// the text before `(` and the argument names from the text between `(` and `)`.
func sdlOperationArgShapes(rootName, body string) []PayloadShape {
	var out []PayloadShape
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parenOpen := strings.IndexByte(line, '(')
		parenClose := strings.IndexByte(line, ')')

		var opName string
		var argBody string

		if parenOpen > 0 && parenClose > parenOpen {
			opName = strings.TrimSpace(line[:parenOpen])
			argBody = line[parenOpen+1 : parenClose]
		} else {
			// No parens — no inline args; skip (no request shape to emit).
			continue
		}

		if opName == "" {
			continue
		}

		var argFields []PayloadField
		if argBody != "" {
			for _, m := range sdlOperationArgRe.FindAllStringSubmatch(argBody, -1) {
				if len(m) > 1 && m[1] != "" {
					argFields = append(argFields, PayloadField{Name: m[1]})
				}
			}
		}
		if len(argFields) == 0 {
			continue
		}

		out = append(out, PayloadShape{
			Function:   rootName + "." + opName,
			Direction:  PayloadDirectionRequest,
			Side:       PayloadSideProducer,
			Fields:     DedupFields(argFields),
			Confidence: 1.0,
		})
	}
	return out
}

// sdlBlock is a named block entry extracted from SDL content.
type sdlBlock struct {
	name  string
	start int
	kind  string // "input" | "type"
}

// collectSDLBlocks returns the named blocks in the content, including both
// input types and object types.
func collectSDLBlocks(content string) []sdlBlock {
	var blocks []sdlBlock

	for _, m := range sdlInputBlockRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		name := content[m[2]:m[3]]
		blocks = append(blocks, sdlBlock{name: name, start: m[0], kind: "input"})
	}
	for _, m := range sdlTypeBlockRe.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 {
			continue
		}
		name := content[m[2]:m[3]]
		blocks = append(blocks, sdlBlock{name: name, start: m[0], kind: "type"})
	}
	return blocks
}

// sdlBlockBody returns the text between the opening `{` (at abs) and the
// matching closing `}`, using a simple brace-counter. The returned string
// does NOT include the surrounding braces.
func sdlBlockBody(content string, abs int) (string, int) {
	depth := 1
	for i := abs; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[abs:i], i
			}
		}
	}
	return content[abs:], len(content)
}

// sdlExtractFields returns field names found in an SDL block body.
// When isRoot is true (for Query/Mutation/Subscription), the lines have the
// shape `opName(args): ReturnType` and we still extract the field name only
// (the root case is handled separately by sdlOperationArgShapes so the caller
// should not call both — this path is unreachable for root types from the
// main loop).
func sdlExtractFields(body string, _ bool) []PayloadField {
	var fields []PayloadField
	for _, line := range strings.Split(body, "\n") {
		m := sdlFieldLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		if name == "" || sdlBuiltinScalar(name) {
			continue
		}
		fields = append(fields, PayloadField{Name: name})
	}
	return fields
}

// sdlBuiltinScalar reports whether name is a GraphQL built-in directive or
// keyword that should not be treated as a field name.
func sdlBuiltinScalar(name string) bool {
	switch name {
	case "implements", "on", "schema", "scalar", "type", "input",
		"enum", "union", "interface", "directive", "extend",
		"fragment", "query", "mutation", "subscription":
		return true
	}
	return false
}
