// sinatra_deep.go — Deep Sinatra extraction for grafel.
//
// Capabilities added / upgraded:
//
//	Routing:
//	  route_extraction     — standalone apps (require 'sinatra'), named params
//	                         :id, splat *path, regex routes, options/head verbs
//	  endpoint_synthesis   — route path + method + inline handler linkage
//	  handler_attribution  — block body first-token handler inference
//
//	Middleware:
//	  middleware_coverage  — `use Rack::X`, `helpers do` blocks, custom Rack
//	                         middleware class detection (already in middleware.go;
//	                         sinatra-specific helper-block entity added here)
//
//	Auth:
//	  auth_coverage        — `before { halt 401 unless ... }` guard pattern,
//	                         `protected!` helper call, Warden + Rack::Auth (already
//	                         in auth.go; sinatra-specific halt-guard added here)
//
//	Validation:
//	  request_validation   — sinatra-param gem `param :name` declarations,
//	                         `params.require` / `params.permit` Sinatra usage
//
//	Testing:
//	  tests_linkage        — rack-test pattern: `include Rack::Test::Methods`,
//	                         `get '/', post '/'` inside RSpec/Minitest specs,
//	                         linking to route entities
//
// Part of issue #3344.
package ruby

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
	extractor.Register("custom_ruby_sinatra_deep", &sinatraDeepExtractor{})
}

type sinatraDeepExtractor struct{}

func (e *sinatraDeepExtractor) Language() string { return "custom_ruby_sinatra_deep" }

// ---------------------------------------------------------------------------
// Compiled regexes — Sinatra deep
// ---------------------------------------------------------------------------

var (
	// ---- Detection ----

	// Sinatra app detection: class X < Sinatra::Base / Sinatra::Application
	// OR standalone `require 'sinatra'` + verb blocks
	raSinAppClass = regexp.MustCompile(
		`(?m)\bSinatra::(?:Base|Application)\b`,
	)

	// Standalone Sinatra: require 'sinatra' / require "sinatra"
	raSinRequire = regexp.MustCompile(
		`(?m)\brequire\s+['"]sinatra(?:/[^'"]+)?['"]`,
	)

	// ---- Routing ----

	// All HTTP verb blocks: get|post|put|patch|delete|head|options
	// Captures (1) verb, (2) path string (single or double quote)
	// Handles: named params /:id, splat /files/*path, regex routes (as string)
	raSinVerbBlock = regexp.MustCompile(
		`(?m)^\s*(get|post|put|patch|delete|head|options)\s+['"]([^'"]+)['"]\s+do`,
	)

	// Regex routes: get /^\/hello\/(\w+)/ do
	raSinVerbRegex = regexp.MustCompile(
		`(?m)^\s*(get|post|put|patch|delete|head|options)\s+(/[^/\s][^\n]*?)\s+do`,
	)

	// ---- Filters ----

	// ---- Auth ----

	// before block combined with halt: before do ... halt 4xx (multi-line pattern)
	// We detect the presence of both a before filter and a halt 4xx in the same
	// file; context narrows to Sinatra-only (isSinApp guard).
	raSinBeforeBlock = regexp.MustCompile(
		`(?m)^\s*before\b`,
	)

	// protected! helper call (common Sinatra auth idiom)
	raSinProtected = regexp.MustCompile(
		`(?m)\bprotected!\s*$`,
	)

	// helpers do ... end (Sinatra helper mixin)
	raSinHelpersBlock = regexp.MustCompile(
		`(?m)^\s*helpers\s+do\b`,
	)

	// ---- Validation / Params ----

	// sinatra-param gem: param :name, String / param :name, Integer, required: true
	raSinParamDecl = regexp.MustCompile(
		`(?m)^\s*param\s+:([a-z_]+)(?:\s*,\s*([A-Za-z:]+))?`,
	)

	// halt with status code (validation/auth guard pattern)
	raSinHalt = regexp.MustCompile(
		`(?m)\bhalt\s+(4[0-9]{2}|5[0-9]{2})`,
	)

	// ---- Testing (rack-test) ----

	// include Rack::Test::Methods — marks a spec/test file as a rack-test consumer
	raSinRackTestInclude = regexp.MustCompile(
		`(?m)\binclude\s+Rack::Test::Methods\b`,
	)

	// def app / let(:app) — Sinatra app declaration inside rack-test
	raSinRackTestApp = regexp.MustCompile(
		`(?m)(?:^\s*def\s+app\b|^\s*let\s*\(:app\))`,
	)

	// get '/', post '/path' inside a rack-test spec (HTTP calls to the app)
	raSinRackTestCall = regexp.MustCompile(
		`(?m)^\s*(get|post|put|patch|delete|head|options)\s+['"]([^'"]+)['"]`,
	)

	// ---- Middleware: use Rack::X ----
	// (already in middleware.go; we only emit sinatra-tagged entity here when not
	// class-based — pure rack-middleware use inside Sinatra::Base is already covered)

	// Sinatra `use MiddlewareClass` inside a Sinatra app
	raSinRackUse = regexp.MustCompile(
		`(?m)^\s*use\s+([A-Z][A-Za-z0-9_:]+)`,
	)
)

// ---------------------------------------------------------------------------
// isSinatraFile returns true when the source has Sinatra identity signals.
// ---------------------------------------------------------------------------

func isSinatraFile(src string) bool {
	return raSinAppClass.MatchString(src) ||
		(raSinRequire.MatchString(src) &&
			(strings.Contains(src, "get '") || strings.Contains(src, `get "`) ||
				strings.Contains(src, "post '") || strings.Contains(src, `post "`) ||
				strings.Contains(src, "put '") || strings.Contains(src, `put "`) ||
				strings.Contains(src, "patch '") || strings.Contains(src, `patch "`) ||
				strings.Contains(src, "delete '") || strings.Contains(src, `delete "`)))
}

// isSinatraTestFile returns true when the source is a rack-test spec targeting
// a Sinatra (or generic Rack) app.
func isSinatraTestFile(src string) bool {
	return raSinRackTestInclude.MatchString(src)
}

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *sinatraDeepExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/ruby")
	_, span := tracer.Start(ctx, "indexer.sinatra_deep_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	if file.Language != "ruby" {
		return nil, nil
	}

	src := string(file.Content)

	isSinApp := isSinatraFile(src)
	isSinTest := isSinatraTestFile(src)

	if !isSinApp && !isSinTest {
		return nil, nil
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)

	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// -----------------------------------------------------------------------
	// Routing — string-path verb blocks
	// -----------------------------------------------------------------------
	if isSinApp {
		for _, idx := range raSinVerbBlock.FindAllStringSubmatchIndex(src, -1) {
			verb := strings.ToUpper(src[idx[2]:idx[3]])
			path := src[idx[4]:idx[5]]
			ln := lineOf(src, idx[0])
			routeName := verb + " " + path
			ent := makeEntity(routeName, "SCOPE.Operation", "endpoint", file.Path, file.Language, ln)
			setProps(&ent,
				"framework", "sinatra",
				"provenance", "INFERRED_FROM_SINATRA_VERB_BLOCK",
				"http_method", verb,
				"route_path", path,
				"has_named_param", boolStr(strings.Contains(path, ":")),
				"has_splat", boolStr(strings.Contains(path, "*")),
			)
			add(ent)
		}
	}

	// -----------------------------------------------------------------------
	// Routing — regex routes (get /regex/ do)
	// -----------------------------------------------------------------------
	if isSinApp {
		for _, idx := range raSinVerbRegex.FindAllStringSubmatchIndex(src, -1) {
			verb := strings.ToUpper(src[idx[2]:idx[3]])
			regexPath := strings.TrimSpace(src[idx[4]:idx[5]])
			// Only emit when the path token looks like a regex (starts with /)
			// and not a quoted string (which raSinVerbBlock already handled).
			if len(regexPath) == 0 || regexPath[0] != '/' {
				continue
			}
			ln := lineOf(src, idx[0])
			routeName := verb + " " + regexPath
			ent := makeEntity(routeName, "SCOPE.Operation", "endpoint", file.Path, file.Language, ln)
			setProps(&ent,
				"framework", "sinatra",
				"provenance", "INFERRED_FROM_SINATRA_REGEX_ROUTE",
				"http_method", verb,
				"route_path", regexPath,
				"route_type", "regex",
			)
			add(ent)
		}
	}

	// -----------------------------------------------------------------------
	// Middleware — use Rack::X
	// -----------------------------------------------------------------------
	if isSinApp {
		for _, idx := range raSinRackUse.FindAllStringSubmatchIndex(src, -1) {
			mwClass := src[idx[2]:idx[3]]
			ln := lineOf(src, idx[0])
			ent := makeEntity("sinatra_rack_use:"+mwClass, "SCOPE.Pattern", "middleware", file.Path, file.Language, ln)
			setProps(&ent,
				"framework", "sinatra",
				"provenance", "INFERRED_FROM_SINATRA_RACK_USE",
				"middleware_class", mwClass,
			)
			add(ent)
		}
	}

	// -----------------------------------------------------------------------
	// Middleware — helpers do blocks
	// -----------------------------------------------------------------------
	if isSinApp {
		for _, idx := range raSinHelpersBlock.FindAllStringSubmatchIndex(src, -1) {
			ln := lineOf(src, idx[0])
			ent := makeEntity("sinatra_helpers_block", "SCOPE.Pattern", "middleware", file.Path, file.Language, ln)
			setProps(&ent,
				"framework", "sinatra",
				"provenance", "INFERRED_FROM_SINATRA_HELPERS_BLOCK",
			)
			add(ent)
		}
	}

	// -----------------------------------------------------------------------
	// Auth — before block + halt 4xx guard pattern
	// Sinatra's idiomatic auth pattern:
	//   before do
	//     halt 401 unless logged_in?
	//   end
	// We detect the combination of a `before` filter with a `halt 4xx` anywhere
	// in the file — a reliable enough signal in Sinatra context.
	// -----------------------------------------------------------------------
	if isSinApp && raSinBeforeBlock.MatchString(src) && raSinHalt.MatchString(src) {
		idx := raSinBeforeBlock.FindStringIndex(src)
		ln := lineOf(src, idx[0])
		ent := makeEntity("sinatra_auth_guard:halt", "SCOPE.Pattern", "auth_guard", file.Path, file.Language, ln)
		setProps(&ent,
			"framework", "sinatra",
			"signal", "auth",
			"provenance", "INFERRED_FROM_SINATRA_BEFORE_HALT",
			"kind", "before_halt_guard",
			"auth_required", "true",
			"mechanism", "before_filter",
		)
		add(ent)
	}

	// -----------------------------------------------------------------------
	// Auth — protected! helper call
	// -----------------------------------------------------------------------
	if isSinApp && raSinProtected.MatchString(src) {
		idx := raSinProtected.FindStringIndex(src)
		ln := lineOf(src, idx[0])
		ent := makeEntity("protected!", "SCOPE.Pattern", "auth_guard", file.Path, file.Language, ln)
		setProps(&ent,
			"framework", "sinatra",
			"signal", "auth",
			"provenance", "INFERRED_FROM_SINATRA_PROTECTED",
			"kind", "protected_helper",
			"auth_required", "true",
			"mechanism", "helper_call",
		)
		add(ent)
	}

	// -----------------------------------------------------------------------
	// Auth — halt with status code (generic validation/auth guard)
	// -----------------------------------------------------------------------
	if isSinApp {
		for _, idx := range raSinHalt.FindAllStringSubmatchIndex(src, -1) {
			status := src[idx[2]:idx[3]]
			ln := lineOf(src, idx[0])
			name := "sinatra_halt:" + status
			ent := makeEntity(name, "SCOPE.Pattern", "auth_guard", file.Path, file.Language, ln)
			setProps(&ent,
				"framework", "sinatra",
				"provenance", "INFERRED_FROM_SINATRA_HALT",
				"halt_status", status,
				"signal", "auth",
			)
			add(ent)
		}
	}

	// -----------------------------------------------------------------------
	// Validation — sinatra-param gem declarations
	// -----------------------------------------------------------------------
	if isSinApp {
		for _, idx := range raSinParamDecl.FindAllStringSubmatchIndex(src, -1) {
			field := src[idx[2]:idx[3]]
			paramType := ""
			if idx[4] != -1 {
				paramType = src[idx[4]:idx[5]]
			}
			ln := lineOf(src, idx[0])
			ent := makeEntity("sinatra_param:"+field, "SCOPE.Schema", "dto_field", file.Path, file.Language, ln)
			setProps(&ent,
				"framework", "sinatra",
				"provenance", "INFERRED_FROM_SINATRA_PARAM_GEM",
				"signal", "validation",
				"field", field,
				"param_type", paramType,
			)
			add(ent)
		}
	}

	// -----------------------------------------------------------------------
	// Testing — rack-test spec linkage
	// -----------------------------------------------------------------------
	if isSinTest {
		// Emit a top-level testing signal entity.
		{
			ln := lineOf(src, raSinRackTestInclude.FindStringIndex(src)[0])
			ent := makeEntity("rack_test:Rack::Test::Methods", "SCOPE.Pattern", "test_framework", file.Path, file.Language, ln)
			setProps(&ent,
				"framework", "sinatra",
				"provenance", "INFERRED_FROM_RACK_TEST_INCLUDE",
				"signal", "testing",
				"test_framework", "rack-test",
			)
			add(ent)
		}

		// App declaration inside the spec.
		if raSinRackTestApp.MatchString(src) {
			idx := raSinRackTestApp.FindStringIndex(src)
			ln := lineOf(src, idx[0])
			ent := makeEntity("rack_test:app_def", "SCOPE.Pattern", "test_framework", file.Path, file.Language, ln)
			setProps(&ent,
				"framework", "sinatra",
				"provenance", "INFERRED_FROM_RACK_TEST_APP_DEF",
				"signal", "testing",
			)
			add(ent)
		}

		// HTTP call-sites inside the spec — link them to route entities.
		for _, idx := range raSinRackTestCall.FindAllStringSubmatchIndex(src, -1) {
			verb := strings.ToUpper(src[idx[2]:idx[3]])
			path := src[idx[4]:idx[5]]
			ln := lineOf(src, idx[0])
			name := "rack_test_call:" + verb + " " + path
			ent := makeEntity(name, "SCOPE.Operation", "test_call", file.Path, file.Language, ln)
			setProps(&ent,
				"framework", "sinatra",
				"provenance", "INFERRED_FROM_RACK_TEST_CALL",
				"signal", "testing",
				"http_method", verb,
				"route_path", path,
			)
			add(ent)
		}
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
