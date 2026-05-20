package resolve

import "regexp"

// goDynamicPatterns are per-language patterns for Go.
// Registered via init() into dynamicPatternsByLang.
var goDynamicPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^reflect\.`),       // reflect.* (Call, ValueOf, MethodByName, ...)
	regexp.MustCompile(`\.MethodByName\(`), // v.MethodByName("X").Call(...)
	regexp.MustCompile(`\.FieldByName\(`),  // v.FieldByName("X")
	regexp.MustCompile(`^plugin\.Open\(`),  // Go plugin loader
	// Anchored: only `plugin.Lookup(` (or `<x>.plugin.Lookup(`) — bare
	// `repo.Lookup(id)` / `cache.Lookup(...)` are NOT reflection.
	regexp.MustCompile(`\bplugin\.Lookup\(`),
}

func init() {
	dynamicPatternsByLang["go"] = goDynamicPatterns
}
