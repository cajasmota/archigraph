// rails_routes.go — deep Rails config/routes.rb DSL route extractor.
//
// This is the dedicated `route_extraction` extractor for Ruby on Rails. It
// parses config/routes.rb comprehensively and emits one SCOPE.Operation
// (subtype "endpoint") entity per *resolved* route, carrying:
//
//   - the fully-composed path  (namespace/scope/nested-resource prefixes
//     applied, e.g. /admin/photos/:photo_id/comments/:id),
//   - the HTTP method,
//   - the controller#action handler reference (e.g. "comments#show"), plus a
//     CALLS structural-ref edge pointing at the ActionController action method
//     in the conventionally-named controller file.
//
// It matches the depth bar set by the JS/TS backend synthesizers
// (internal/engine/http_endpoint_jsts_backend.go): full path composition,
// method, and handler attribution across nested/grouped routes.
//
// Supported routes.rb DSL:
//
//	resources :photos                    → 7 RESTful routes (index/create/new/
//	                                       show/edit/update[PUT+PATCH]/destroy)
//	resource :profile                    → 6 singular routes (no index, no :id)
//	  nested resources                   → /photos/:photo_id/comments…
//	namespace :admin do … end            → /admin prefix + Admin:: module
//	scope '/v1' do … end                 → path-only prefix
//	scope module: :internal do … end     → module-only prefix
//	scope path: '/v2', module: :v2       → both
//	member { get :preview }              → /photos/:id/preview
//	collection { get :search }           → /photos/search
//	only: […] / except: […]              → filter the 7 RESTful actions
//	root 'home#index'  /  root to: '…'   → GET /
//	get 'p', to: 'c#a'  (+post/put/patch/delete)
//	match 'p', to: 'c#a', via: [:get,:post]
//	mount Engine => '/path'              → engine mount point
//	concern :commentable do … end        → reusable route fragment
//	concerns: [:commentable]             → expands the concern inline
//
// Honest-partial remainder (backlogged): route `constraints` blocks beyond
// recording the prefix are not expanded; `direct`/`resolve` custom URL
// helpers are not modelled.
//
// Part of the Rails routing depth grind (route_extraction → full).
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
	extractor.Register("custom_ruby_rails_routes", &railsRoutesExtractor{})
}

type railsRoutesExtractor struct{}

func (e *railsRoutesExtractor) Language() string { return "custom_ruby_rails_routes" }

// ---------------------------------------------------------------------------
// Gate
// ---------------------------------------------------------------------------

// railsRoutesDrawGate matches the routes.rb signature. Files without it are
// skipped so this never fires on arbitrary Ruby (controllers, models, …).
var railsRoutesDrawGate = regexp.MustCompile(`(?m)\b(?:Rails\.application\.routes\.draw|Routes\.draw|routes\.draw)\b`)

// ---------------------------------------------------------------------------
// Line-level regexes
// ---------------------------------------------------------------------------

var (
	// `resources :photos` / `resources :photos, only: [:index]` (+ trailing `do`).
	rrResources = regexp.MustCompile(`^\s*resources\s+:(\w+)(.*)$`)
	// `resource :profile` (singular).
	rrResource = regexp.MustCompile(`^\s*resource\s+:(\w+)(.*)$`)
	// `namespace :admin do`.
	rrNamespace = regexp.MustCompile(`^\s*namespace\s+:?(\w+)\s+do\b`)
	// `scope '/v1' do`, `scope path: '/v1', module: 'x' do`, `scope module: :x do`.
	rrScope = regexp.MustCompile(`^\s*scope\b(.*)\bdo\b`)
	// `member do` / `collection do`.
	rrMemberCollection = regexp.MustCompile(`^\s*(member|collection)\s+do\b`)
	// `concern :commentable do`.
	rrConcern = regexp.MustCompile(`^\s*concern\s+:(\w+)\s+do\b`)
	// `constraints(...) do` / `constraints subdomain: 'api' do`.
	rrConstraints = regexp.MustCompile(`^\s*constraints\b.*\bdo\b`)
	// verb route: `get 'path', to: 'c#a'` | `get :preview` | `get 'p' => 'c#a'`.
	rrVerb = regexp.MustCompile(`^\s*(get|post|put|patch|delete|head|options)\b(.*)$`)
	// `match 'p', to: 'c#a', via: [:get, :post]`.
	rrMatch = regexp.MustCompile(`^\s*match\b(.*)$`)
	// `mount SomeEngine => '/blog'` / `mount SomeEngine, at: '/blog'`.
	rrMount = regexp.MustCompile(`^\s*mount\s+([\w:]+)(?:::Engine)?\b(.*)$`)
	// extract `to: 'c#a'` or `=> 'c#a'`.
	rrToRef = regexp.MustCompile(`(?:to:\s*|=>\s*)['"]([\w/]+#\w+)['"]`)
	// a bare positional 'controller#action' string (root 'home#index').
	rrBareRef = regexp.MustCompile(`['"]([\w/]+#\w+)['"]`)
	// extract a leading quoted path argument.
	rrFirstStr = regexp.MustCompile(`['"]([^'"]*)['"]`)
	// extract a leading symbol argument (`get :preview`).
	rrFirstSym = regexp.MustCompile(`^\s*:(\w+)`)
	// `only: [:index, :show]` / `only: :show`.
	rrOnly = regexp.MustCompile(`\bonly:\s*(?:\[([^\]]*)\]|:(\w+))`)
	// `except: [:destroy]`.
	rrExcept = regexp.MustCompile(`\bexcept:\s*(?:\[([^\]]*)\]|:(\w+))`)
	// `concerns: [:commentable]` / `concerns: :commentable`.
	rrConcerns = regexp.MustCompile(`\bconcerns:\s*(?:\[([^\]]*)\]|:(\w+))`)
	// `path: '/v1'`.
	rrPathOpt = regexp.MustCompile(`\bpath:\s*['"]([^'"]+)['"]`)
	// `module: 'x'` / `module: :x`.
	rrModuleOpt = regexp.MustCompile(`\bmodule:\s*['"]?:?(\w+)['"]?`)
	// `via: [:get, :post]` / `via: :all`.
	rrVia = regexp.MustCompile(`\bvia:\s*(?:\[([^\]]*)\]|:(\w+))`)
)

// railsRestfulActions are the 7 routes generated by `resources :name`.
var railsRestfulActions = []struct{ method, suffix, action string }{
	{"GET", "", "index"},
	{"POST", "", "create"},
	{"GET", "/new", "new"},
	{"GET", "/:id", "show"},
	{"GET", "/:id/edit", "edit"},
	{"PATCH", "/:id", "update"},
	{"PUT", "/:id", "update"},
	{"DELETE", "/:id", "destroy"},
}

// railsSingularActions are the 6 routes generated by singular `resource :name`
// (no index, no :id member segment — it identifies a single resource).
var railsSingularActions = []struct{ method, suffix, action string }{
	{"GET", "/new", "new"},
	{"POST", "", "create"},
	{"GET", "", "show"},
	{"GET", "/edit", "edit"},
	{"PATCH", "", "update"},
	{"PUT", "", "update"},
	{"DELETE", "", "destroy"},
}

// ---------------------------------------------------------------------------
// Parse state
// ---------------------------------------------------------------------------

// rrScopeState is the composed prefix context at the current nesting depth.
type rrScopeState struct {
	pathPrefix   string // composed URL prefix, e.g. "/admin/photos/:photo_id"
	modulePrefix string // composed controller module, e.g. "admin"
	// resource is the enclosing resource name (set inside a `resources` block),
	// used to attribute member/collection routes to the right controller.
	resource       string
	resourceMember bool // true → inside member block (route gets /:id segment)
}

// rrLine is one significant routes.rb line with its 1-based number.
type rrLine struct {
	text string
	num  int
}

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *railsRoutesExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/ruby")
	_, span := tracer.Start(ctx, "indexer.rails_routes_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "rails"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "ruby" {
		return nil, nil
	}
	src := string(file.Content)

	p := &railsRouteParser{
		filePath: file.Path,
		language: file.Language,
		rootDir:  deriveRailsRootDir(file.Path),
		concerns: map[string][]rrLine{},
		seen:     map[string]bool{},
	}
	p.lines = splitRRLines(src)
	// Gate on comment-stripped code: the routes.draw sentinel must appear as
	// actual code, not inside a comment.
	if !p.hasRoutesDrawSentinel() {
		return nil, nil
	}
	// First pass: collect concern definitions so `concerns:` can expand them.
	p.collectConcerns()
	// Second pass: walk the route tree.
	p.walk(0, len(p.lines), rrScopeState{})

	span.SetAttributes(attribute.Int("entity_count", len(p.out)))
	return p.out, nil
}

type railsRouteParser struct {
	filePath string
	language string
	rootDir  string
	lines    []rrLine
	concerns map[string][]rrLine
	out      []types.EntityRecord
	seen     map[string]bool
}

// splitRRLines tokenises source into significant lines (blank + comment-only
// lines dropped) with 1-based line numbers preserved.
func splitRRLines(src string) []rrLine {
	var out []rrLine
	num := 0
	for _, raw := range strings.Split(src, "\n") {
		num++
		t := strings.TrimSpace(raw)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		out = append(out, rrLine{text: stripRRComment(raw), num: num})
	}
	return out
}

// stripRRComment removes a trailing `# comment` from a line, honoring quoted
// strings so a '#' inside a path literal (rare) isn't mistaken for a comment.
func stripRRComment(line string) string {
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return strings.TrimRight(line[:i], " \t")
			}
		}
	}
	return line
}

// hasRoutesDrawSentinel reports whether any significant (comment-stripped) line
// contains the routes.draw block opener.
func (p *railsRouteParser) hasRoutesDrawSentinel() bool {
	for _, l := range p.lines {
		if railsRoutesDrawGate.MatchString(l.text) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Concern collection
// ---------------------------------------------------------------------------

func (p *railsRouteParser) collectConcerns() {
	i := 0
	for i < len(p.lines) {
		if m := rrConcern.FindStringSubmatch(p.lines[i].text); m != nil {
			name := m[1]
			end := p.matchEnd(i + 1)
			p.concerns[name] = append([]rrLine(nil), p.lines[i+1:end]...)
			i = end + 1
			continue
		}
		i++
	}
}

// ---------------------------------------------------------------------------
// Block walking
// ---------------------------------------------------------------------------

// matchEnd returns the index of the `end` line that closes the block whose
// body starts at index `start`, honoring nested do/end pairs. Returns
// len(lines) if unterminated.
func (p *railsRouteParser) matchEnd(start int) int {
	depth := 1
	for i := start; i < len(p.lines); i++ {
		t := strings.TrimSpace(p.lines[i].text)
		if t == "end" {
			depth--
			if depth == 0 {
				return i
			}
			continue
		}
		if rrOpensBlock(t) {
			depth++
		}
	}
	return len(p.lines)
}

// rrOpensBlock reports whether a trimmed line opens a do/end block.
func rrOpensBlock(t string) bool {
	if t == "do" || strings.HasSuffix(t, " do") {
		return true
	}
	// `... do |args|`
	if idx := strings.LastIndex(t, " do |"); idx >= 0 {
		return true
	}
	return false
}

// walk processes lines in [lo, hi) at the given scope state.
func (p *railsRouteParser) walk(lo, hi int, st rrScopeState) {
	i := lo
	for i < hi {
		line := p.lines[i]
		t := line.text

		// ---- namespace :admin do ----
		if m := rrNamespace.FindStringSubmatch(t); m != nil {
			name := m[1]
			end := p.matchEnd(i + 1)
			child := st
			child.pathPrefix = joinRRPath(st.pathPrefix, "/"+name)
			child.modulePrefix = joinRRModule(st.modulePrefix, name)
			child.resource = ""
			p.walk(i+1, min(end, hi), child)
			i = end + 1
			continue
		}

		// ---- scope … do ----
		if rrScope.MatchString(t) {
			opts := t
			end := p.matchEnd(i + 1)
			child := st
			if pm := rrPathOpt.FindStringSubmatch(opts); pm != nil {
				child.pathPrefix = joinRRPath(st.pathPrefix, ensureRRSlash(pm[1]))
			} else if sm := rrFirstStr.FindStringSubmatch(scopeArgHead(opts)); sm != nil {
				// bare positional path: `scope '/v1' do`
				child.pathPrefix = joinRRPath(st.pathPrefix, ensureRRSlash(sm[1]))
			}
			if mm := rrModuleOpt.FindStringSubmatch(opts); mm != nil {
				child.modulePrefix = joinRRModule(st.modulePrefix, mm[1])
			}
			child.resource = ""
			p.walk(i+1, min(end, hi), child)
			i = end + 1
			continue
		}

		// ---- constraints … do ---- (record-only: walk body, no path change)
		if rrConstraints.MatchString(t) {
			end := p.matchEnd(i + 1)
			p.walk(i+1, min(end, hi), st)
			i = end + 1
			continue
		}

		// ---- concern :name do ---- (already collected; skip its body)
		if rrConcern.MatchString(t) {
			end := p.matchEnd(i + 1)
			i = end + 1
			continue
		}

		// member/collection blocks are only valid inside a resource body and
		// are handled by walkResourceBody; a stray one at this level is skipped.
		if m := rrMemberCollection.FindStringSubmatch(t); m != nil {
			end := p.matchEnd(i + 1)
			i = end + 1
			continue
		}

		// ---- resources :photos [do … end] ----
		if m := rrResources.FindStringSubmatch(t); m != nil {
			name := m[1]
			opts := m[2]
			hasBlock := rrOpensBlock(strings.TrimSpace(t))
			base := joinRRPath(st.pathPrefix, "/"+name)
			actions := p.filterActions(railsRestfulActions, opts)
			for _, a := range actions {
				p.emitRoute(a.method, base+a.suffix, name, a.action, st.modulePrefix, line.num)
			}
			// inline concerns: on the resource declaration
			p.expandConcerns(opts, rrScopeState{
				pathPrefix:   base + "/:" + railsRouteSingularize(name) + "_id",
				modulePrefix: st.modulePrefix,
			}, line.num)
			if hasBlock {
				end := p.matchEnd(i + 1)
				child := st
				// Nested children compose under /photos/:photo_id.
				child.pathPrefix = base + "/:" + railsRouteSingularize(name) + "_id"
				child.resource = name
				child.resourceMember = false
				// member/collection inside resolve relative to /photos (not :photo_id).
				p.walkResourceBody(i+1, min(end, hi), child, base, name, st.modulePrefix)
				i = end + 1
				continue
			}
			i++
			continue
		}

		// ---- resource :profile (singular) [do … end] ----
		if m := rrResource.FindStringSubmatch(t); m != nil {
			name := m[1]
			opts := m[2]
			hasBlock := rrOpensBlock(strings.TrimSpace(t))
			base := joinRRPath(st.pathPrefix, "/"+name)
			for _, a := range p.filterSingular(railsSingularActions, opts) {
				p.emitRoute(a.method, base+a.suffix, name, a.action, st.modulePrefix, line.num)
			}
			if hasBlock {
				end := p.matchEnd(i + 1)
				child := st
				child.pathPrefix = base
				child.resource = name
				p.walkResourceBody(i+1, min(end, hi), child, base, name, st.modulePrefix)
				i = end + 1
				continue
			}
			i++
			continue
		}

		// ---- root 'home#index' / root to: 'home#index' ----
		if strings.HasPrefix(strings.TrimSpace(t), "root") {
			ref := extractRRRef(t)
			if ref == "" {
				if m := rrBareRef.FindStringSubmatch(t); m != nil {
					ref = m[1]
				}
			}
			if ref != "" {
				ctrl, action := splitRRRef(ref)
				p.emitRouteRef("GET", ensureRRSlash(st.pathPrefix), ctrl, action, st.modulePrefix, line.num)
			}
			i++
			continue
		}

		// ---- match 'p', to: 'c#a', via: [...] ----
		if rrMatch.MatchString(t) && strings.HasPrefix(strings.TrimSpace(t), "match") {
			p.emitMatch(t, st, line.num)
			i++
			continue
		}

		// ---- mount Engine => '/path' ----
		if m := rrMount.FindStringSubmatch(t); m != nil {
			engine := strings.TrimSuffix(m[1], "::Engine")
			mountPath := ""
			if sm := rrFirstStr.FindStringSubmatch(m[2]); sm != nil {
				mountPath = sm[1]
			}
			p.emitMount(engine, joinRRPath(st.pathPrefix, ensureRRSlash(mountPath)), line.num)
			i++
			continue
		}

		// ---- verb route: get 'p', to: 'c#a' ----
		if m := rrVerb.FindStringSubmatch(t); m != nil {
			method := strings.ToUpper(m[1])
			p.emitVerb(method, m[2], st, line.num)
			i++
			continue
		}

		i++
	}
}

// walkResourceBody walks the body of a `resources`/`resource` block. It handles
// member/collection blocks and inline `on:` routes (which resolve relative to
// the *collection* base, not the nested :id prefix), then delegates everything
// else (nested resources, plain verb routes) to a single recursive walk under
// the nested /:resource_id prefix.
func (p *railsRouteParser) walkResourceBody(lo, hi int, nested rrScopeState, collBase, resName, modulePrefix string) {
	i := lo
	for i < hi {
		line := p.lines[i]
		t := line.text

		// member/collection block → resolve relative to collBase.
		if m := rrMemberCollection.FindStringSubmatch(t); m != nil {
			end := p.matchEnd(i + 1)
			memberBase := collBase
			if m[1] == "member" {
				memberBase = collBase + "/:id"
			}
			p.emitMemberCollectionRoutes(i+1, min(end, hi), memberBase, resName, modulePrefix)
			i = end + 1
			continue
		}

		// inline `get :preview, on: :member` / `on: :collection` (no block).
		if mv := rrVerb.FindStringSubmatch(t); mv != nil && strings.Contains(t, "on:") {
			method := strings.ToUpper(mv[1])
			p.emitOnRoute(method, mv[2], collBase, resName, modulePrefix, line.num)
			i++
			continue
		}

		// A block opener (nested `resources … do`, `namespace`, etc.): hand the
		// whole block to walk() under the nested prefix and skip past it.
		if rrOpensBlock(strings.TrimSpace(t)) {
			end := p.matchEnd(i + 1)
			p.walk(i, min(end+1, hi), nested)
			i = end + 1
			continue
		}

		// Single non-block line (nested `resources :x` without block, a plain
		// verb route, etc.) under the nested prefix.
		p.walk(i, i+1, nested)
		i++
	}
}

// emitMemberCollectionRoutes emits routes inside a member/collection block:
// `get :preview` → <base>/preview attributed to <resource>#preview.
func (p *railsRouteParser) emitMemberCollectionRoutes(lo, hi int, base, resName, modulePrefix string) {
	for i := lo; i < hi; i++ {
		line := p.lines[i]
		t := line.text
		m := rrVerb.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		method := strings.ToUpper(m[1])
		action := rrActionName(m[2])
		if action == "" {
			continue
		}
		p.emitRoute(method, base+"/"+action, resName, action, modulePrefix, line.num)
	}
}

// emitOnRoute handles `get :preview, on: :member` / `on: :collection`.
func (p *railsRouteParser) emitOnRoute(method, args, collBase, resName, modulePrefix string, ln int) {
	action := rrActionName(args)
	if action == "" {
		return
	}
	base := collBase
	if strings.Contains(args, ":member") {
		base = collBase + "/:id"
	}
	p.emitRoute(method, base+"/"+action, resName, action, modulePrefix, ln)
}

// ---------------------------------------------------------------------------
// Verb / match / mount emitters
// ---------------------------------------------------------------------------

func (p *railsRouteParser) emitVerb(method, args string, st rrScopeState, ln int) {
	// Path: first quoted string, or leading symbol (`get :preview`).
	rawPath := ""
	if sm := rrFirstStr.FindStringSubmatch(args); sm != nil {
		rawPath = sm[1]
	} else if sm := rrFirstSym.FindStringSubmatch(args); sm != nil {
		rawPath = sm[1]
	}
	full := joinRRPath(st.pathPrefix, ensureRRSlash(rawPath))
	if ref := extractRRRef(args); ref != "" {
		ctrl, action := splitRRRef(ref)
		p.emitRouteRef(method, full, ctrl, action, st.modulePrefix, ln)
		return
	}
	// No explicit handler — attribute action by the path's last segment if it
	// is a bare symbol verb (`get :preview` inside a resource handled elsewhere).
	p.emitRouteRef(method, full, "", "", st.modulePrefix, ln)
}

func (p *railsRouteParser) emitMatch(t string, st rrScopeState, ln int) {
	args := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "match"))
	rawPath := ""
	if sm := rrFirstStr.FindStringSubmatch(args); sm != nil {
		rawPath = sm[1]
	}
	full := joinRRPath(st.pathPrefix, ensureRRSlash(rawPath))
	ref := extractRRRef(args)
	ctrl, action := splitRRRef(ref)
	methods := []string{"GET"}
	if vm := rrVia.FindStringSubmatch(args); vm != nil {
		methods = parseRRVia(vm)
	}
	for _, method := range methods {
		p.emitRouteRef(method, full, ctrl, action, st.modulePrefix, ln)
	}
}

func (p *railsRouteParser) emitMount(engine, path string, ln int) {
	name := "MOUNT " + path
	ent := makeEntity(name, "SCOPE.Operation", "endpoint", p.filePath, p.language, ln)
	setProps(&ent,
		"framework", "rails",
		"provenance", "INFERRED_FROM_RAILS_MOUNT",
		"route_path", path,
		"mounted_engine", engine,
	)
	p.add(ent)
}

// ---------------------------------------------------------------------------
// Core emit
// ---------------------------------------------------------------------------

// emitRoute emits a route attributed to <resource>#<action> (the resource
// controller convention).
func (p *railsRouteParser) emitRoute(method, path, resource, action, modulePrefix string, ln int) {
	ctrl := resource
	if modulePrefix != "" {
		ctrl = strings.ReplaceAll(modulePrefix, "::", "/") + "/" + resource
	}
	p.emitRouteRef(method, path, ctrl, action, "", ln)
}

// emitRouteRef emits a route with an explicit controller/action handler ref.
// ctrl may already include a module path (e.g. "admin/photos"); modulePrefix
// is applied when ctrl has none.
func (p *railsRouteParser) emitRouteRef(method, path, ctrl, action, modulePrefix string, ln int) {
	path = normalizeRRPath(path)
	handler := ""
	if ctrl != "" && action != "" {
		full := ctrl
		if modulePrefix != "" && !strings.Contains(ctrl, "/") {
			full = strings.ReplaceAll(modulePrefix, "::", "/") + "/" + ctrl
		}
		handler = full + "#" + action
	}
	name := method + " " + path
	ent := makeEntity(name, "SCOPE.Operation", "endpoint", p.filePath, p.language, ln)
	props := []string{
		"framework", "rails",
		"provenance", "INFERRED_FROM_RAILS_ROUTE",
		"http_method", method,
		"route_path", path,
	}
	if handler != "" {
		props = append(props, "handler", handler, "controller_action", handler)
		// CALLS structural-ref to the controller action method in its file.
		ctrlFile := railsControllerFile(handler, p.rootDir)
		ent.Relationships = append(ent.Relationships, types.RelationshipRecord{
			ToID: extractor.BuildOperationStructuralRef("ruby", ctrlFile, action),
			Kind: "CALLS",
		})
		props = append(props, "handler_file", ctrlFile)
	}
	setProps(&ent, props...)
	p.add(ent)
}

func (p *railsRouteParser) add(ent types.EntityRecord) {
	key := ent.Kind + ":" + ent.Name
	if p.seen[key] {
		return
	}
	p.seen[key] = true
	p.out = append(p.out, ent)
}

// ---------------------------------------------------------------------------
// only:/except:/concerns: filtering
// ---------------------------------------------------------------------------

type rrAction struct{ method, suffix, action string }

func (p *railsRouteParser) filterActions(in []struct{ method, suffix, action string }, opts string) []rrAction {
	allow := map[string]bool{}
	deny := map[string]bool{}
	if m := rrOnly.FindStringSubmatch(opts); m != nil {
		for _, a := range parseSymList(m[1], m[2]) {
			allow[a] = true
		}
	}
	if m := rrExcept.FindStringSubmatch(opts); m != nil {
		for _, a := range parseSymList(m[1], m[2]) {
			deny[a] = true
		}
	}
	var out []rrAction
	for _, a := range in {
		if len(allow) > 0 && !allow[a.action] {
			continue
		}
		if deny[a.action] {
			continue
		}
		out = append(out, rrAction{a.method, a.suffix, a.action})
	}
	return out
}

func (p *railsRouteParser) filterSingular(in []struct{ method, suffix, action string }, opts string) []rrAction {
	return p.filterActions(in, opts)
}

// expandConcerns inlines any `concerns: [:name]` referenced on a resource line.
func (p *railsRouteParser) expandConcerns(opts string, st rrScopeState, ln int) {
	m := rrConcerns.FindStringSubmatch(opts)
	if m == nil {
		return
	}
	for _, name := range parseSymList(m[1], m[2]) {
		body, ok := p.concerns[name]
		if !ok {
			continue
		}
		// Splice the concern body into a temporary parser run under st.
		sub := &railsRouteParser{
			filePath: p.filePath, language: p.language, rootDir: p.rootDir,
			lines: body, concerns: p.concerns, seen: p.seen,
		}
		sub.walk(0, len(body), st)
		p.out = append(p.out, sub.out...)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// scopeArgHead returns the portion of a `scope … do` line between `scope` and
// `do`, used to find a bare positional path string.
func scopeArgHead(t string) string {
	t = strings.TrimSpace(t)
	t = strings.TrimPrefix(t, "scope")
	if idx := strings.LastIndex(t, " do"); idx >= 0 {
		t = t[:idx]
	}
	return t
}

// extractRRRef pulls a `to: 'c#a'` or `=> 'c#a'` handler reference.
func extractRRRef(s string) string {
	if m := rrToRef.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// splitRRRef splits "controller#action" into ("controller", "action").
func splitRRRef(ref string) (ctrl, action string) {
	if ref == "" {
		return "", ""
	}
	if h := strings.IndexByte(ref, '#'); h > 0 && h < len(ref)-1 {
		return ref[:h], ref[h+1:]
	}
	return "", ""
}

// rrActionName returns the action symbol/string from a member/collection verb
// line's args (`:preview` or `'preview'`).
func rrActionName(args string) string {
	if sm := rrFirstSym.FindStringSubmatch(args); sm != nil {
		return sm[1]
	}
	if sm := rrFirstStr.FindStringSubmatch(args); sm != nil {
		seg := strings.Trim(sm[1], "/")
		if i := strings.LastIndexByte(seg, '/'); i >= 0 {
			seg = seg[i+1:]
		}
		return seg
	}
	return ""
}

// parseSymList parses an array body ("a, :b, c") or a single symbol capture.
func parseSymList(arr, single string) []string {
	if single != "" {
		return []string{single}
	}
	var out []string
	for _, p := range strings.Split(arr, ",") {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, ":")
		p = strings.Trim(p, "'\" ")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseRRVia maps a `via:` capture to upper-cased HTTP methods.
func parseRRVia(m []string) []string {
	syms := parseSymList(m[1], m[2])
	if len(syms) == 1 && syms[0] == "all" {
		return []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	}
	var out []string
	for _, s := range syms {
		out = append(out, strings.ToUpper(s))
	}
	if len(out) == 0 {
		return []string{"GET"}
	}
	return out
}

// joinRRPath composes two URL fragments avoiding double slashes.
func joinRRPath(a, b string) string {
	a = strings.TrimRight(a, "/")
	b = strings.TrimLeft(b, "/")
	if a == "" && b == "" {
		return "/"
	}
	if b == "" {
		return ensureRRSlash(a)
	}
	return ensureRRSlash(a + "/" + b)
}

// joinRRModule composes a controller module path (lower-cased, slash-joined).
func joinRRModule(a, b string) string {
	b = strings.ToLower(b)
	if a == "" {
		return b
	}
	return a + "::" + b
}

func ensureRRSlash(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		return "/" + p
	}
	return p
}

// normalizeRRPath collapses duplicate slashes and trims a trailing slash
// (except for root "/").
func normalizeRRPath(p string) string {
	if p == "" {
		return "/"
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	if p == "" {
		return "/"
	}
	return p
}

// railsRouteSingularize is a small, convention-driven singularizer sufficient for Rails
// nested-resource id segments (photos→photo, comments→comment, categories→
// category, addresses→address). Not a full inflector — covers the common
// English plural endings that appear in routes.rb.
func railsRouteSingularize(s string) string {
	switch {
	case strings.HasSuffix(s, "ies"):
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "sses"), strings.HasSuffix(s, "shes"),
		strings.HasSuffix(s, "ches"), strings.HasSuffix(s, "xes"):
		return s[:len(s)-2]
	case strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss"):
		return s[:len(s)-1]
	}
	return s
}

// railsControllerFile maps "admin/photos#index" → app/controllers/admin/
// photos_controller.rb under rootDir.
func railsControllerFile(handler, rootDir string) string {
	ref := handler
	if h := strings.IndexByte(ref, '#'); h >= 0 {
		ref = ref[:h]
	}
	rel := "app/controllers/" + ref + "_controller.rb"
	if rootDir != "" {
		rel = rootDir + "/" + rel
	}
	return rel
}

// deriveRailsRootDir returns the project root containing config/routes.rb.
func deriveRailsRootDir(routesPath string) string {
	norm := strings.ReplaceAll(routesPath, "\\", "/")
	if idx := strings.LastIndex(norm, "/config/"); idx >= 0 {
		return norm[:idx]
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
