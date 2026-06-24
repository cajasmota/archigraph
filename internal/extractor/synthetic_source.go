package extractor

// synthetic_source.go — shared helper for recognizing the synthetic SourceFile
// sentinels grafel emits for entities that have no real backing file on disk.
//
// Several extractors assign a constant, angle-bracketed SourceFile to entities
// that are aggregated to a single graph node rather than tied to one physical
// file (config keys, exception types, external services, translation keys,
// templates). These sentinels are NOT real paths and must never be handed to
// the filesystem (os.Stat / os.Open / os.ReadFile): the characters "<" and ">"
// are legal on POSIX (where a stat merely returns fs.ErrNotExist) but ILLEGAL
// on Windows, where the syscall returns ERROR_INVALID_NAME (123). A pass that
// only tolerates fs.ErrNotExist therefore aborts on Windows — see issue #5523,
// where the cross-repo string pass zeroed out all cross-repo edges.
//
// Any pass that scans entity SourceFiles against the filesystem should call
// IsSyntheticSourceFile and skip matches BEFORE touching the filesystem.

// syntheticSourceFiles is the set of known synthetic SourceFile sentinels.
// Keep this in sync with the per-extractor constants:
//   - ConfigKeySourceFile        ("<config>")        — config_key.go
//   - ExceptionTypeSourceFile    ("<exception>")     — exception_flow.go
//   - ExternalServiceSourceFile  ("<external-service>") — external_service.go
//   - TranslationKeySourceFile   ("<translation-key>")  — translation_key.go
//   - TemplateSourceFile         ("<template>")      — template_render.go
var syntheticSourceFiles = map[string]bool{
	ConfigKeySourceFile:       true,
	ExceptionTypeSourceFile:   true,
	ExternalServiceSourceFile: true,
	TranslationKeySourceFile:  true,
	TemplateSourceFile:        true,
}

// IsSyntheticSourceFile reports whether path is a synthetic SourceFile sentinel
// — either one of the known constants above, or any value matching the general
// "<...>" angle-bracketed shape grafel uses for synthetic sources. Such a value
// never corresponds to a real file on any platform and must be skipped before
// any filesystem access (it is a hard error on Windows).
func IsSyntheticSourceFile(path string) bool {
	if syntheticSourceFiles[path] {
		return true
	}
	// General shape guard: a non-empty value wrapped in angle brackets, e.g.
	// "<config>", "<generated>", "<stdin>", "<synthetic>". This future-proofs
	// the check against new synthetic sentinels added without updating the set.
	if len(path) >= 2 && path[0] == '<' && path[len(path)-1] == '>' {
		return true
	}
	return false
}
