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

// goStdlibBuiltinNames is the Go-specific set of bare-name builtin targets.
// These are the identifiers declared in the Go universe block — they cannot
// be imported from any package and should never produce a placeholder External
// entity. Type names that collide with common user-defined symbols (string,
// int, int64, byte, bool, float64, error) are intentionally excluded because
// they are often valid type-entity names inside a graph.
var goStdlibBuiltinNames = map[string]struct{}{
	// Built-in functions (spec: https://go.dev/ref/spec#Built-in_functions)
	"make":    {},
	"new":     {},
	"len":     {},
	"cap":     {},
	"append":  {},
	"copy":    {},
	"delete":  {},
	"print":   {},
	"println": {},
	"panic":   {},
	"recover": {},
	"close":   {},
	"complex": {},
	"real":    {},
	"imag":    {},
}

// javascriptStdlibBuiltinNames covers both JavaScript and TypeScript. These are
// the global/built-in objects that are always present in any JS/TS runtime and
// can never be resolved to a user-defined entity or a real npm package.
// Collision-prone names that are common method names in user code are excluded.
var javascriptStdlibBuiltinNames = map[string]struct{}{
	// Core language built-in globals (ECMAScript standard)
	"console":        {},
	"JSON":           {},
	"Math":           {},
	"Object":         {},
	"Array":          {},
	"String":         {},
	"Number":         {},
	"Boolean":        {},
	"Date":           {},
	"RegExp":         {},
	"Map":            {},
	"Set":            {},
	"WeakMap":        {},
	"WeakSet":        {},
	"Promise":        {},
	"Symbol":         {},
	"BigInt":         {},
	"Error":          {},
	"TypeError":      {},
	"RangeError":     {},
	"ReferenceError": {},
	// Browser globals
	"window":                {},
	"document":              {},
	"localStorage":          {},
	"sessionStorage":        {},
	"fetch":                 {},
	"URL":                   {},
	"URLSearchParams":       {},
	"Headers":               {},
	"Request":               {},
	"Response":              {},
	"FormData":              {},
	"Blob":                  {},
	"File":                  {},
	"FileReader":            {},
	"setTimeout":            {},
	"setInterval":           {},
	"clearTimeout":          {},
	"clearInterval":         {},
	"requestAnimationFrame": {},
	"cancelAnimationFrame":  {},
	// Node.js globals
	"process":    {},
	"Buffer":     {},
	"globalThis": {},
	"require":    {},
	"module":     {},
	"__dirname":  {},
	"__filename": {},
}

// rubyStdlibBuiltinNames covers Ruby built-in kernel methods and core stdlib
// classes that cannot be user-defined under Ruby conventions. Collision-prone
// names (String, Integer, Float, Array, Hash — which are valid class entities
// in a user codebase) are excluded deliberately.
var rubyStdlibBuiltinNames = map[string]struct{}{
	// Kernel / top-level output methods
	"puts":  {},
	"print": {},
	"p":     {},
	"pp":    {},
	"raise": {},
	// Class macro methods (Module-level DSL)
	"attr_accessor": {},
	"attr_reader":   {},
	"attr_writer":   {},
	// Forwardable module DSL
	"def_delegators": {},
	"delegate":       {},
	// Core stdlib classes that appear unambiguously as bare-name calls and
	// cannot collide with real user-defined class names under Ruby conventions.
	"Symbol": {},
	"Range":  {},
	"Regexp": {},
	"Time":   {},
	"Date":   {},
}

// stdlibBuiltinsByLang maps a normalised language tag to its per-language
// stdlib-builtin name set. Only languages with a non-trivial builtin surface
// that produces significant External entity noise are listed here. Other
// languages' stdlib symbols are filtered upstream by different mechanisms
// (e.g. goBareNames / goPackageFold in classifyExternal for Go).
var stdlibBuiltinsByLang = map[string]map[string]struct{}{
	"python":     pythonStdlibBuiltinNames,
	"go":         goStdlibBuiltinNames,
	"javascript": javascriptStdlibBuiltinNames,
	"typescript": javascriptStdlibBuiltinNames, // same set; TS is a JS superset
	"ruby":       rubyStdlibBuiltinNames,
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
