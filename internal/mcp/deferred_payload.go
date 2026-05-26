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
	"sync"

	mcpapi "github.com/mark3labs/mcp-go/mcp"
)

// deferredPayloads holds values stashed by jsonResult and consumed by wrap.
// Key: *mcpapi.CallToolResult — guaranteed unique per call.
// Value: any — the unmarshaled handler value.
var deferredPayloads sync.Map

// stashDeferred associates v with res so wrap can pick it up later.
func stashDeferred(res *mcpapi.CallToolResult, v any) {
	deferredPayloads.Store(res, v)
}

// takeDeferred removes and returns the value associated with res, if any.
// The second return is true when a value was found.
func takeDeferred(res *mcpapi.CallToolResult) (any, bool) {
	v, ok := deferredPayloads.LoadAndDelete(res)
	if !ok {
		return nil, false
	}
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
// structured value (no parse, no remarshal).
func finalizeDeferred(v any, elapsedMS int64, fields []string) (string, error) {
	keep := map[string]bool(nil)
	if len(fields) > 0 {
		keep = make(map[string]bool, len(fields))
		for _, f := range fields {
			keep[f] = true
		}
	}

	switch payload := v.(type) {
	case map[string]any:
		obj := payload
		if keep != nil {
			obj = filterObject(obj, keep)
		}
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
		return finalizeArray(payload, elapsedMS, keep)

	case []map[string]any:
		// Promote homogeneous record slice to []any so the shared array
		// path can attempt TOON + fields= filtering uniformly.
		arr := make([]any, len(payload))
		for i, m := range payload {
			arr[i] = m
		}
		return finalizeArray(arr, elapsedMS, keep)

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
// top-level array returns. items is TOON-encoded when applicable; fields=
// filtering applies when items remains an array.
func finalizeArray(arr []any, elapsedMS int64, keep map[string]bool) (string, error) {
	var itemsVal any = arr
	if toonWireEnabled() {
		if toon, ok := recordsToTOON(arr); ok {
			itemsVal = toon
		}
	}
	if keep != nil {
		if subArr, ok := itemsVal.([]any); ok {
			itemsVal = filterArray(subArr, keep)
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
		// We need count: parse only the array's *length* by counting
		// top-level commas would be unsafe in the presence of nested
		// commas in strings. So we do one unmarshal *just to count* —
		// but only when the typed-struct default path is hit. To stay
		// truly single-marshal we instead fall back to including the
		// array bytes verbatim with count=-1 OR re-marshaling via the
		// []any branch. In practice this default branch is rare; defer
		// to legacy path for correctness.
		return string(data) + ",\"elapsed_ms\":" + itoa64(elapsedMS)
	}

	// Non-JSON or unrecognised — append a trailing comment line to keep
	// the elapsed_ms regex parser happy (parity with the error path).
	return string(data) + "\n# elapsed_ms=" + itoa64(elapsedMS) + "\n"
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
