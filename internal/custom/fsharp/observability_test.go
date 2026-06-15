package fsharp_test

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
)

// logExtract runs the F# observability extractor and returns its records.
func logExtract(t *testing.T, path, lang, src string) []logRec {
	t.Helper()
	e, ok := extreg.Get("custom_fsharp_observability")
	if !ok {
		t.Fatal("custom_fsharp_observability not registered")
	}
	ents, err := e.Extract(context.Background(), fi(path, lang, src))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	out := make([]logRec, 0, len(ents))
	for _, en := range ents {
		out = append(out, logRec{
			kind:      en.Kind,
			subtype:   en.Subtype,
			framework: en.Properties["log_framework"],
			pattern:   en.Properties["pattern"],
			level:     en.Properties["log_level"],
			template:  en.Properties["message_template"],
			dynamic:   en.Properties["dynamic"],
		})
	}
	return out
}

type logRec struct {
	kind, subtype, framework, pattern, level, template, dynamic string
}

// findLog asserts at least one log_extraction record matches framework+level
// (and optionally template) and returns it.
func findLog(t *testing.T, recs []logRec, framework, level string) logRec {
	t.Helper()
	for _, r := range recs {
		if r.kind == "SCOPE.Pattern" && r.subtype == "log_extraction" &&
			r.framework == framework && r.level == level {
			return r
		}
	}
	t.Fatalf("no log_extraction record for framework=%q level=%q in %#v", framework, level, recs)
	return logRec{}
}

func TestFSharpObs_SerilogStatic(t *testing.T) {
	src := `module Svc
open Serilog

let run () =
    Log.Information("Processing order {OrderId}", orderId)
    Log.Warning("low disk")
    Log.Error(ex, "failed to load {Id}", id)
`
	recs := logExtract(t, "src/Svc.fs", "fsharp", src)
	r := findLog(t, recs, "serilog", "information")
	if r.pattern != "Log.Information" {
		t.Errorf("pattern = %q, want Log.Information", r.pattern)
	}
	if r.template != "Processing order {OrderId}" {
		t.Errorf("template = %q", r.template)
	}
	findLog(t, recs, "serilog", "warning")
	findLog(t, recs, "serilog", "error")
}

func TestFSharpObs_SerilogInstance(t *testing.T) {
	src := `module Svc
let handle (logger: ILogger) =
    logger.Information("started")
    logger.Error("boom {Code}", code)
`
	recs := logExtract(t, "src/Svc.fs", "fsharp", src)
	r := findLog(t, recs, "serilog", "information")
	if r.pattern != "logger.Information" {
		t.Errorf("pattern = %q, want logger.Information", r.pattern)
	}
	if r.template != "started" {
		t.Errorf("template = %q, want started", r.template)
	}
	findLog(t, recs, "serilog", "error")
}

func TestFSharpObs_MEL(t *testing.T) {
	src := `module Svc
let handle (logger: ILogger<Svc>) =
    logger.LogInformation("Processing {Id}", id)
    logger.LogWarning("careful")
    logger.LogError(ex, "boom")
    logger.LogDebug("dbg")
`
	recs := logExtract(t, "src/Svc.fs", "fsharp", src)
	info := findLog(t, recs, "microsoft.extensions.logging", "information")
	if info.pattern != "logger.LogInformation" {
		t.Errorf("pattern = %q", info.pattern)
	}
	if info.template != "Processing {Id}" {
		t.Errorf("template = %q", info.template)
	}
	findLog(t, recs, "microsoft.extensions.logging", "warning")
	findLog(t, recs, "microsoft.extensions.logging", "error")
	findLog(t, recs, "microsoft.extensions.logging", "debug")
	// MEL hits must NOT be double-counted as Serilog instance hits.
	for _, r := range recs {
		if r.framework == "serilog" {
			t.Errorf("MEL call leaked into serilog: %#v", r)
		}
	}
}

func TestFSharpObs_Printfn(t *testing.T) {
	src := `module Main
let main () =
    printfn "starting up on %d" port
    printf "tick"
    eprintfn "fatal: %s" msg
    eprintf "err"
`
	recs := logExtract(t, "src/Main.fs", "fsharp", src)
	out := findLog(t, recs, "fsharp.printf", "info")
	if out.template != "starting up on %d" {
		t.Errorf("template = %q", out.template)
	}
	err := findLog(t, recs, "fsharp.printf", "error")
	if err.template != "fatal: %s" {
		t.Errorf("template = %q", err.template)
	}
	// presence of both printf (info) and eprintf (error) patterns
	var info, e bool
	for _, r := range recs {
		if r.pattern == "printf" {
			info = true
		}
		if r.pattern == "eprintf" {
			e = true
		}
	}
	if !info || !e {
		t.Errorf("expected printf+eprintf patterns, got %#v", recs)
	}
}

func TestFSharpObs_DynamicTemplate(t *testing.T) {
	// Non-literal (interpolated/variable) message → dynamic=true, no fabricated template.
	src := `module Svc
let run (logger: ILogger) =
    Log.Information(message)
    logger.LogError($"id={id}")
`
	recs := logExtract(t, "src/Svc.fs", "fsharp", src)
	got := findLog(t, recs, "serilog", "information")
	if got.template != "" {
		t.Errorf("expected no template for non-literal arg, got %q", got.template)
	}
	if got.dynamic != "true" {
		t.Errorf("expected dynamic=true, got %q", got.dynamic)
	}
}

func TestFSharpObs_WrongLanguageNoop(t *testing.T) {
	src := `Log.Information("hi"); printfn "x"`
	recs := logExtract(t, "Svc.cs", "csharp", src)
	if len(recs) != 0 {
		t.Fatalf("expected no records for non-fsharp language, got %d", len(recs))
	}
}

func TestFSharpObs_NoLoggingNoop(t *testing.T) {
	src := `module Math
let add a b = a + b
`
	recs := logExtract(t, "src/Math.fs", "fsharp", src)
	if len(recs) != 0 {
		t.Fatalf("expected no log records, got %d: %#v", len(recs), recs)
	}
}
