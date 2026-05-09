// Package proto implements the tree-sitter–based extractor for Protocol Buffer source files.
//
// Extracted entities:
//   - service  → Kind="SCOPE.Service",   Subtype="service"
//   - rpc      → Kind="SCOPE.Operation", Subtype="endpoint" (Properties["type"]="rpc")
//   - message  → Kind="SCOPE.Schema",    Subtype="message"
//   - enum     → Kind="SCOPE.Schema",    Subtype="enum"
//
// Issue #377 — relationship parity:
//
//   - IMPORTS edges are emitted from file.Path → import target for every
//     `import "x.proto";` and `import public "x.proto";` directive.
//     `import public` carries Properties["public"]="true".
//   - CONTAINS edges:
//   - file → service / message / enum (top-level definitions),
//   - service → rpc,
//   - message → field,
//   - enum → enum value.
//     ToIDs use BuildOperationStructuralRef("proto", file, name) for entity
//     children (service/message/enum/rpc) and the table#column-style ref
//     `scope:schema:column:proto:<file>:<parent>#<member>` for fields and
//     enum values, mirroring SQL Format B.
//
// Uses the protobuf grammar from smacker/go-tree-sitter.
// Registers itself via init() and is imported by registry_gen.go.
package proto

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("proto", &Extractor{})
}

// Extractor implements extractor.Extractor for Protocol Buffers.
type Extractor struct{}

// Language returns the canonical language name.
func (e *Extractor) Language() string { return "proto" }

// Extract walks the tree-sitter CST and returns entity records.
func (e *Extractor) Extract(_ context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	if file.Tree == nil || len(file.Content) == 0 {
		return nil, nil
	}

	var entities []types.EntityRecord
	walkProto(file.Tree.RootNode(), file, &entities)

	// Append IMPORTS stub entities, one per `import "..."` directive.
	importEntities := buildImportEntities(file)
	if len(importEntities) > 0 {
		entities = append(entities, importEntities...)
	}

	return entities, nil
}

func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	return string(src[node.StartByte():node.EndByte()])
}

func childByType(node *sitter.Node, types_ ...string) *sitter.Node {
	set := make(map[string]bool, len(types_))
	for _, t := range types_ {
		set[t] = true
	}
	for i := range node.ChildCount() {
		ch := node.Child(int(i))
		if ch != nil && set[ch.Type()] {
			return ch
		}
	}
	return nil
}

// fieldMemberRef returns the Format B structural-ref for a parent#member edge
// inside a proto file (message field, enum value). Mirrors
// BuildSchemaColumnStructuralRef but tagged with the "proto" language.
func fieldMemberRef(filePath, parent, member string) string {
	return "scope:schema:column:proto:" + filePath + ":" + parent + "#" + member
}

// fileContainsRel builds a CONTAINS edge from file.Path → top-level entity.
func fileContainsRel(filePath, name string) types.RelationshipRecord {
	return types.RelationshipRecord{
		FromID: filePath,
		ToID:   extractor.BuildOperationStructuralRef("proto", filePath, name),
		Kind:   "CONTAINS",
	}
}

func walkProto(node *sitter.Node, file extractor.FileInput, out *[]types.EntityRecord) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "service":
		if rec, ok := buildService(node, file); ok {
			*out = append(*out, rec)
		}
		// Walk inside service for rpc nodes.
		for i := range node.ChildCount() {
			ch := node.Child(int(i))
			if ch != nil && ch.Type() == "rpc" {
				if rec, ok := buildRPC(ch, file); ok {
					*out = append(*out, rec)
				}
			}
		}
		return // Don't recurse further into service — already handled.
	case "message":
		if rec, ok := buildMessage(node, file); ok {
			*out = append(*out, rec)
		}
	case "enum":
		if rec, ok := buildEnum(node, file); ok {
			*out = append(*out, rec)
		}
	}

	for i := range node.ChildCount() {
		walkProto(node.Child(int(i)), file, out)
	}
}

func buildService(node *sitter.Node, file extractor.FileInput) (types.EntityRecord, bool) {
	nameNode := childByType(node, "service_name")
	if nameNode == nil {
		return types.EntityRecord{}, false
	}
	name := strings.TrimSpace(nodeText(nameNode, file.Content))
	// service_name wraps identifier
	if ident := childByType(nameNode, "identifier"); ident != nil {
		name = nodeText(ident, file.Content)
	}
	if name == "" {
		return types.EntityRecord{}, false
	}

	// CONTAINS edges: service → each rpc child + file → service.
	var rels []types.RelationshipRecord
	rels = append(rels, fileContainsRel(file.Path, name))
	for i := range node.ChildCount() {
		ch := node.Child(int(i))
		if ch == nil || ch.Type() != "rpc" {
			continue
		}
		rpcNameNode := childByType(ch, "rpc_name")
		if rpcNameNode == nil {
			continue
		}
		rpcName := strings.TrimSpace(nodeText(rpcNameNode, file.Content))
		if ident := childByType(rpcNameNode, "identifier"); ident != nil {
			rpcName = nodeText(ident, file.Content)
		}
		if rpcName == "" {
			continue
		}
		rels = append(rels, types.RelationshipRecord{
			ToID: extractor.BuildOperationStructuralRef("proto", file.Path, rpcName),
			Kind: "CONTAINS",
		})
	}

	return types.EntityRecord{
		Name:               name,
		Kind:               "SCOPE.Service",
		Subtype:            "service",
		SourceFile:         file.Path,
		Language:           "protobuf",
		StartLine:          int(node.StartPoint().Row) + 1,
		EndLine:            int(node.EndPoint().Row) + 1,
		Signature:          "service " + name,
		EnrichmentRequired: false,
		Relationships:      rels,
	}, true
}

func buildRPC(node *sitter.Node, file extractor.FileInput) (types.EntityRecord, bool) {
	// rpc: rpc <rpc_name> ( <msg_type> ) returns ( <msg_type> ) ;
	nameNode := childByType(node, "rpc_name")
	if nameNode == nil {
		return types.EntityRecord{}, false
	}
	name := strings.TrimSpace(nodeText(nameNode, file.Content))
	if ident := childByType(nameNode, "identifier"); ident != nil {
		name = nodeText(ident, file.Content)
	}
	if name == "" {
		return types.EntityRecord{}, false
	}

	// Collect request and response types.
	var msgTypes []string
	for i := range node.ChildCount() {
		ch := node.Child(int(i))
		if ch != nil && ch.Type() == "message_or_enum_type" {
			msgTypes = append(msgTypes, nodeText(ch, file.Content))
		}
	}
	reqType, respType := "?", "?"
	if len(msgTypes) >= 1 {
		reqType = msgTypes[0]
	}
	if len(msgTypes) >= 2 {
		respType = msgTypes[1]
	}

	sig := fmt.Sprintf("rpc %s(%s) returns (%s)", name, reqType, respType)
	return types.EntityRecord{
		Name:               name,
		Kind:               "SCOPE.Operation",
		Subtype:            "endpoint",
		SourceFile:         file.Path,
		Language:           "protobuf",
		StartLine:          int(node.StartPoint().Row) + 1,
		EndLine:            int(node.EndPoint().Row) + 1,
		Signature:          sig,
		EnrichmentRequired: false,
		// Set type=rpc explicitly so buildOutputDoc doesn't override with "endpoint".
		// Python golden uses type=rpc, subtype=endpoint for RPC entities.
		Properties: map[string]string{"type": "rpc"},
		// rpc carries the request/response IMPORTS edges historically emitted
		// here. File-level `import "..."` edges are emitted separately as stub
		// entities by buildImportEntities.
		Relationships: []types.RelationshipRecord{
			{FromID: file.Path, ToID: reqType, Kind: "IMPORTS"},
			{FromID: file.Path, ToID: respType, Kind: "IMPORTS"},
		},
	}, true
}

func buildMessage(node *sitter.Node, file extractor.FileInput) (types.EntityRecord, bool) {
	nameNode := childByType(node, "message_name")
	if nameNode == nil {
		return types.EntityRecord{}, false
	}
	name := strings.TrimSpace(nodeText(nameNode, file.Content))
	if ident := childByType(nameNode, "identifier"); ident != nil {
		name = nodeText(ident, file.Content)
	}
	if name == "" {
		return types.EntityRecord{}, false
	}

	// CONTAINS edges: file → message + message → each field.
	var rels []types.RelationshipRecord
	rels = append(rels, fileContainsRel(file.Path, name))
	if body := childByType(node, "message_body"); body != nil {
		seen := make(map[string]bool)
		for i := range body.ChildCount() {
			ch := body.Child(int(i))
			if ch == nil || ch.Type() != "field" {
				continue
			}
			fname := fieldName(ch, file.Content)
			if fname == "" || seen[fname] {
				continue
			}
			seen[fname] = true
			rels = append(rels, types.RelationshipRecord{
				ToID: fieldMemberRef(file.Path, name, fname),
				Kind: "CONTAINS",
			})
		}
	}

	return types.EntityRecord{
		Name:               name,
		Kind:               "SCOPE.Schema",
		Subtype:            "message",
		SourceFile:         file.Path,
		Language:           "protobuf",
		StartLine:          int(node.StartPoint().Row) + 1,
		EndLine:            int(node.EndPoint().Row) + 1,
		Signature:          "message " + name,
		EnrichmentRequired: false,
		Relationships:      rels,
	}, true
}

func buildEnum(node *sitter.Node, file extractor.FileInput) (types.EntityRecord, bool) {
	nameNode := childByType(node, "enum_name")
	if nameNode == nil {
		return types.EntityRecord{}, false
	}
	name := strings.TrimSpace(nodeText(nameNode, file.Content))
	if ident := childByType(nameNode, "identifier"); ident != nil {
		name = nodeText(ident, file.Content)
	}
	if name == "" {
		return types.EntityRecord{}, false
	}

	// CONTAINS edges: file → enum + enum → each enum value.
	var rels []types.RelationshipRecord
	rels = append(rels, fileContainsRel(file.Path, name))
	if body := childByType(node, "enum_body"); body != nil {
		seen := make(map[string]bool)
		for i := range body.ChildCount() {
			ch := body.Child(int(i))
			if ch == nil || ch.Type() != "enum_field" {
				continue
			}
			vname := enumValueName(ch, file.Content)
			if vname == "" || seen[vname] {
				continue
			}
			seen[vname] = true
			rels = append(rels, types.RelationshipRecord{
				ToID: fieldMemberRef(file.Path, name, vname),
				Kind: "CONTAINS",
			})
		}
	}

	return types.EntityRecord{
		Name:               name,
		Kind:               "SCOPE.Schema",
		Subtype:            "enum",
		SourceFile:         file.Path,
		Language:           "protobuf",
		StartLine:          int(node.StartPoint().Row) + 1,
		EndLine:            int(node.EndPoint().Row) + 1,
		Signature:          "enum " + name,
		EnrichmentRequired: false,
		Relationships:      rels,
	}, true
}

// fieldName returns the message-field's identifier (the second `identifier`
// child after the `type` node — the first `identifier` under `type` is the
// type name, not the field name). The grammar lays a `field` node out as:
//
//	field
//	  type
//	    (string|identifier|message_or_enum_type ...)
//	  identifier   ← field name
//	  =
//	  field_number
func fieldName(node *sitter.Node, src []byte) string {
	for i := range node.ChildCount() {
		ch := node.Child(int(i))
		if ch == nil {
			continue
		}
		if ch.Type() == "identifier" {
			return nodeText(ch, src)
		}
	}
	return ""
}

// enumValueName returns the first identifier child of an `enum_field` node.
//
//	enum_field
//	  identifier   ← value name (e.g. UNKNOWN)
//	  =
//	  int_lit
func enumValueName(node *sitter.Node, src []byte) string {
	if id := childByType(node, "identifier"); id != nil {
		return nodeText(id, src)
	}
	return ""
}

// buildImportEntities scans top-level `import "..."` and `import public "..."`
// directives and returns one stub SCOPE.Component entity per import target,
// each carrying an IMPORTS edge from file.Path → target. `import public`
// imports carry Properties["public"]="true" on the relationship.
func buildImportEntities(file extractor.FileInput) []types.EntityRecord {
	root := file.Tree.RootNode()
	var entities []types.EntityRecord
	for i := range root.ChildCount() {
		ch := root.Child(int(i))
		if ch == nil || ch.Type() != "import" {
			continue
		}
		path, public := parseImport(ch, file.Content)
		if path == "" {
			continue
		}
		rel := types.RelationshipRecord{
			FromID: file.Path,
			ToID:   path,
			Kind:   "IMPORTS",
		}
		if public {
			rel.Properties = map[string]string{"public": "true"}
		}
		entities = append(entities, types.EntityRecord{
			Name:               path,
			Kind:               "SCOPE.Component",
			Subtype:            "import",
			SourceFile:         file.Path,
			Language:           "protobuf",
			StartLine:          int(ch.StartPoint().Row) + 1,
			EndLine:            int(ch.EndPoint().Row) + 1,
			Signature:          nodeText(ch, file.Content),
			EnrichmentRequired: false,
			Relationships:      []types.RelationshipRecord{rel},
		})
	}
	return entities
}

// parseImport extracts the quoted path and the `public` modifier from an
// `import` node. The grammar shape is:
//
//	import
//	  import          (keyword)
//	  [public|weak]   (optional modifier)
//	  string          ("path/to.proto")
//	  ;
func parseImport(node *sitter.Node, src []byte) (path string, public bool) {
	for i := range node.ChildCount() {
		ch := node.Child(int(i))
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "public":
			public = true
		case "string":
			raw := nodeText(ch, src)
			path = strings.Trim(raw, "\"'")
		}
	}
	return path, public
}
