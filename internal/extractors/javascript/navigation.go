// Package javascript — navigation edge extraction for issue #2655.
//
// This file detects navigation call sites across Expo Router, React Navigation,
// and Next.js patterns, emitting NAVIGATES_TO edges from the caller entity to a
// route stub (or to the matched destination entity if resolvable).
//
// Detected patterns (Phase 1):
//   - router.push('/path')                     → NAVIGATES_TO, route='/path'
//   - router.navigate('/path')                 → NAVIGATES_TO, route='/path'
//   - router.replace('/path')                  → NAVIGATES_TO, route='/path'
//   - router.back()                            → NAVIGATES_TO, route='<back>'
//   - router.push({pathname: '/x', params: {a, b}}) → route='/x', params=[a,b]
//   - navigation.navigate('Screen')            → NAVIGATES_TO, route='Screen'
//   - navigation.push('Screen')                → NAVIGATES_TO, route='Screen'
//   - Linking.openURL('https://...')           → NAVIGATES_TO (external)
//
// Phase 2 additions (#2658):
//   - Template-literal routes: router.push(`/users/${id}`) → route='/users/{*}'
//   - Hook-rename binding: const nav = useNavigation(); nav.navigate('X')
//     detected via hookVarToNavModule table scanned alongside hookVarToModule.
package javascript

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/cajasmota/archigraph/internal/types"
)

// templateExprRe matches JavaScript template-literal expression slots
// (${...}) and is used to normalise dynamic route strings to a stable
// pattern compatible with server-side route definitions (e.g. /users/{*}).
// Phase 2 of #2658 — mirrors the {*} sentinel used by normalizePathForIndex
// in internal/links/http_pass.go.
var templateExprRe = regexp.MustCompile(`\$\{[^}]*\}`)

// normalizeTemplateLiteralRoute converts a raw template-literal string
// (including surrounding backticks and any ${…} interpolations) into a
// stable route pattern by:
//  1. Stripping the surrounding backtick delimiters.
//  2. Replacing every ${…} slot with the {*} sentinel so the result can
//     be matched against server-side route definitions like /users/{id}.
//
// Example:  "`/users/${id}/profile`"  →  "/users/{*}/profile"
func normalizeTemplateLiteralRoute(raw string) string {
	// Strip surrounding backticks added by nodeText for template_string nodes.
	s := strings.Trim(raw, "`")
	// Replace every ${…} interpolation with the {*} placeholder.
	s = templateExprRe.ReplaceAllString(s, "{*}")
	return s
}

// navigationHookNames is the set of hook function names whose return value
// should be treated as a navigation object. When a variable is bound to one
// of these hooks (e.g. `const nav = useNavigation()`), any .navigate/.push
// call on that variable is recognised as a navigation call.
//
// Phase 2 of #2658 — hook-rename binding detection.
var navigationHookNames = map[string]bool{
	"useNavigation": true,
	"useRouter":     true,
	"useNavigate":   true,
}

// navigationMethodNames is the set of method names recognised as navigation
// calls. "push" appears both here (router.push / navigation.push) and in
// builtinMethodNames (Array.push). The navigation detector runs BEFORE the
// normal callTarget path in extractCallRelationships and gates on the full
// receiver.method shape to avoid misidentifying plain array .push() calls.
var navigationMethodNames = map[string]bool{
	"push":     true,
	"navigate": true,
	"replace":  true,
	"back":     true,
	"openURL":  true,
}

// navigationReceiverNames is the set of receiver identifiers whose method
// calls are treated as navigation, regardless of local variable name aliasing.
// This is a conservative allowlist: it covers the most common single-word
// aliases for the Expo Router / React Navigation / Linking APIs.
var navigationReceiverNames = map[string]bool{
	"router":     true,
	"navigation": true,
	"Linking":    true,
}

// extractNavigationCall inspects a single call_expression node and, if it
// matches a navigation pattern, returns the destination route, any param key
// names extracted from the `params:` object, a variable reference if the
// params was a variable reference, and ok=true.
//
// Three argument shapes are handled:
//
//  1. String literal first arg:
//     router.push('/foo')  →  route='/foo', params=nil
//
//  2. Template literal first arg (dynamic — captured as-is):
//     router.push(`/foo/${id}`)  →  route='`/foo/${id}`', params=nil
//
//  3. Object-form first arg with pathname + optional params:
//     router.push({pathname: '/users/[id]', params: {id, type}})
//     →  route='/users/[id]', params=['id','type']
//
//  4. No first arg (e.g. router.back()):
//     →  route='<back>'
func extractNavigationCall(x *extractor, call *sitter.Node) (route string, params []string, varRef string, ok bool) {
	if call == nil {
		return "", nil, "", false
	}

	// Must be a call_expression whose function child is a member_expression.
	fn := call.ChildByFieldName("function")
	if fn == nil || fn.Type() != "member_expression" {
		return "", nil, "", false
	}

	// Resolve receiver and method names.
	objNode := fn.ChildByFieldName("object")
	propNode := fn.ChildByFieldName("property")
	if objNode == nil || propNode == nil {
		return "", nil, "", false
	}

	receiver := x.nodeText(objNode)
	method := x.nodeText(propNode)

	// Receiver must be in the static allowlist OR bound to a navigation hook
	// via the hookVarToNavModule table built in buildNavigationHookVarTable.
	// Phase 2 of #2658 — hook-rename binding detection.
	if !navigationReceiverNames[receiver] && !x.isNavigationHookVar(receiver) {
		return "", nil, "", false
	}
	// Method must be a navigation verb.
	if !navigationMethodNames[method] {
		return "", nil, "", false
	}

	// router.back() / navigation.back() — no argument, route stub.
	if method == "back" {
		return "<back>", nil, "", true
	}

	// Inspect the arguments list.
	args := call.ChildByFieldName("arguments")
	if args == nil {
		// No args node at all (shouldn't happen for these methods, but be safe).
		return "<" + method + ">", nil, "", true
	}

	// Find the first non-punctuation child of the arguments node.
	firstArg := firstMeaningfulArg(args)
	if firstArg == nil {
		// e.g. Linking.openURL() with no argument — skip.
		return "", nil, "", false
	}

	switch firstArg.Type() {
	case "string":
		// Quoted string literal: 'screen' or "/path".
		raw := x.nodeText(firstArg)
		route = strings.Trim(raw, `"'`+"`")
		return route, nil, "", true

	case "template_string":
		// Template literal — normalise ${…} slots to {*} so the route
		// can be matched against server-side definitions (Phase 2, #2658).
		route = normalizeTemplateLiteralRoute(x.nodeText(firstArg))
		return route, nil, "", true

	case "object", "object_expression":
		// Object-form: {pathname: '/x', params: {a, b, ...}}.
		route, params, varRef, ok = extractObjectFormRoute(x, firstArg)
		return route, params, varRef, ok

	default:
		// Identifier or other expression — dynamic / unresolvable.
		return "", nil, "", false
	}
}

// firstMeaningfulArg returns the first child of an arguments node that is not
// a punctuation token ("(", ")", ",").
func firstMeaningfulArg(args *sitter.Node) *sitter.Node {
	for i := 0; i < int(args.ChildCount()); i++ {
		ch := args.Child(i)
		if ch == nil {
			continue
		}
		t := ch.Type()
		if t == "(" || t == ")" || t == "," {
			continue
		}
		return ch
	}
	return nil
}

// extractObjectFormRoute parses an object literal node for the
// `pathname` key and the `params` key, returning the route, param key names,
// and any variable reference (#2672).
func extractObjectFormRoute(x *extractor, obj *sitter.Node) (route string, params []string, varRef string, ok bool) {
	for i := 0; i < int(obj.ChildCount()); i++ {
		child := obj.Child(i)
		if child == nil {
			continue
		}
		// Both JS and TS grammars use "pair" for key: value inside objects.
		if child.Type() != "pair" {
			continue
		}
		keyNode := child.ChildByFieldName("key")
		valNode := child.ChildByFieldName("value")
		if keyNode == nil || valNode == nil {
			continue
		}
		key := strings.Trim(x.nodeText(keyNode), `"'`+"`")
		switch key {
		case "pathname":
			raw := x.nodeText(valNode)
			route = strings.Trim(raw, `"'`+"`")
		case "params":
			params, varRef = extractParamKeys(x, valNode)
		}
	}
	if route == "" {
		return "", nil, "", false
	}
	return route, params, varRef, true
}

// extractParamKeys returns the property/shorthand key names from an object
// expression used as the `params:` value in a navigation call.
//
// Handles:
//   - explicit `{id: value}` (pair)
//   - shorthand `{id}` (shorthand_property_identifier[_pattern])
//   - spread elements `{...rest}` — recorded as the sentinel "...spread" so
//     downstream queries know dynamic keys may exist (#2665)
//   - variable references `params: opts` — recorded as varRef for post-walk
//     resolution (#2672)
//
// If the params value is not an object literal, the params slice will be empty
// and varRef will be non-empty (variable reference case).
func extractParamKeys(x *extractor, obj *sitter.Node) (params []string, varRef string) {
	if obj == nil {
		return nil, ""
	}
	// Unwrap through one level of parenthesisation.
	if obj.Type() == "parenthesized_expression" {
		for i := 0; i < int(obj.ChildCount()); i++ {
			ch := obj.Child(i)
			if ch != nil && (ch.Type() == "object" || ch.Type() == "object_expression") {
				obj = ch
				break
			}
		}
	}
	if obj.Type() != "object" && obj.Type() != "object_expression" {
		// #2665/#2672: caller passed e.g. `params: opts` — variable reference.
		// Record the variable name for post-walk resolution in resolveParamsVarRefs.
		if obj.Type() == "identifier" {
			varName := x.nodeText(obj)
			if varName != "" {
				// Record the variable name and its source location for later resolution.
				x.recordParamsVarRef(varName, obj)
				return []string{}, varName
			}
		}
		// Return empty params and no varRef (couldn't identify the reference).
		return []string{}, ""
	}

	params = extractParamKeysFromObjectNode(x, obj)
	return params, ""
}

// emitNavigationEdge constructs a NAVIGATES_TO RelationshipRecord from a
// navigation call site, with optional tracking of variable references for later
// resolution. The ToID is the route string (prefixed with "route:") so it forms
// a stable synthetic stub ID that the linker can later match against declared
// route entities.
//
// Properties set:
//   - "route"       : the destination route/screen name
//   - "params"      : comma-separated param key names (omitted when empty)
//   - "params_keys" : JSON array string of sorted, deduped param keys (#2665).
//                     Empty array "[]" indicates the params object existed but
//                     contained no statically-extractable keys (e.g. variable
//                     reference). Property is omitted entirely when no params
//                     object was observed.
//   - "line"        : 1-indexed source line of the call site
//   - "via"         : "navigation_call" (traceability tag)
//   - "_var_ref"    : (temporary, #2672) variable name if params was a reference;
//                     removed after resolution
func emitNavigationEdge(route string, params []string, varRef string, call *sitter.Node) types.RelationshipRecord {
	toID := "route:" + route
	props := map[string]string{
		"route": route,
		"line":  strconv.Itoa(int(call.StartPoint().Row) + 1),
		"via":   "navigation_call",
	}
	if len(params) > 0 {
		props["params"] = strings.Join(params, ", ")
	}
	// #2665: emit params_keys whenever a params object was observed (params != nil),
	// even when empty — distinguishes "no params arg" from "params: <dynamic>".
	if params != nil {
		// Dedupe + sort. extractParamKeys already dedupes, but normalise here
		// so callers that build the edge from other paths get the same shape.
		seen := make(map[string]struct{}, len(params))
		uniq := make([]string, 0, len(params))
		for _, k := range params {
			if k == "" {
				continue
			}
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			uniq = append(uniq, k)
		}
		sort.Strings(uniq)
		if b, err := json.Marshal(uniq); err == nil {
			props["params_keys"] = string(b)
		}
		// #2672: if this was a variable reference, store it temporarily for
		// later resolution.
		if varRef != "" {
			props["_var_ref"] = varRef
		}
	}
	return types.RelationshipRecord{
		ToID:       toID,
		Kind:       "NAVIGATES_TO",
		Properties: props,
	}
}

// ---------------------------------------------------------------------------
// Phase 2 (#2658) — Hook-rename binding helpers
// ---------------------------------------------------------------------------

// isNavigationHookVar reports whether varName is known to hold the result of a
// useNavigation() / useRouter() / useNavigate() hook call. The table is
// pre-built in Extract() via buildNavigationHookVarTable and stored on x.navHookVars.
func (x *extractor) isNavigationHookVar(varName string) bool {
	return x.navHookVars[varName]
}

// ---------------------------------------------------------------------------
// Issue #2672 — params_keys variable-ref resolution helpers
// ---------------------------------------------------------------------------

// paramsVarRef tracks a variable reference encountered as the params argument
// in a navigation call site, along with the source location. Used in a second
// pass to resolve the binding to extract param keys.
type paramsVarRef struct {
	varName    string
	sourceNode *sitter.Node
}

// recordParamsVarRef records a variable reference for later resolution. Called
// during initial extraction when a params: <identifier> is encountered.
func (x *extractor) recordParamsVarRef(varName string, sourceNode *sitter.Node) {
	if x.paramsVarRefs == nil {
		x.paramsVarRefs = make([]*paramsVarRef, 0, 8)
	}
	x.paramsVarRefs = append(x.paramsVarRefs, &paramsVarRef{
		varName:    varName,
		sourceNode: sourceNode,
	})
}

// buildNavigationHookVarTable scans the AST for variable declarations of the
// form `const <varName> = <hookName>(...)` where <hookName> is in
// navigationHookNames. Returns a map from varName → true for every variable
// that should be treated as a navigation receiver.
//
// This is called once per file from Extract() (alongside buildHookVarToModule)
// and stored on x.navHookVars so extractNavigationCall can consult it via
// isNavigationHookVar without needing to re-traverse the AST. Phase 2 of #2658.
func buildNavigationHookVarTable(x *extractor, root *sitter.Node) map[string]bool {
	if root == nil {
		return nil
	}
	out := make(map[string]bool)

	// Walk the AST looking for:
	//   const <varName> = <hookName>(...)
	// where <hookName> ∈ navigationHookNames.
	stack := make([]*sitter.Node, 0, 64)
	stack = append(stack, root)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil {
			continue
		}
		if n.Type() == "variable_declarator" {
			nameNode := n.ChildByFieldName("name")
			valNode := n.ChildByFieldName("value")
			if nameNode != nil && valNode != nil && valNode.Type() == "call_expression" {
				localName := x.nodeText(nameNode)
				if localName != "" && !strings.ContainsAny(localName, "{}[].,") {
					fnNode := valNode.ChildByFieldName("function")
					if fnNode != nil {
						hookName := x.nodeText(fnNode)
						if navigationHookNames[hookName] {
							out[localName] = true
						}
					}
				}
			}
		}
		count := int(n.ChildCount())
		for i := count - 1; i >= 0; i-- {
			stack = append(stack, n.Child(i))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// resolveParamsVarRefs is called after the initial walk() to resolve variable
// references encountered in params: arguments. For each recorded variable, we
// search for the most recent const/let/var binding whose RHS is an object
// literal and appears textually before the reference site. If found and the RHS
// is an object literal, we extract its keys and update the corresponding
// NAVIGATES_TO edge.
//
// Issue #2672: same-file symbol-table resolution for params_keys from variable
// references. This is a lightweight approach that avoids cross-file data flow.
func (x *extractor) resolveParamsVarRefs(root *sitter.Node) {
	if len(x.paramsVarRefs) == 0 {
		return
	}

	// For each recorded variable reference, find its binding and extract keys.
	for _, ref := range x.paramsVarRefs {
		keys := x.findVariableBinding(root, ref.varName, ref.sourceNode)
		// Update the relationship that has this reference (even if no keys found).
		x.updateNavigatesEdgeParamKeys(ref.varName, keys)
	}
}

// findVariableBinding searches the AST for a const/let/var declaration or
// assignment of varName that appears textually before refNode and has an
// object literal as its RHS. Returns the extracted keys if found, or nil
// otherwise. The search looks for the MOST RECENT binding that satisfies the
// constraints (last-in-scope semantics).
func (x *extractor) findVariableBinding(root *sitter.Node, varName string, refNode *sitter.Node) []string {
	if root == nil || varName == "" {
		return nil
	}

	refRow := refNode.StartPoint().Row

	// Walk the AST looking for:
	//   1. variable_declarator nodes with matching name (const/let/var)
	//   2. assignment_expression nodes where LHS is the variable and RHS is an object literal
	// We collect all candidates and pick the one with the highest row number
	// that is still before refRow (last-in-scope wins).
	var bestBinding *sitter.Node
	bestRow := int32(-1) // Use -1 as initial value so any valid row (>=0) will be "better"

	stack := make([]*sitter.Node, 0, 128)
	stack = append(stack, root)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil {
			continue
		}

		if n.Type() == "variable_declarator" {
			nameNode := n.ChildByFieldName("name")
			valNode := n.ChildByFieldName("value")
			if nameNode != nil && valNode != nil {
				declName := x.nodeText(nameNode)
				// Match the variable name and ensure the declaration appears before the reference.
				if declName == varName {
					declRow := int32(n.StartPoint().Row)
					// Only consider declarators that appear textually before the reference
					// and are object literals (or unwrap to object literals through
					// parenthesisation).
					if declRow < int32(refRow) {
						// Check if the RHS is an object literal.
						if isObjectLiteral(valNode) {
							// Pick the declaration closest to (but before) the reference
							// to implement last-in-scope semantics.
							if declRow > bestRow {
								bestBinding = valNode
								bestRow = declRow
							}
						}
					}
				}
			}
		} else if n.Type() == "assignment_expression" {
			// Handle reassignments like: varName = { ... }
			leftNode := n.ChildByFieldName("left")
			rightNode := n.ChildByFieldName("right")
			if leftNode != nil && rightNode != nil {
				assignName := x.nodeText(leftNode)
				// Match the variable name and ensure it appears before the reference.
				if assignName == varName {
					assignRow := int32(n.StartPoint().Row)
					if assignRow < int32(refRow) {
						// Check if the RHS is an object literal.
						if isObjectLiteral(rightNode) {
							// Pick the assignment closest to (but before) the reference
							// to implement last-in-scope semantics.
							if assignRow > bestRow {
								bestBinding = rightNode
								bestRow = assignRow
							}
						}
					}
				}
			}
		}

		count := int(n.ChildCount())
		for i := count - 1; i >= 0; i-- {
			stack = append(stack, n.Child(i))
		}
	}

	if bestBinding == nil {
		return nil
	}

	// Extract keys from the best-match binding's RHS.
	return extractParamKeysFromObjectNode(x, bestBinding)
}

// isObjectLiteral reports whether a node is an object literal, possibly wrapped
// in parentheses (unwraps once).
func isObjectLiteral(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	t := node.Type()
	if t == "object" || t == "object_expression" {
		return true
	}
	// Unwrap one level of parenthesisation.
	if t == "parenthesized_expression" {
		for i := 0; i < int(node.ChildCount()); i++ {
			ch := node.Child(i)
			if ch != nil {
				ct := ch.Type()
				if ct == "object" || ct == "object_expression" {
					return true
				}
			}
		}
	}
	return false
}

// extractParamKeysFromObjectNode is similar to extractParamKeys but directly
// operates on an object literal node (assumed to be valid). Used by
// findVariableBinding to extract keys from a resolved variable binding.
func extractParamKeysFromObjectNode(x *extractor, obj *sitter.Node) []string {
	if obj == nil {
		return nil
	}

	// Unwrap parenthesisation.
	if obj.Type() == "parenthesized_expression" {
		for i := 0; i < int(obj.ChildCount()); i++ {
			ch := obj.Child(i)
			if ch != nil && (ch.Type() == "object" || ch.Type() == "object_expression") {
				obj = ch
				break
			}
		}
	}

	if obj.Type() != "object" && obj.Type() != "object_expression" {
		return nil
	}

	var keys []string
	seen := make(map[string]bool)
	for i := 0; i < int(obj.ChildCount()); i++ {
		child := obj.Child(i)
		if child == nil {
			continue
		}
		var keyName string
		switch child.Type() {
		case "pair":
			if kn := child.ChildByFieldName("key"); kn != nil {
				keyName = strings.Trim(x.nodeText(kn), `"'`+"`")
			}
		case "shorthand_property_identifier", "shorthand_property_identifier_pattern":
			keyName = x.nodeText(child)
		case "spread_element":
			keyName = "...spread"
		}
		if keyName != "" && !seen[keyName] {
			seen[keyName] = true
			keys = append(keys, keyName)
		}
	}
	return keys
}

// updateNavigatesEdgeParamKeys updates the NAVIGATES_TO relationships that
// reference varName with the resolved param keys. This is called after
// findVariableBinding has resolved a variable reference.
// Relationships are embedded in entities, so we must search through all
// entities and their relationships.
func (x *extractor) updateNavigatesEdgeParamKeys(varName string, keys []string) {
	if len(x.entities) == 0 {
		return
	}

	// Search through all entities' relationships for NAVIGATES_TO edges
	// marked with the variable reference.
	for _, e := range x.entities {
		if len(e.Relationships) == 0 {
			continue
		}
		for i := range e.Relationships {
			rel := &e.Relationships[i]
			if rel.Kind != "NAVIGATES_TO" {
				continue
			}
			if rel.Properties == nil {
				continue
			}
			// Check if this edge was marked with the variable reference.
			if tmpVar, hasVar := rel.Properties["_var_ref"]; hasVar && tmpVar == varName {
				// Match! Update with resolved keys (if any were found).
				if len(keys) > 0 {
					sort.Strings(keys)
					if b, err := json.Marshal(keys); err == nil {
						rel.Properties["params_keys"] = string(b)
					}
				}
				// Remove the temporary marker.
				delete(rel.Properties, "_var_ref")
			}
		}
	}
}
