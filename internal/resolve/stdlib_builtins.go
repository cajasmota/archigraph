package resolve

// stdlib_builtins.go — per-language sets of bare-name stdlib targets that
// should NEVER produce a placeholder External entity in the graph. Issue #1085.
//
// These are the "pure stdlib" names: language builtins, core type constructors,
// and unambiguous stdlib methods that cannot be user-defined symbols. They are
// distinct from the dynamic-pattern catalog (which handles reflective dispatch
// and framework DSL names) and from the external allowlist (which handles
// real third-party packages).
//
// The synthesiser (internal/external) calls IsStdlibBuiltinTarget before
// emitting a placeholder entity; a true result means the edge should carry a
// "dynamic_target" property instead, and no entity should be created.
//
// Design rules (per issue #94 lesson):
//   - Only include names that are UNAMBIGUOUSLY a language builtin.
//   - Exclude names that collide with common user-defined methods even within
//     the same language (write, read, close, update, pop, clear, etc.).
//   - Do NOT include names that the per-import gate in classifyExternal folds
//     to a canonical package (e.g. Flask DSL names like "route",
//     "before_request" — those should still flow through to ext:flask).

// pythonStdlibBuiltinNames is the Python-specific set of bare-name stdlib
// targets. Populated here rather than in dynamic_patterns_python.go so it
// stays separate from the dispatch-pattern catalog and doesn't accidentally
// affect cross-language tests that verify catalog disjointness.
var pythonStdlibBuiltinNames = map[string]struct{}{
	// Core builtin functions and type constructors (PEP 3102 / builtins module)
	"abs":       {},
	"all":       {},
	"any":       {},
	"bool":      {},
	"callable":  {},
	"chr":       {},
	"dict":      {},
	"enumerate": {},
	"filter":    {},
	"float":     {},
	"format":    {},
	"frozenset": {},
	// getattr/setattr/hasattr/delattr/eval/exec/__import__ are covered by
	// pythonDynamicPatterns (they are reflective primitives, not simple
	// stdlib builtins). Do NOT duplicate them here.
	"hash":       {},
	"id":         {},
	"int":        {},
	"isinstance": {},
	"issubclass": {},
	"iter":       {},
	"len":        {},
	"list":       {},
	"map":        {},
	"max":        {},
	"min":        {},
	"next":       {},
	"object":     {},
	"open":       {},
	"ord":        {},
	"print":      {},
	"property":   {},
	"range":      {},
	"repr":       {},
	"reversed":   {},
	"round":      {},
	"set":        {},
	"slice":      {},
	"sorted":     {},
	"str":        {},
	"sum":        {},
	"super":      {},
	"tuple":      {},
	"type":       {},
	"vars":       {},
	"zip":        {},
	// Python stdlib exceptions — unambiguously built-in, not user-defined
	"Exception":           {},
	"ValueError":          {},
	"TypeError":           {},
	"KeyError":            {},
	"IndexError":          {},
	"AttributeError":      {},
	"RuntimeError":        {},
	"NotImplementedError": {},
	"StopIteration":       {},
	"FileNotFoundError":   {},
	// High-volume Python str/list/dict/set/file methods (bare-name after
	// receiver strip). Exact match only; collision-prone names (write, read,
	// close, update, pop, clear, append, remove, extend, items, keys, values)
	// deliberately excluded per issue #94 — misclassifying a real local method
	// as a stdlib builtin hides real bugs.
	"insert":     {},
	"setdefault": {},
	"startswith": {},
	"endswith":   {},
	"strip":      {},
	"lstrip":     {},
	"rstrip":     {},
	"split":      {},
	"rsplit":     {},
	"splitlines": {},
	"join":       {},
	"lower":      {},
	"upper":      {},
	"title":      {},
	"encode":     {},
	"decode":     {},
	"isdigit":    {},
	"isalpha":    {},
	"isalnum":    {},
	"readline":   {},
	"readlines":  {},
	"writelines": {},
	// flush, seek, tell — kept; collision-prone names (write/read/close)
	// are excluded above.
	"seek": {},
	"tell": {},
	// Python os/stdlib functions (bare-name, no module qualifier)
	"getcwd":          {},
	"listdir":         {},
	"makedirs":        {},
	"deepcopy":        {},
	"deque":           {},
	"defaultdict":     {},
	"OrderedDict":     {},
	"Counter":         {},
	"namedtuple":      {},
	"RawConfigParser": {},
	"ConfigParser":    {},
	// io module: BytesIO, StringIO appear at high volume and cannot be
	// user-defined under normal Python conventions.
	"BytesIO":  {},
	"StringIO": {},
}

// stdlibBuiltinsByLang maps a normalised language tag to its per-language
// stdlib-builtin name set. Only languages with a non-trivial builtin surface
// that produces significant External entity noise are listed here. Other
// languages' stdlib symbols are filtered upstream by different mechanisms
// (e.g. goBareNames / goPackageFold in classifyExternal for Go).
var stdlibBuiltinsByLang = map[string]map[string]struct{}{
	"python": pythonStdlibBuiltinNames,
}

// IsStdlibBuiltinTarget reports whether stub is an unambiguous stdlib builtin
// for the given language — i.e. a bare-name call that should NEVER produce a
// placeholder External entity. The caller (internal/external.Synthesize) uses
// this to stamp "dynamic_target" on the edge and skip entity creation.
//
// Returns false for empty/unknown languages and for names that are not in the
// per-language stdlib-builtin set (those continue through classifyExternal so
// real third-party packages still get their placeholder entities).
func IsStdlibBuiltinTarget(stub, lang string) bool {
	if stub == "" || lang == "" {
		return false
	}
	builtins, ok := stdlibBuiltinsByLang[normalizeLang(lang)]
	if !ok {
		return false
	}
	_, found := builtins[stub]
	return found
}
