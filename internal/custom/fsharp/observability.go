// Package fsharp — observability (log-call) extractor for F# source files.
//
// #5128 (follow-up #5049): recover F# logging call sites as SCOPE.Pattern /
// log_extraction entities so the lang.fsharp.framework.giraffe Observability →
// log_extraction coverage cell lights up. Mirrors the per-framework log-call
// model used by the C# observability extractor
// (internal/custom/csharp/observability.go) — a call-site regex heuristic that
// captures the log FRAMEWORK, the call PATTERN, the log LEVEL, and (where it is
// a string/interpolated literal) the message TEMPLATE on Properties.
//
// # log_extraction (partial — call-site heuristic, no cross-file DI binding)
//
//   - Serilog static API:   Log.Information/Warning/Error/Debug/Fatal/Verbose(...)
//   - Serilog instance API:  logger.Information/Warning/Error/Debug/Fatal/Verbose(...)
//   - Microsoft.Extensions.Logging (MEL):
//     logger.LogInformation/LogWarning/LogError/LogDebug/LogTrace/LogCritical(...)
//   - F# console logging:    printfn / eprintfn / printf / eprintf "...".
//
// NOTE (honest-partial): we do not cross-file-bind an ILogger<T> to its concrete
// handler (that needs dataflow beyond a single-file regex), and a non-literal /
// interpolated message argument is captured as traced=true+dynamic=true without a
// fabricated template. The level is taken from the called method name; the
// message template is the FIRST string-literal argument when present.
package fsharp

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_fsharp_observability", &observabilityExtractor{})
}

type observabilityExtractor struct{}

func (e *observabilityExtractor) Language() string { return "custom_fsharp_observability" }

// ---------------------------------------------------------------------------
// Log regexes
// ---------------------------------------------------------------------------

var (
	// Serilog STATIC API: Log.Information/Warning/Error/Debug/Fatal/Verbose("...")
	// Group 1 = level method, group 2 (optional) = first string-literal arg.
	reFSSerilogStatic = regexp.MustCompile(
		`\bLog\s*\.\s*(Information|Warning|Error|Debug|Fatal|Verbose)\s*\(\s*(?:@?"((?:[^"\\]|\\.)*)")?`,
	)

	// Serilog INSTANCE API: logger.Information/Warning/Error/Debug/Fatal/Verbose("...")
	// A bare `logger`/`_logger`/`log`/`_log` receiver (case-insensitive prefix) to
	// avoid matching arbitrary `.Error(` members. Group 1 = level, group 2 = msg.
	reFSSerilogInstance = regexp.MustCompile(
		`\b_?[Ll]og(?:ger)?\s*\.\s*(Information|Warning|Error|Debug|Fatal|Verbose)\s*\(\s*(?:@?"((?:[^"\\]|\\.)*)")?`,
	)

	// Microsoft.Extensions.Logging (MEL): logger.LogInformation/LogWarning/LogError/
	// LogDebug/LogTrace/LogCritical("..."). Group 1 = Log<Level>, group 2 = msg.
	reFSMEL = regexp.MustCompile(
		`\b_?[Ll]og(?:ger)?\s*\.\s*(Log(?:Information|Warning|Error|Debug|Trace|Critical))\s*\(\s*(?:@?\$?"((?:[^"\\]|\\.)*)")?`,
	)

	// F# console logging: printfn / eprintfn / printf / eprintf "...".
	// Group 1 = the print function, group 2 (optional) = the format/message string.
	reFSPrintfn = regexp.MustCompile(
		`\b(eprintfn|eprintf|printfn|printf)\b\s*(?:@?"((?:[^"\\]|\\.)*)")?`,
	)
)

// melLevel maps a MEL LogXxx method to its bare log level.
func melLevel(method string) string {
	return strings.ToLower(strings.TrimPrefix(method, "Log"))
}

// ---------------------------------------------------------------------------
// helpers (package-local; the custom/fsharp package has no shared helpers)
// ---------------------------------------------------------------------------

func lineOf(source string, offset int) int {
	return strings.Count(source[:offset], "\n") + 1
}

func makeLogEntity(name, filePath string, lineNum int) types.EntityRecord {
	ent := types.EntityRecord{
		Name:             name,
		Kind:             "SCOPE.Pattern",
		Subtype:          "log_extraction",
		SourceFile:       filePath,
		StartLine:        lineNum,
		EndLine:          lineNum,
		Language:         "fsharp",
		EnrichmentStatus: types.StatusPending,
		QualityScore:     1.0,
		Properties: map[string]string{
			"kind":    "SCOPE.Pattern",
			"subtype": "log_extraction",
		},
	}
	ent.ID = ent.ComputeID()
	return ent
}

func setProps(e *types.EntityRecord, kv ...string) {
	if len(kv)%2 != 0 {
		return
	}
	for i := 0; i < len(kv); i += 2 {
		e.Properties[kv[i]] = kv[i+1]
	}
}

// stampTemplate records the message template, or marks the call dynamic when no
// string literal was captured (non-literal / interpolated arg).
func stampTemplate(e *types.EntityRecord, template string, hadLiteral bool) {
	if hadLiteral {
		setProps(e, "message_template", template)
	} else {
		setProps(e, "dynamic", "true", "traced", "true")
	}
}

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *observabilityExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/fsharp")
	_, span := tracer.Start(ctx, "indexer.fsharp_observability_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "fsharp" {
		return nil, nil
	}

	src := string(file.Content)
	var entities []types.EntityRecord
	seen := make(map[string]bool)

	add := func(ent types.EntityRecord) {
		key := ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// --- MEL: logger.LogInformation/... -------------------------------------
	// Matched FIRST so MEL hits are claimed by the MEL pattern; the Serilog
	// instance pattern matches Information/Error/... (no Log prefix) and so does
	// not collide.
	melSpans := reFSMEL.FindAllStringSubmatchIndex(src, -1)
	for _, m := range melSpans {
		method := src[m[2]:m[3]]
		level := melLevel(method)
		line := lineOf(src, m[0])
		name := "log:mel:" + method + ":" + file.Path + ":" + itoa(line)
		ent := makeLogEntity(name, file.Path, line)
		setProps(&ent, "log_framework", "microsoft.extensions.logging",
			"pattern", "logger."+method, "log_level", level)
		hadLiteral := m[4] >= 0
		tmpl := ""
		if hadLiteral {
			tmpl = src[m[4]:m[5]]
		}
		stampTemplate(&ent, tmpl, hadLiteral)
		add(ent)
	}

	// --- Serilog static: Log.Information/... ---------------------------------
	staticClaimed := make(map[int]bool)
	for _, m := range reFSSerilogStatic.FindAllStringSubmatchIndex(src, -1) {
		staticClaimed[m[0]] = true
		level := strings.ToLower(src[m[2]:m[3]])
		line := lineOf(src, m[0])
		name := "log:serilog.static:" + src[m[2]:m[3]] + ":" + file.Path + ":" + itoa(line)
		ent := makeLogEntity(name, file.Path, line)
		setProps(&ent, "log_framework", "serilog",
			"pattern", "Log."+src[m[2]:m[3]], "log_level", level)
		hadLiteral := m[4] >= 0
		tmpl := ""
		if hadLiteral {
			tmpl = src[m[4]:m[5]]
		}
		stampTemplate(&ent, tmpl, hadLiteral)
		add(ent)
	}

	// --- Serilog instance: logger.Information/... ----------------------------
	// Skip any offset already claimed by a MEL match (defensive — patterns do
	// not overlap, but the receiver prefix is shared).
	melClaimed := make(map[int]bool, len(melSpans))
	for _, m := range melSpans {
		melClaimed[m[0]] = true
	}
	for _, m := range reFSSerilogInstance.FindAllStringSubmatchIndex(src, -1) {
		if melClaimed[m[0]] || staticClaimed[m[0]] {
			continue
		}
		method := src[m[2]:m[3]]
		level := strings.ToLower(method)
		line := lineOf(src, m[0])
		name := "log:serilog.instance:" + method + ":" + file.Path + ":" + itoa(line)
		ent := makeLogEntity(name, file.Path, line)
		setProps(&ent, "log_framework", "serilog",
			"pattern", "logger."+method, "log_level", level)
		hadLiteral := m[4] >= 0
		tmpl := ""
		if hadLiteral {
			tmpl = src[m[4]:m[5]]
		}
		stampTemplate(&ent, tmpl, hadLiteral)
		add(ent)
	}

	// --- printfn / eprintfn / printf / eprintf -------------------------------
	for _, m := range reFSPrintfn.FindAllStringSubmatchIndex(src, -1) {
		fn := src[m[2]:m[3]]
		level := "info"
		if strings.HasPrefix(fn, "e") {
			level = "error"
		}
		line := lineOf(src, m[0])
		name := "log:printf:" + fn + ":" + file.Path + ":" + itoa(line)
		ent := makeLogEntity(name, file.Path, line)
		setProps(&ent, "log_framework", "fsharp.printf",
			"pattern", fn, "log_level", level)
		hadLiteral := m[4] >= 0
		tmpl := ""
		if hadLiteral {
			tmpl = src[m[4]:m[5]]
		}
		stampTemplate(&ent, tmpl, hadLiteral)
		add(ent)
	}

	return entities, nil
}

// itoa avoids importing strconv for a single int→string conversion in name keys.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
