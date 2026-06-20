// field_validations.go — #5049 (follow-up #4942). Per-field validation
// constraints for F# record fields, stamped onto the SCOPE.Schema/field entity
// under Properties["validations"] (comma-joined) so the dashboard ShapeTree
// surfaces them as small constraint chips — mirroring the Java Bean-Validation
// support (#4872, internal/extractors/java/field_validations.go), the TS
// class-validator support (#4858) and the Python field-validation support
// (#4871).
//
// The dashboard side already exists: internal/dashboard/shape_tree.go reads
// Properties["validations"] (comma-split) into v2ShapeRow.Validations, and
// webui-v2/src/components/ShapeTree.tsx renders the chips. This pass only has
// to populate that property on the F# record-field entities.
//
// F# DataAnnotations attributes are written in the `[<Attribute>]` syntax and
// sit on the line(s) preceding the record field they decorate, e.g.
//
//	type CreateUserDto = {
//	    [<Required>]
//	    [<EmailAddress>]
//	    Email: string
//
//	    [<StringLength(120)>]
//	    [<MinLength(2)>]
//	    Name: string
//	}
//
// Several attributes may also share a single bracket as a `;`-separated list
// (`[<Required; EmailAddress>]`). The chip text is kept terse and comma-free
// (Properties is comma-joined and the dashboard splits on ","): scalar bounds
// fold their value (`MaxLength:120`, `Range:1..5`), the rest render as a bare
// marker (`Required`, `Email`). Only stamped when at least one constraint is
// found; existing Properties are preserved.
package fsharp

import (
	"strings"
)

// fsValidationMarkers maps a DataAnnotations attribute simple name (without the
// `Attribute` suffix) to the bare marker chip it produces. These carry no
// scalar bound worth folding, so only the attribute identity is recorded.
//
// [<Required>] renders as `Required` to match the Java/TS/Python required
// marker.
var fsValidationMarkers = map[string]string{
	"Required":         "Required",
	"EmailAddress":     "Email",
	"Phone":            "Phone",
	"Url":              "Url",
	"CreditCard":       "CreditCard",
	"Compare":          "Compare",
	"DataType":         "DataType",
	"EnumDataType":     "EnumDataType",
	"FileExtensions":   "FileExtensions",
	"Base64String":     "Base64String",
	"AllowedValues":    "AllowedValues",
	"DeniedValues":     "DeniedValues",
	"Timestamp":        "Timestamp",
	"ConcurrencyCheck": "ConcurrencyCheck",
}

// fsValidationScalar maps a DataAnnotations attribute whose single argument is a
// scalar bound to the chip prefix it folds into (`[<MaxLength(120)>]` →
// "MaxLength:120").
var fsValidationScalar = map[string]string{
	"MaxLength":    "MaxLength",
	"MinLength":    "MinLength",
	"StringLength": "MaxLength", // StringLength(max) → MaxLength:max
}

// fsValidationChips parses the attribute lines preceding an F# record field and
// returns the terse, comma-free validation chips. `attrLines` are the raw
// source lines that appeared above the field, each potentially carrying one or
// more `[<...>]` attribute groups. Unrecognised attributes are ignored so the
// chip list stays validation-only (a `[<JsonProperty>]` or `[<CLIMutable>]`
// attribute never leaks in).
func fsValidationChips(attrLines []string) []string {
	var chips []string
	seen := make(map[string]bool)
	add := func(c string) {
		if c == "" || seen[c] {
			return
		}
		seen[c] = true
		chips = append(chips, c)
	}
	for _, line := range attrLines {
		for _, attr := range fsParseAttrGroup(line) {
			name, arg := fsSplitAttr(attr)
			name = strings.TrimSuffix(name, "Attribute")
			switch {
			case name == "Range":
				// [<Range(1, 5)>] → Range:1..5
				lo, hi, ok := fsRangeBounds(arg)
				if ok {
					add("Range:" + lo + ".." + hi)
				} else {
					add("Range")
				}
			case name == "RegularExpression":
				// Pattern body is a regex full of commas/brackets — record only
				// the marker so it can't corrupt the comma-joined property.
				add("Pattern")
			case fsValidationScalar[name] != "":
				v := fsFirstArg(arg)
				if v != "" {
					add(fsValidationScalar[name] + ":" + v)
				} else {
					add(fsValidationScalar[name])
				}
			case fsValidationMarkers[name] != "":
				add(fsValidationMarkers[name])
			}
		}
	}
	return chips
}

// fsParseAttrGroup extracts the individual attribute bodies from a source line,
// splitting `[<A>] [<B>]` into ["A","B"] and `[<A; B(1)>]` into ["A","B(1)"].
// Only the text between `[<` and `>]` is considered.
func fsParseAttrGroup(line string) []string {
	var out []string
	for {
		open := strings.Index(line, "[<")
		if open < 0 {
			break
		}
		close := strings.Index(line[open:], ">]")
		if close < 0 {
			break
		}
		inner := line[open+2 : open+close]
		for _, seg := range strings.Split(inner, ";") {
			seg = strings.TrimSpace(seg)
			if seg != "" {
				out = append(out, seg)
			}
		}
		line = line[open+close+2:]
	}
	return out
}

// fsSplitAttr splits an attribute body `StringLength(120)` into its name
// (`StringLength`) and raw argument text (`120`). A bare attribute (`Required`)
// returns an empty argument.
func fsSplitAttr(attr string) (name, arg string) {
	attr = strings.TrimSpace(attr)
	if p := strings.IndexByte(attr, '('); p >= 0 {
		name = strings.TrimSpace(attr[:p])
		arg = attr[p+1:]
		if c := strings.LastIndexByte(arg, ')'); c >= 0 {
			arg = arg[:c]
		}
		return name, strings.TrimSpace(arg)
	}
	return attr, ""
}

// fsFirstArg returns the first comma-separated argument of an attribute's raw
// argument text, dropping any named-argument noise (`ErrorMessage = "..."`).
func fsFirstArg(arg string) string {
	if arg == "" {
		return ""
	}
	first := strings.SplitN(arg, ",", 2)[0]
	first = strings.TrimSpace(first)
	// A named argument (e.g. ErrorMessage="x") is not a scalar bound.
	if strings.ContainsRune(first, '=') || strings.ContainsRune(first, '"') {
		return ""
	}
	return first
}

// fsRangeBounds parses a `[<Range(lo, hi)>]` argument list into its low/high
// scalar bounds, ignoring any trailing named arguments.
func fsRangeBounds(arg string) (lo, hi string, ok bool) {
	parts := strings.Split(arg, ",")
	if len(parts) < 2 {
		return "", "", false
	}
	lo = strings.TrimSpace(parts[0])
	hi = strings.TrimSpace(parts[1])
	if lo == "" || hi == "" {
		return "", "", false
	}
	if strings.ContainsAny(lo, "=\"") || strings.ContainsAny(hi, "=\"") {
		return "", "", false
	}
	return lo, hi, true
}
