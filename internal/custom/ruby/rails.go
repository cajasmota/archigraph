package ruby

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_ruby_rails", &railsExtractor{})
}

type railsExtractor struct{}

func (e *railsExtractor) Language() string { return "custom_ruby_rails" }

var (
	reRailsResources = regexp.MustCompile(
		`(?m)^\s*resources?\s+:([a-z_]+)`,
	)
	reRailsHTTPRoute = regexp.MustCompile(
		`(?m)^\s*(get|post|put|patch|delete|options|head)\s+['"]([^'"]+)['"]`,
	)
	reRailsNamespace = regexp.MustCompile(
		`(?m)^\s*namespace\s+:([a-z_]+)\s+do`,
	)
	reRailsConcern = regexp.MustCompile(
		`(?m)^\s*concern\s+:([a-z_]+)\s+do`,
	)
	reRailsCallback = regexp.MustCompile(
		`(?m)^\s*(before_save|after_save|before_create|after_create|before_update|after_update|before_destroy|after_destroy|after_commit|before_validation|after_validation)\s+:([a-z_]+)`,
	)
	reRailsAssociation = regexp.MustCompile(
		`(?m)^\s*(has_many|belongs_to|has_one|has_and_belongs_to_many)\s+:([a-z_]+)`,
	)
	reRailsBeforeAction = regexp.MustCompile(
		`(?m)^\s*(before_action|after_action|around_action)\s+:([a-z_]+[!?]?)`,
	)
	reRailsScope = regexp.MustCompile(
		`(?m)^\s*scope\s+:([a-z_]+)`,
	)
	reRailsActiveJobPerform = regexp.MustCompile(
		`(?m)^\s*def\s+(perform)\s*\(`,
	)
	// ActiveJob job class declaration: `class FooJob < ApplicationJob` or
	// `class FooJob < ActiveJob::Base`. ApplicationJob is the Rails-generated
	// base; ActiveJob::Base is the framework primitive.
	reRailsJobClass = regexp.MustCompile(
		`(?m)^\s*class\s+([A-Z][A-Za-z0-9_:]*)\s*<\s*(ApplicationJob|ActiveJob::Base)\b`,
	)
	// `queue_as :mailers` / `queue_as "mailers"` queue attribution inside a job.
	reRailsQueueAs = regexp.MustCompile(
		`(?m)^\s*queue_as\s+(?::([a-z_][a-z0-9_]*)|['"]([^'"]+)['"])`,
	)
	// ActiveJob producer dispatch: `FooJob.perform_later(args)` /
	// `FooJob.perform_now(args)` / `FooJob.set(...).perform_later`. The leading
	// receiver is a CONSTANT (job class), distinguishing it from Sidekiq's
	// `worker.perform_async` (lowercase receiver, handled by the sidekiq
	// extractor).
	reRailsJobDispatch = regexp.MustCompile(
		`(?m)\b([A-Z][A-Za-z0-9_:]*)(?:\.set\([^)]*\))?\.(perform_later|perform_now)\b`,
	)
	reRailsMailerMethod = regexp.MustCompile(
		`(?m)^\s*def\s+([a-z_]+)\s*(?:\()?`,
	)
	reRailsChannelStream = regexp.MustCompile(
		`(?m)\bstream_from\s+['"]([^'"]+)['"]`,
	)
)

// railsCRUDRoutes defines the 8 standard CRUD routes for resources.
var railsCRUDRoutes = []struct{ method, suffix string }{
	{"GET", ""},
	{"POST", ""},
	{"GET", "/new"},
	{"GET", "/:id"},
	{"GET", "/:id/edit"},
	{"PATCH", "/:id"},
	{"PUT", "/:id"},
	{"DELETE", "/:id"},
}

func (e *railsExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/ruby")
	_, span := tracer.Start(ctx, "indexer.rails_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "rails"),
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

	// 1. resources :name -> expand to 8 CRUD routes
	for _, m := range reRailsResources.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ln := lineOf(src, m[0])
		for _, cr := range railsCRUDRoutes {
			path := "/" + name + cr.suffix
			routeName := cr.method + " " + path
			ent := makeEntity(routeName, "SCOPE.Operation", "endpoint", file.Path, file.Language, ln)
			setProps(&ent, "framework", "rails", "provenance", "INFERRED_FROM_RAILS_RESOURCES",
				"http_method", cr.method, "route_path", path, "resource", name)
			add(ent)
		}
	}

	// 2. Explicit HTTP routes: get/post/put/patch/delete
	for _, m := range reRailsHTTPRoute.FindAllStringSubmatchIndex(src, -1) {
		method := strings.ToUpper(src[m[2]:m[3]])
		path := src[m[4]:m[5]]
		name := method + " " + path
		ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "rails", "provenance", "INFERRED_FROM_RAILS_ROUTE",
			"http_method", method, "route_path", path)
		add(ent)
	}

	// 3. namespace :name -> SCOPE.Component
	for _, m := range reRailsNamespace.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Component", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "rails", "provenance", "INFERRED_FROM_RAILS_NAMESPACE",
			"namespace", name)
		add(ent)
	}

	// 4. concern :name -> SCOPE.Pattern
	for _, m := range reRailsConcern.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "rails", "provenance", "INFERRED_FROM_RAILS_CONCERN",
			"concern_name", name)
		add(ent)
	}

	// 5. ActiveRecord callbacks -> SCOPE.Operation/function
	for _, m := range reRailsCallback.FindAllStringSubmatchIndex(src, -1) {
		cbType := src[m[2]:m[3]]
		cbName := src[m[4]:m[5]]
		name := fmt.Sprintf("%s:%s", cbType, cbName)
		ent := makeEntity(name, "SCOPE.Operation", "function", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "rails", "provenance", "INFERRED_FROM_RAILS_CALLBACK",
			"callback_type", cbType, "callback_method", cbName)
		add(ent)
	}

	// 6. ActiveRecord associations -> SCOPE.Component
	for _, m := range reRailsAssociation.FindAllStringSubmatchIndex(src, -1) {
		assocType := src[m[2]:m[3]]
		assocName := src[m[4]:m[5]]
		name := fmt.Sprintf("%s:%s", assocType, assocName)
		ent := makeEntity(name, "SCOPE.Component", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "rails", "provenance", "INFERRED_FROM_RAILS_ASSOCIATION",
			"association_type", assocType, "association_name", assocName)
		add(ent)
	}

	// 7. before_action/after_action filters -> SCOPE.Pattern
	// Each filter entity also carries a CALLS edge (structural-ref) pointing
	// at the filter method in the same controller file. The structural-ref
	// format matches what the tree-sitter Ruby extractor assigns to method
	// entities (scope:operation:method:ruby:<file>:<name>), so the resolver
	// can bind the edge within the same file without needing a cross-file
	// lookup. This closes the gap where filter chain relationships were
	// extracted but left as orphan SCOPE.Pattern entities with no outbound
	// edges.
	for _, m := range reRailsBeforeAction.FindAllStringSubmatchIndex(src, -1) {
		filterType := src[m[2]:m[3]]
		filterMethod := src[m[4]:m[5]]
		name := fmt.Sprintf("%s:%s", filterType, filterMethod)
		ent := makeEntity(name, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "rails", "provenance", "INFERRED_FROM_RAILS_FILTER",
			"filter_type", filterType, "filter_method", filterMethod)
		// Emit CALLS edge to the actual filter method via structural-ref so the
		// resolver can bind the pattern to the concrete SCOPE.Operation entity.
		ent.Relationships = append(ent.Relationships, types.RelationshipRecord{
			ToID: extractor.BuildOperationStructuralRef("ruby", file.Path, filterMethod),
			Kind: "CALLS",
		})
		add(ent)
	}

	// 8. Model scopes -> SCOPE.Operation/function
	for _, m := range reRailsScope.FindAllStringSubmatchIndex(src, -1) {
		name := "scope:" + src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Operation", "function", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "rails", "provenance", "INFERRED_FROM_RAILS_SCOPE",
			"scope_name", src[m[2]:m[3]])
		add(ent)
	}

	// 9a. ActiveJob job class declaration -> SCOPE.Service (the queue consumer).
	//     Capture an optional `queue_as` for queue attribution so the graph can
	//     answer "which jobs run on the :mailers queue?".
	jobClassMatches := reRailsJobClass.FindAllStringSubmatchIndex(src, -1)
	queueAs := ""
	if qm := reRailsQueueAs.FindStringSubmatch(src); qm != nil {
		if qm[1] != "" {
			queueAs = qm[1]
		} else {
			queueAs = qm[2]
		}
	}
	for _, m := range jobClassMatches {
		name := src[m[2]:m[3]]
		base := src[m[4]:m[5]]
		ent := makeEntity(name, "SCOPE.Service", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "activejob", "provenance", "INFERRED_FROM_ACTIVEJOB_CLASS",
			"job_base", base)
		if queueAs != "" {
			setProps(&ent, "queue", queueAs)
		}
		add(ent)
	}

	// 9b. ActiveJob perform -> SCOPE.Operation/function (the consumer handler).
	for _, m := range reRailsActiveJobPerform.FindAllStringSubmatchIndex(src, -1) {
		ent := makeEntity("perform", "SCOPE.Operation", "function", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "rails", "provenance", "INFERRED_FROM_RAILS_JOB_PERFORM")
		if queueAs != "" && len(jobClassMatches) > 0 {
			setProps(&ent, "queue", queueAs)
		}
		add(ent)
	}

	// 9c. ActiveJob producer dispatch -> SCOPE.Operation/function. A
	//     `FooJob.perform_later(args)` enqueue is the producer side that was
	//     previously entirely missing (only Sidekiq's lowercase-receiver
	//     `perform_async` was modelled). The constant receiver names the target
	//     job class so producer and consumer converge by name.
	for _, m := range reRailsJobDispatch.FindAllStringSubmatchIndex(src, -1) {
		jobClass := src[m[2]:m[3]]
		dispatchMethod := src[m[4]:m[5]]
		name := jobClass + "." + dispatchMethod
		ent := makeEntity(name, "SCOPE.Operation", "function", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "activejob", "provenance", "INFERRED_FROM_ACTIVEJOB_DISPATCH",
			"job_class", jobClass, "dispatch_method", dispatchMethod)
		add(ent)
	}

	// 10. stream_from -> SCOPE.Pattern
	for _, m := range reRailsChannelStream.FindAllStringSubmatchIndex(src, -1) {
		channel := src[m[2]:m[3]]
		name := "stream_from:" + channel
		ent := makeEntity(name, "SCOPE.Pattern", "", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "rails", "provenance", "INFERRED_FROM_RAILS_CHANNEL",
			"channel", channel)
		add(ent)
	}

	_ = reRailsMailerMethod // used implicitly via SCOPE.Operation patterns

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
