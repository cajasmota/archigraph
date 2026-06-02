package main

import (
	"fmt"
	"sort"
	"strings"
)

// Cross-linking cross-cutting "hub" records to the per-technology "spoke"
// records whose REAL extraction coverage lives elsewhere — often in a
// different category (#3873, generalizing #3853).
//
// The honesty problem this solves: a hub record like `db.mongodb`,
// `protocol.grpc`, `msg.broker.kafka` or an observability vendor renders
// as a thin page that looks almost empty, even though the SAME technology
// has rich code-level extraction spread across many separate per-language
// records (mongoose / mongoengine / the native drivers for MongoDB;
// tonic / grpc++ / grpc-dotnet / elixir-grpc for gRPC; rdkafka /
// librdkafka for Kafka; ...). The detail-page generator renders each
// record in ISOLATION, so the hub page never surfaces the real coverage
// and badly undersells it.
//
// We fix this in the GENERATOR (no extractor change): a curated set of
// hub→spoke groups associates each hub record with the id-substrings of
// the per-technology records that genuinely target the SAME tech/concept,
// constrained to a whitelist of spoke categories so an alias can never
// bleed into an unrelated record. The detail page grows a "Related
// extraction records" section (on the hub page, listing the spokes with a
// status digest) plus a back-link (on each spoke page, pointing at its
// hub). Rendering is deterministic and idempotent.
//
// HONESTY rules (apply to every group, not just databases):
//
//   - Only associate a spoke when it genuinely targets the SAME
//     technology/concept as the hub. General-purpose abstractions that
//     span many technologies (e.g. general SQL ORMs gorm/prisma/
//     sqlalchemy that target Postgres OR MySQL OR SQLite) are OMITTED —
//     linking them under one store would mis-attribute their coverage.
//   - Substrings match against the record ID's TERMINAL segment (the
//     library slug) and against the whole id; either hit associates the
//     record. They never match an arbitrary middle segment of the dotted
//     path.
//   - A spoke must live in one of the hub group's whitelisted spoke
//     categories. This is what keeps the grpc/graphql aliases (which
//     resolve into `http_framework`) from colliding with anything else.
//   - If a hub resolves to ZERO spokes, the section is omitted entirely
//     (no fabrication). protocol.soap / protocol.jsonrpc currently have
//     no per-language spoke records and therefore render no section.

// crossLinkGroup describes one hub→spoke association rule. Each hub record
// (identified by HubID) links to every spoke record whose terminal slug or
// id contains one of MatchSubstrings AND whose category is in SpokeCats.
type crossLinkGroup struct {
	HubID           string   // e.g. "protocol.grpc", "msg.broker.kafka", "db.mongodb"
	HubCategory     string   // category the hub record must belong to
	MatchSubstrings []string // id-slug substrings identifying the spokes
	SpokeCats       []string // whitelisted categories a spoke may live in
}

// crossLinkGroups is the curated set of hub→spoke rules across every
// cross-cutting hub category. Keep it HONEST: only add a substring when
// the spoke genuinely targets the hub's technology, and only widen
// SpokeCats when a real spoke lives there.
//
// databases: the original #3853 rules, now expressed as generic groups.
// General-purpose SQL ORMs (gorm/prisma/sqlalchemy/hibernate/ecto/diesel/
// sequelize/knex/dapper/doctrine/eloquent/slick) are deliberately absent —
// they target multiple SQL stores and naming one would mis-attribute.
//
// protocol: grpc/graphql resolve into per-language http_framework records
// (tonic, grpc++, grpc-dotnet, gqlgen, graphene, strawberry, ...) plus,
// for graphql, the msg.graphql-subscriptions transport record. soap and
// jsonrpc currently have NO per-language spoke records → no section.
//
// message_broker: kafka/rabbitmq broker hubs resolve to the per-language
// client records that live in the SAME message_broker category but are
// modelled as `lang.*.framework.*` rows (rdkafka, librdkafka, lapin).
//
// observability: vendor hubs resolve to the per-language/JVM facade
// records that export to them (Prometheus ← Micrometer / Dropwizard; the
// rest currently have no separate per-language spoke records → no
// section, honestly omitted rather than fabricated).
var crossLinkGroups = []crossLinkGroup{
	// ---- databases (preserved from #3853) -------------------------------
	{HubID: "db.mongodb", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"mongodb", "mongo", "mongoose", "mongoengine", "mongoid", "beanie", "spring-data-mongo", "mongocxx"}},
	{HubID: "db.neo4j", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"neo4j", "neomodel", "neogma", "spring-data-neo4j", "grafeo", "activegraph", "bolt-sips", "bolt_sips", "neo4rs"}},
	{HubID: "db.redis", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"redis", "ioredis", "spring-data-redis", "redix", "predis"}},
	{HubID: "db.cassandra", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"cassandra", "spring-data-cassandra", "scylla", "xandra"}},
	{HubID: "db.elasticsearch", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"elasticsearch", "elastic", "elastic4s"}},
	{HubID: "db.dynamodb", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"dynamodb", "scanamo"}},
	{HubID: "db.postgres", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"postgres", "postgrex", "psycopg", "asyncpg", "npgsql", "libpqxx", "pgx", "node-postgres"}},
	{HubID: "db.mysql", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"mysql", "mariadb", "myxql", "mysql2", "mysql-connector", "mysqlconnector", "pymysql", "mysqlclient", "go-sql-driver"}},
	{HubID: "db.sqlite", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"sqlite", "rusqlite", "better-sqlite", "ecto-sqlite", "ecto_sqlite3"}},
	{HubID: "db.clickhouse", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"clickhouse"}},
	{HubID: "db.snowflake", HubCategory: "databases", SpokeCats: []string{"orm"},
		MatchSubstrings: []string{"snowflake"}},

	// ---- protocol -------------------------------------------------------
	// gRPC: the per-language gRPC server/stub frameworks. Each names gRPC
	// explicitly (grpc++/grpc-net/elixir-grpc) or is a dedicated gRPC impl
	// (tonic = Rust gRPC; scalapb-grpc = Scala gRPC).
	{HubID: "protocol.grpc", HubCategory: "protocol", SpokeCats: []string{"http_framework", "orm"},
		MatchSubstrings: []string{"grpc", "tonic", "scalapb-grpc", "zio-grpc"}},
	// GraphQL: the per-language GraphQL frameworks/resolvers + the GraphQL
	// subscriptions transport record. Every entry is a dedicated GraphQL
	// library, so the slug match is unambiguous.
	{HubID: "protocol.graphql", HubCategory: "protocol", SpokeCats: []string{"http_framework", "message_broker"},
		MatchSubstrings: []string{"graphql", "gqlgen", "graphene", "strawberry", "absinthe", "hotchocolate", "type-graphql"}},
	// soap / jsonrpc: no per-language spoke records exist in the registry
	// today, so they are intentionally absent (the section is omitted).

	// ---- message_broker -------------------------------------------------
	// Kafka clients live in the same message_broker category but as
	// per-language `lang.*.framework.*` rows. rdkafka/librdkafka are
	// dedicated Kafka clients; kafka-streams is the Kafka streaming layer.
	{HubID: "msg.broker.kafka", HubCategory: "message_broker", SpokeCats: []string{"message_broker"},
		MatchSubstrings: []string{"rdkafka", "librdkafka", "kafka-streams"}},
	// RabbitMQ / AMQP: lapin is the dedicated Rust AMQP/RabbitMQ client.
	{HubID: "msg.broker.rabbitmq", HubCategory: "message_broker", SpokeCats: []string{"message_broker"},
		MatchSubstrings: []string{"lapin"}},

	// ---- observability --------------------------------------------------
	// Prometheus: Micrometer (JVM metrics facade) and Dropwizard Metrics
	// export to Prometheus; they are the per-runtime instrumentation that
	// feeds the Prometheus hub. Honest cross-link within observability.
	{HubID: "infra.observability.prometheus", HubCategory: "observability", SpokeCats: []string{"observability"},
		MatchSubstrings: []string{"micrometer", "dropwizard-metrics"}},
	// Other vendors (datadog/sentry/newrelic/otel) currently have no
	// separate per-language spoke records → no section (honestly omitted).
}

// crossLinkGroupByHubID indexes the curated groups by hub record ID for
// O(1) hub lookup. Built once at init.
var crossLinkGroupByHubID = func() map[string]crossLinkGroup {
	m := make(map[string]crossLinkGroup, len(crossLinkGroups))
	for _, g := range crossLinkGroups {
		m[g.HubID] = g
	}
	return m
}()

// datastoreAliases is retained as a derived view of the database groups so
// existing callers/tests that key off `db.<tech>` continue to work.
var datastoreAliases = func() map[string][]string {
	m := map[string][]string{}
	for _, g := range crossLinkGroups {
		if g.HubCategory == "databases" {
			m[g.HubID] = g.MatchSubstrings
		}
	}
	return m
}()

// relatedRecordDigest is one related spoke record rendered under the hub
// page's "Related extraction records" section, or the single hub record
// back-linked from a spoke page.
type relatedRecordDigest struct {
	ID       string // e.g. lang.jsts.orm.mongoose, lang.rust.framework.tonic
	Label    string // e.g. Mongoose, Tonic
	Language string // e.g. jsts, rust
	Kind     string // classifier from the id (driver/orm/odm/framework/broker), "" for hub
	Digest   string // compact status digest, e.g. "3 full, 2 partial"
}

// terminalSegment returns the last dotted segment of a record ID
// (the library slug), e.g. "mongoose" for "lang.jsts.orm.mongoose".
func terminalSegment(id string) string {
	if i := strings.LastIndex(id, "."); i >= 0 {
		return id[i+1:]
	}
	return id
}

// recordKind extracts the classifier from a record ID (the segment
// immediately before the terminal slug), e.g. "orm" for
// "lang.jsts.orm.mongoose" or "framework" for "lang.rust.framework.tonic".
// Returns "" when the id has no recognised classifier segment.
func recordKind(id string) string {
	parts := strings.Split(id, ".")
	if len(parts) < 2 {
		return ""
	}
	switch parts[len(parts)-2] {
	case "driver", "orm", "odm", "framework", "broker", "observability":
		return parts[len(parts)-2]
	}
	return ""
}

// matchesSlug reports whether recID's terminal slug (or whole id) contains
// any of the group's match substrings. Matching the terminal slug keeps an
// alias substring from bleeding across unrelated dotted segments.
func matchesSlug(subs []string, recID string) bool {
	seg := terminalSegment(recID)
	for _, s := range subs {
		if strings.Contains(seg, s) || strings.Contains(recID, s) {
			return true
		}
	}
	return false
}

// inCategorySet reports whether cat is one of the whitelisted categories.
func inCategorySet(cats []string, cat string) bool {
	for _, c := range cats {
		if c == cat {
			return true
		}
	}
	return false
}

// spokeMatches reports whether spoke record `rec` is a genuine spoke of
// the given hub group: its category is whitelisted and its slug matches.
// The hub record itself never matches (HubID guard).
func spokeMatches(g crossLinkGroup, rec Record) bool {
	if rec.ID == g.HubID {
		return false
	}
	if !inCategorySet(g.SpokeCats, rec.Category) {
		return false
	}
	return matchesSlug(g.MatchSubstrings, rec.ID)
}

// matchesDatastore is the database-only predicate retained for the
// collision-guard test. It reports whether a record (by ID) matches the
// db.<tech> hub's alias substrings, irrespective of category.
func matchesDatastore(db, recID string) bool {
	g, ok := crossLinkGroupByHubID[db]
	if !ok || g.HubCategory != "databases" {
		return false
	}
	return matchesSlug(g.MatchSubstrings, recID)
}

// statusDigest renders a compact per-status digest for one record, e.g.
// "3 full, 2 partial" or "5 partial" or "all not applicable". Counts are
// taken over every capability cell on the record (canonical + framework-
// specific). Statuses are listed in severity order (full, partial,
// missing, not_applicable); zero-count statuses are omitted. An empty
// record (no cells) renders "no cells".
func statusDigest(rec Record) string {
	caps := rec.AllCapabilitiesIncludingFrameworkSpecific()
	if len(caps) == 0 {
		return "no cells"
	}
	var full, partial, missing, na int
	for _, c := range caps {
		switch c.Status {
		case StatusFull:
			full++
		case StatusPartial:
			partial++
		case StatusMissing:
			missing++
		case StatusNotApplicable:
			na++
		}
	}
	parts := make([]string, 0, 4)
	if full > 0 {
		parts = append(parts, fmt.Sprintf("%d full", full))
	}
	if partial > 0 {
		parts = append(parts, fmt.Sprintf("%d partial", partial))
	}
	if missing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", missing))
	}
	if na > 0 {
		parts = append(parts, fmt.Sprintf("%d n/a", na))
	}
	if len(parts) == 0 {
		return "no cells"
	}
	return strings.Join(parts, ", ")
}

// relatedSpokeRecords returns the spoke records that target the same
// technology/concept as the given hub record, each with a compact status
// digest, sorted by (language, id) for deterministic output. Returns nil
// when the record is not a recognised hub or resolves to no spokes — the
// caller omits the section entirely in that case (never fabricates).
func relatedSpokeRecords(hub Record, all []Record) []relatedRecordDigest {
	g, ok := crossLinkGroupByHubID[hub.ID]
	if !ok || hub.Category != g.HubCategory {
		return nil
	}
	out := make([]relatedRecordDigest, 0, 16)
	for _, r := range all {
		if !spokeMatches(g, r) {
			continue
		}
		out = append(out, relatedRecordDigest{
			ID:       r.ID,
			Label:    r.Label,
			Language: r.Language,
			Kind:     recordKind(r.ID),
			Digest:   statusDigest(r),
		})
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Language != out[j].Language {
			return out[i].Language < out[j].Language
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// relatedDriverORMRecords is the database-specific alias retained for the
// existing #3853 tests. It delegates to the generic resolver but only for
// `databases`-category hubs.
func relatedDriverORMRecords(hub Record, all []Record) []relatedRecordDigest {
	if hub.Category != "databases" {
		return nil
	}
	return relatedSpokeRecords(hub, all)
}

// hubRecordFor returns the hub record that the given spoke record targets,
// or nil when there is none. A spoke matches a hub when the hub's group
// claims it (the same association used by relatedSpokeRecords, kept
// symmetric). When more than one hub could claim a spoke, the
// lexicographically-smallest hub ID wins for determinism (the live
// registry has no such collisions — guarded by test).
func hubRecordFor(spoke Record, all []Record) *relatedRecordDigest {
	// Index hub records present in `all` by ID for label/digest lookup.
	hubByID := map[string]Record{}
	for _, r := range all {
		if g, ok := crossLinkGroupByHubID[r.ID]; ok && r.Category == g.HubCategory {
			hubByID[r.ID] = r
		}
	}
	hubIDs := make([]string, 0, len(hubByID))
	for id := range hubByID {
		hubIDs = append(hubIDs, id)
	}
	sort.Strings(hubIDs)
	for _, id := range hubIDs {
		g := crossLinkGroupByHubID[id]
		if spokeMatches(g, spoke) {
			r := hubByID[id]
			return &relatedRecordDigest{
				ID:       r.ID,
				Label:    r.Label,
				Language: r.Language,
				Kind:     "",
				Digest:   statusDigest(r),
			}
		}
	}
	return nil
}

// infraRecordFor is the database-specific alias retained for the existing
// #3853 tests. It returns the `databases`-category hub a driver/ORM record
// targets, or nil. Delegates to hubRecordFor but restricts the result to
// database hubs and driver/ORM spokes.
func infraRecordFor(spoke Record, all []Record) *relatedRecordDigest {
	switch recordKind(spoke.ID) {
	case "driver", "orm", "odm":
	default:
		return nil
	}
	hub := hubRecordFor(spoke, all)
	if hub == nil {
		return nil
	}
	if g, ok := crossLinkGroupByHubID[hub.ID]; !ok || g.HubCategory != "databases" {
		return nil
	}
	return hub
}
