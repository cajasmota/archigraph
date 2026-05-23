package utils

import (
	"strings"
	"unicode"
)

// ToSnakeCase converts a string from CamelCase to snake_case.
func ToSnakeCase(s string) string {
	var res strings.Builder
	res.Grow(len(s) + 5)
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := rune(s[i-1])
				var next rune
				if i < len(s)-1 {
					next = rune(s[i+1])
				}

				if (!unicode.IsUpper(prev) && prev != '_') ||
					(unicode.IsUpper(prev) && next != 0 && !unicode.IsUpper(next) && next != '_') {
					res.WriteRune('_')
				}
			}
			res.WriteRune(unicode.ToLower(r))
		} else {
			res.WriteRune(r)
		}
	}
	return res.String()
}
