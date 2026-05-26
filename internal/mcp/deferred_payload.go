// deferred_payload.go — single-marshal-at-the-wire envelope (#2287).
//
// Before this file existed, every JSON-shaped MCP response went through:
//
//  1. handler builds a value v (typically map[string]any or []any)
//  2. jsonResult(v) marshals v -> []byte -> stored in TextContent.Text
//  3. wrap() calls injectElapsedMS, which UNMARSHALS the bytes back into a
//     map/array, mutates it (adds elapsed_ms, maybe TOON-encodes items),
//     and MARSHALS again.
//  4. The bytes go on the wire.
//
// That's marshal -> parse -> marshal per call, and the parse step's cost
// scales linearly with payload size (worst on endpoints, clusters,
// get_source). #2287 collapses (2)+(3) into a single marshal at the wire
// boundary.
//
// Mechanism:
//   - jsonResult() stops marshaling up front. It allocates an empty
//     *CallToolResult and stashes the raw value v in a package-level
//     sync.Map keyed by the result pointer.
//   - wrap() looks up the pointer after the handler returns. If a deferred
//     value is present it:
//      - applies fields= filtering on the structured value (no parse)
//      - performs TOON conversion on items arrays (no parse)
//      - injects elapsed_ms into the envelope
//      - marshals exactly ONCE and writes the bytes into res.Content[0]
//   - Results that don't have a deferred payload (markdown handlers,
//     errors, hand-built TextContent in tests) fall through to the
//     existing injectElapsedMS path — back-compat preserved.
//
// Concurrency: sync.Map is safe under load. Each *CallToolResult returned
// by jsonResult is freshly allocated, so pointer-identity collisions are
// impossible. The wrap finalizer Loads-then-Deletes; entries cannot leak
// because every jsonResult result MUST flow through wrap (it's the only
// dispatch path that calls a tool handler).

package mcp

import (
	"encoding/json"

	mcpapi "github.com/mark3labs/mcp-go/mcp"
)

// HandlerResult is the typed envelope that pairs an unmarshaled handler
// value with the *CallToolResult that carries it on the wire.
//
// #2327: prior to this issue the deferred value was stashed in a
// package-level sync.Map keyed on the *CallToolResult pointer. That was
// pure plumbing for a missing abstraction — every jsonResult callsite
// allocated an empty *CallToolResult only so the package-level map could
// rendezvous with it inside `wrap`.
//
// The cleaner shape is to carry the typed value inline on the result
// itself. The upstream mcp-go protocol type already has a
// `StructuredContent any` field for exactly this purpose (designed for
// structured tool output). We co-opt it as the deferred-value carrier
// during dispatch and clear it before the result leaves the wrap
// boundary. Net effect: package-level sync.Map deleted; no allocation
// for a sidechannel; no lifetime/leak concerns; the typed envelope is
// the *CallToolResult itself.
//
// The pair `HandlerResult{Value, Format}` from the design brief is
// realised by the (StructuredContent, IsError/Content[0]) fields on
// *mcpapi.CallToolResult itself — handlers continue to return that
// pointer, but jsonResult now sets StructuredContent so the dispatcher
// sees a typed value, not just wire bytes.

// stashDeferred stores v on res so wrap can pick it up later. Uses the
// mcp-go protocol type's StructuredContent field as the carrier; that
// field is cleared again before the response goes on the wire, so it
// does not affect the wire envelope shape.
func stashDeferred(res *mcpapi.CallToolResult, v any) {
	if res == nil {
		return
	}
	res.StructuredContent = v
}

// takeDeferred reads and clears the value stashed by jsonResult. Returns
// (value, true) when present; (nil, false) for results produced by other
// paths (markdown, error, hand-built TextContent).
func takeDeferred(res *mcpapi.CallToolResult) (any, bool) {
	if res == nil || res.StructuredContent == nil {
		return nil, false
	}
	v := res.StructuredContent
	res.StructuredContent = nil
	return v, true
}

// finalizeDeferred turns a stashed value into the final on-the-wire JSON
// bytes. This is the single marshal point — there is no parse, no
// remarshal. It handles:
//
//   - object payloads (map[string]any): inject elapsed_ms; if items is a
//     []any of homogeneous records and TOON is enabled, convert items to
//     a TOON string in-place.
//   - array payloads ([]any, []map[string]any): wrap in
//     {items, count, elapsed_ms}; TOON-convert items when applicable.
//   - everything else (typed structs, scalars): marshal to bytes first
//     then byte-level inject elapsed_ms before the trailing '}'. No
//     parse cycle.
//
// fields=, when non-nil, prunes per-record keys directly on the
// structured value (no parse, no remarshal). #2328: typed-struct
// returns are also pruned via reflection (applyFieldsToValue), so
// `fields=` callers ride the single-marshal fast path.
func finalizeDeferred(v any, elapsedMS int64, fields []string) (string, error) {
	keep := map[string]bool(nil)
	if len(fields) > 0 {
		keep = make(map[string]bool, len(fields))
		for _, f := range fields {
			keep[f] = true
		}
	}
	// #2328: apply fields= filtering on typed values before the type
	// switch. For typed-struct shapes this lifts the value into
	// map[string]any / []any form, which then takes the fast path below.
	if keep != nil {
		v = applyFieldsToValue(v, keep)
	}

	switch payload := v.(type) {
	case map[string]any:
		obj := payload
		// Items-array TOON conversion (parity with #1686 path).
		if toonWireEnabled() {
			if rawItems, ok := obj["items"]; ok {
				if arr, ok := rawItems.([]any); ok {
					if toon, ok := recordsToTOON(arr); ok {
						obj["items"] = toon
					}
				}
			}
		}
		obj["elapsed_ms"] = elapsedMS
		data, err := json.Marshal(obj)
		if err != nil {
			return "", err
		}
		return string(data), nil

	case []any:
		return finalizeArray(payload, elapsedMS)

	case []map[string]any:
		// Promote homogeneous record slice to []any so the shared array
		// path can attempt TOON uniformly.
		arr := make([]any, len(payload))
		for i, m := range payload {
			arr[i] = m
		}
		return finalizeArray(arr, elapsedMS)

	default:
		// Typed structs / scalars: marshal once, then byte-inject
		// elapsed_ms before the trailing '}' if it looks like a JSON
		// object. Otherwise wrap in {items, count, elapsed_ms} when it
		// looks like a JSON array. The byte-level path saves a full
		// unmarshal+remarshal vs the legacy injectElapsedMS path.
		data, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return injectElapsedMSIntoBytes(data, elapsedMS), nil
	}
}

// finalizeArray builds the {items, count, elapsed_ms} envelope used for
// top-level array returns. items is TOON-encoded when applicable.
// fields= filtering, if requested, is applied by finalizeDeferred before
// the array reaches this point.
func finalizeArray(arr []any, elapsedMS int64) (string, error) {
	var itemsVal any = arr
	if toonWireEnabled() {
		if toon, ok := recordsToTOON(arr); ok {
			itemsVal = toon
		}
	}
	env := map[string]any{
		"items":      itemsVal,
		"count":      len(arr),
		"elapsed_ms": elapsedMS,
	}
	data, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// injectElapsedMSIntoBytes does a byte-level injection of an elapsed_ms
// field into an already-marshaled JSON object, or wraps an array in an
// {items, count, elapsed_ms} envelope. Cheaper than parse+remarshal.
//
// Best-effort: if data does not parse as a JSON object/array surface (we
// inspect only the first/last non-whitespace byte), we fall back to the
// generic plain-text append used for errors and non-JSON payloads.
func injectElapsedMSIntoBytes(data []byte, elapsedMS int64) string {
	// Find the first non-whitespace byte.
	start := 0
	for start < len(data) {
		b := data[start]
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			start++
			continue
		}
		break
	}
	if start >= len(data) {
		return string(data)
	}

	switch data[start] {
	case '{':
		// Find the matching closing '}'. For well-formed minified
		// json.Marshal output the last byte is '}'. Locate the trailing
		// '}' deterministically by walking backwards over whitespace.
		end := len(data) - 1
		for end > start {
			b := data[end]
			if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
				end--
				continue
			}
			break
		}
		if data[end] != '}' {
			// Unexpected shape — bail to comment append.
			break
		}
		// Empty object {}: insert without leading comma.
		inner := bytesTrimSpaces(data[start+1 : end])
		var b []byte
		if len(inner) == 0 {
			b = make([]byte, 0, len(data)+24)
			b = append(b, data[:start]...)
			b = append(b, '{')
			b = appendElapsedField(b, elapsedMS)
			b = append(b, '}')
			b = append(b, data[end+1:]...)
		} else {
			b = make([]byte, 0, len(data)+25)
			b = append(b, data[:end]...)
			b = append(b, ',')
			b = appendElapsedField(b, elapsedMS)
			b = append(b, data[end:]...)
		}
		return string(b)

	case '[':
		// Wrap array in {items: <bytes>, count: N, elapsed_ms: M}.
		// #2329: compute count by scanning top-level elements with full
		// string/escape awareness — no unmarshal, no second marshal.
		end := len(data) - 1
		for end > start {
			b := data[end]
			if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
				end--
				continue
			}
			break
		}
		if data[end] != ']' {
			break
		}
		count := countTopLevelJSONArrayElements(data[start : end+1])
		// Build {"items":<array-bytes>,"count":N,"elapsed_ms":M}.
		out := make([]byte, 0, len(data)+48)
		out = append(out, data[:start]...)
		out = append(out, '{', '"', 'i', 't', 'e', 'm', 's', '"', ':')
		out = append(out, data[start:end+1]...)
		out = append(out, ',', '"', 'c', 'o', 'u', 'n', 't', '"', ':')
		out = appendInt(out, int64(count))
		out = append(out, ',')
		out = appendElapsedField(out, elapsedMS)
		out = append(out, '}')
		out = append(out, data[end+1:]...)
		return string(out)
	}

	// Non-JSON or unrecognised — append a trailing comment line to keep
	// the elapsed_ms regex parser happy (parity with the error path).
	return string(data) + "\n# elapsed_ms=" + itoa64(elapsedMS) + "\n"
}

// countTopLevelJSONArrayElements counts the top-level elements in a JSON
// array represented by data (must begin with '[' and end with ']'). It walks
// the bytes with string/escape awareness so commas embedded in strings or
// nested arrays/objects are not miscounted. Returns 0 for an empty array
// `[]` or any malformed input.
//
// #2329: implemented so the byte-level array branch of injectElapsedMSIntoBytes
// can populate the envelope's `count` field without a parse cycle. This closes
// the last out-of-scope edge case from #2287.
func countTopLevelJSONArrayElements(data []byte) int {
	if len(data) < 2 || data[0] != '[' {
		return 0
	}
	// Detect empty array (allowing whitespace between [ and ]).
	i := 1
	for i < len(data)-1 {
		b := data[i]
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			i++
			continue
		}
		break
	}
	if i >= len(data)-1 || data[i] == ']' {
		return 0
	}

	count := 1 // at least one element exists past the leading bracket
	depth := 0
	inString := false
	escape := false
	for ; i < len(data)-1; i++ {
		b := data[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			switch b {
			case '\\':
				escape = true
			case '"':
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}

// bytesTrimSpaces returns b stripped of leading and trailing ASCII
// whitespace (no allocation).
func bytesTrimSpaces(b []byte) []byte {
	i, j := 0, len(b)
	for i < j {
		switch b[i] {
		case ' ', '\t', '\n', '\r':
			i++
			continue
		}
		break
	}
	for j > i {
		switch b[j-1] {
		case ' ', '\t', '\n', '\r':
			j--
			continue
		}
		break
	}
	return b[i:j]
}

// appendElapsedField appends `"elapsed_ms":<n>` to b.
func appendElapsedField(b []byte, n int64) []byte {
	b = append(b, '"', 'e', 'l', 'a', 'p', 's', 'e', 'd', '_', 'm', 's', '"', ':')
	b = appendInt(b, n)
	return b
}

// appendInt writes the base-10 ASCII representation of n to b.
func appendInt(b []byte, n int64) []byte {
	if n == 0 {
		return append(b, '0')
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var tmp [20]byte
	i := len(tmp)
	for n > 0 {
		i--
		tmp[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		tmp[i] = '-'
	}
	return append(b, tmp[i:]...)
}

// itoa64 is a small int64 -> string helper (avoids importing strconv just
// for one call site).
func itoa64(n int64) string {
	return string(appendInt(nil, n))
}
